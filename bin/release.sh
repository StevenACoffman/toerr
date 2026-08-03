#!/bin/bash
set -euo pipefail

# Tag and publish a new release from main.
#
# Aborts unless ALL of these hold:
#   - exactly one version argument, starting with "v"
#   - the current branch is main
#   - the working tree is clean (no staged, unstaged, or untracked changes)
#   - local main is identical to origin/main (nothing unpushed, not behind,
#     not diverged)
#   - the tag does not already exist
ORG="StevenACoffman"
REPO="toerr"
if [ $# -ne 1 ]; then
    echo "usage: ./bin/release.sh vX.Y.Z" >&2
    exit 1
fi
VERSION=$1

if [[ ${VERSION} != v* ]]; then
    echo "version must start with v (got: ${VERSION})" >&2
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

# Refuse to overwrite an existing tag.
if git rev-parse -q --verify "refs/tags/${VERSION}" >/dev/null; then
    echo "tag ${VERSION} already exists, aborting" >&2
    exit 1
fi

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

# All preconditions met: tag the current commit and push only the tag.
git tag -s "${VERSION}" -m "${VERSION}"
git push origin "${VERSION}"

echo "Tagged ${VERSION}. Now write release notes:"
echo "  https://github.com/${ORG}/${REPO}/releases/tag/${VERSION}"

# Prime the Go module proxy so `go get` sees the new version promptly.
curl -sS "https://proxy.golang.org/github.com/!steven!a!coffman/${REPO}/@v/${VERSION}.info"
echo
