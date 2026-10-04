#!/bin/sh
# Lint the workflow YAML and the workflows themselves.
set -eu
cd "$(dirname "$0")"

# has_cmd NAME — true if NAME is an executable file on $PATH.
# Ignores shell functions, aliases, and builtins of the same name.
has_cmd() {
    if [ -n "${ZSH_VERSION:-}" ]; then
        builtin whence -p -- "$1" >/dev/null 2>&1
    elif [ -n "${BASH_VERSION:-}" ]; then
        builtin type -P -- "$1" >/dev/null 2>&1
    else
        command -v -- "$1" >/dev/null 2>&1
    fi
}

GOBIN="${GOBIN:-$HOME/go/bin}"
export GOBIN
PATH="$GOBIN:$PATH"

if has_cmd yamllint; then :; else go install github.com/wasilibs/go-yamllint/cmd/yamllint@latest; fi
if has_cmd actionlint; then :; else go install github.com/rhysd/actionlint/cmd/actionlint@latest; fi
if has_cmd shellcheck; then :; else go install github.com/wasilibs/go-shellcheck/cmd/shellcheck@latest; fi

# The Wasm builds of these tools can only see files beneath the current directory, so
# ../yaml/my.yaml or /separate/root/my.yaml will not be found.
yamllint -c .yamllint.yaml .

# actionlint looks for .github/workflows from the repository root.
cd ..
SHELLCHECK_OPTS='-e SC2086 -e SC2129' actionlint -shellcheck="$(command -v shellcheck)"
