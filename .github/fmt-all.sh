#!/bin/sh
# Format the workflow YAML. See https://til.simonwillison.net/yaml/yamlfmt
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

if has_cmd yamlfmt; then
    :
else
    go install github.com/google/yamlfmt/cmd/yamlfmt@latest
fi

yamlfmt -conf .yamlfmt.yaml .
