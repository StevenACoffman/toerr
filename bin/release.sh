#!/bin/bash
set -euo pipefail

# Tag and publish a new release from main.
#
# Every Go module in the repository is released at the same version, from the same
# commit: the root module gets the tag vX.Y.Z, and each module under errors/ (the
# adapters that are modules of their own, such as errors/errgrpc) gets
# errors/<name>/vX.Y.Z, the tag the Go toolchain resolves for that module path.
# Modules are found by their go.mod, so a new one is released without editing this
# script. proptest/ is a module too, but it is test-only and never released.
#
# Aborts unless ALL of these hold:
#   - exactly one version argument, a canonical vMAJOR.MINOR.PATCH[-PRERELEASE]
#   - the current branch is main
#   - the working tree is clean (no staged, unstaged, or untracked changes)
#   - none of the tags already exists
#   - each module under errors/ declares the module path its directory implies,
#     since that is the path its errors/<name>/vX.Y.Z tag resolves for
#   - each module under errors/ requires toerr at this same version, so what
#     consumers resolve is what was built and tested through the replace
#   - each module under errors/ has an SBOM entry and a `go get` line in the
#     release notes, both in .goreleaser.yaml
#   - local main is identical to origin/main (nothing unpushed, not behind,
#     not diverged)
#
# After tagging, it commits and pushes to main a change that makes each module under
# errors/ require the next toerr version: the next minor, or a pre-release's own
# final version. Main then builds them against the working tree as that release's
# code, and the next release's version check passes without an edit.
ORG="StevenACoffman"
REPO="toerr"
ROOT_MODULE="github.com/${ORG}/${REPO}"
if [ $# -ne 1 ]; then
    echo "usage: ./bin/release.sh vX.Y.Z" >&2
    exit 1
fi
VERSION=$1

# Go reads only canonical semantic versions as module versions: no leading zeros.
if [[ ! ${VERSION} =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$ ]]; then
    echo "version must be vMAJOR.MINOR.PATCH[-PRERELEASE], no leading zeros (got: ${VERSION})" >&2
    exit 1
fi

# Must be on main.
branch=$(git branch --show-current)
if [ "${branch}" != "main" ]; then
    echo "must be on main to release (on: ${branch:-detached HEAD})" >&2
    exit 1
fi

# Working tree must be clean: modifications, staged changes, or untracked files
# all count as dirty.
if [ -n "$(git status --porcelain)" ]; then
    echo "working tree is dirty, aborting" >&2
    exit 1
fi

# The modules released beside the root one, by directory (errors/errgrpc, ...).
module_dirs=()
while IFS= read -r gomod; do
    module_dirs+=("$(dirname "${gomod}")")
done < <(git ls-files -- ':(glob)errors/**/go.mod')

# The tags to create: the root module's first, then one per module directory.
tags=("${VERSION}")
for dir in ${module_dirs[@]+"${module_dirs[@]}"}; do
    tags+=("${dir}/${VERSION}")
done

# Refuse to overwrite an existing tag.
for tag in "${tags[@]}"; do
    if git rev-parse -q --verify "refs/tags/${tag}" >/dev/null; then
        echo "tag ${tag} already exists, aborting" >&2
        exit 1
    fi
done

for dir in ${module_dirs[@]+"${module_dirs[@]}"}; do
    # The Go toolchain finds errors/<name>/vX.Y.Z for the module path
    # <root>/errors/<name>; a go.mod that declares any other path is never found.
    declared=$(go -C "${dir}" list -m)
    if [ "${declared}" != "${ROOT_MODULE}/${dir}" ]; then
        echo "${dir}/go.mod declares ${declared}, not ${ROOT_MODULE}/${dir}" >&2
        exit 1
    fi
    # A module builds against the working tree through a replace, which consumers
    # ignore: they get the toerr version the module requires. Requiring the version
    # being released keeps the two the same.
    required=$(go -C "${dir}" list -m -f '{{.Version}}' "${ROOT_MODULE}")
    if [ "${required}" != "${VERSION}" ]; then
        echo "${dir}/go.mod requires ${ROOT_MODULE} ${required}, not ${VERSION};" >&2
        echo "  run: go -C ${dir} mod edit -require=${ROOT_MODULE}@${VERSION}" >&2
        exit 1
    fi
    # The release publishes one SBOM per module and tells consumers how to get
    # each one; without both, this module would be tagged but missing from it.
    if ! grep -qF -- "dir:../${dir}\"" .goreleaser.yaml; then
        echo "${dir} has no SBOM entry in .goreleaser.yaml (scanning dir:../${dir})" >&2
        exit 1
    fi
    if ! grep -qF -- "/${dir}@{{ .Tag }}" .goreleaser.yaml; then
        echo "${dir} has no go get line in the .goreleaser.yaml release footer" >&2
        exit 1
    fi
done

# Local main must be exactly in sync with the remote.
git fetch --quiet origin main
local_sha=$(git rev-parse HEAD)
remote_sha=$(git rev-parse origin/main)
base_sha=$(git merge-base HEAD origin/main)
if [ "${local_sha}" != "${remote_sha}" ]; then
    if [ "${local_sha}" = "${base_sha}" ]; then
        echo "local main is behind origin/main; pull first" >&2
    elif [ "${remote_sha}" = "${base_sha}" ]; then
        echo "local main has unpushed commits; push first" >&2
    else
        echo "local main and origin/main have diverged; reconcile first" >&2
    fi
    exit 1
fi

# All preconditions met: tag the current commit. If any tag fails, delete the ones
# already made, so a retry starts from nothing.
created=()
cleanup() {
    for tag in ${created[@]+"${created[@]}"}; do
        git tag -d "${tag}" >/dev/null
    done
}
trap cleanup ERR
for tag in "${tags[@]}"; do
    git tag -s "${tag}" -m "${tag}"
    created+=("${tag}")
done

# Push only the tags, all or none, so the proxy never sees a module tag without
# the root tag it requires.
git push --atomic origin "${tags[@]}"
trap - ERR

echo "Tagged ${tags[*]}. Now write release notes:"
echo "  https://github.com/${ORG}/${REPO}/releases/tag/${VERSION}"

# Prime the Go module proxy so `go get` sees each new version promptly. The root
# module goes first, since every other module requires it. The proxy escapes each
# upper-case letter in a module path as "!" and its lower case.
for tag in "${tags[@]}"; do
    path="${ROOT_MODULE}"
    if [ "${tag}" != "${VERSION}" ]; then
        path="${ROOT_MODULE}/${tag%/"${VERSION}"}"
    fi
    escaped=$(printf '%s' "${path}" | perl -pe 's/([A-Z])/!\l$1/g')
    curl -sS "https://proxy.golang.org/${escaped}/@v/${VERSION}.info"
    echo
done

# Point each module at the next release, so main builds it against the working
# tree as that release's code. The tags above keep the released requirement.
# A pre-release points at its own final version; a release at the next minor.
if [[ ${VERSION} =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)(-.*)?$ ]]; then
    if [ -n "${BASH_REMATCH[4]}" ]; then
        next="v${BASH_REMATCH[1]}.${BASH_REMATCH[2]}.${BASH_REMATCH[3]}"
    else
        next="v${BASH_REMATCH[1]}.$((BASH_REMATCH[2] + 1)).0"
    fi
    gomods=()
    for dir in ${module_dirs[@]+"${module_dirs[@]}"}; do
        gomods+=("${dir}/go.mod")
    done
    if [ ${#gomods[@]} -gt 0 ]; then
        # The tags are public by now, so a rerun stops at "tag already exists":
        # say how to finish by hand.
        trap 'echo "released ${VERSION}, but requiring ${next} on main failed; finish with:" >&2
              echo "  git checkout -- ${gomods[*]}; then for each module:" >&2
              echo "  go -C <dir> mod edit -require=${ROOT_MODULE}@${next}" >&2
              echo "  and commit, then: git pull --rebase && git push origin main" >&2' ERR
        for dir in "${module_dirs[@]}"; do
            go -C "${dir}" mod edit -require="${ROOT_MODULE}@${next}"
        done
        git commit -S -m "errgrpc, errpgx: require toerr ${next} for the next release" -- "${gomods[@]}"
        trap 'echo "released ${VERSION} and committed requiring ${next}, but the push failed;" >&2
              echo "  finish with: git pull --rebase && git push origin main" >&2' ERR
        git push origin main
        trap - ERR
    fi
fi
