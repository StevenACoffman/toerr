#!/bin/bash
set -uo pipefail

# Mutation-test the library packages under errors/ (usage: bin/mutate.sh [mutago flags]).
#
# mutago's built-in runner tests a mutant with the mutated package's own tests only.
# Here that undercounts: internal/trace is tested through the errors package, the
# properties in proptest/ check what no example does, and logctx's locking is checked
# only by the race detector. So each mutant runs every package's tests and the
# properties, with -race as CI runs them. A custom runner gets one worker per mutago
# process, so one process per package runs in parallel.
#
# mutago runs this script once per mutant, with the mutated file's path in
# MUTATE_ORIGINAL and the mutant's in MUTATE_CHANGED. go test's -overlay builds the
# mutant in place of the file, so the working tree is never edited. The exit code
# reports the result: 0 killed, 1 survived, 2 did not build.
root=$(cd "$(dirname "$0")/.." && pwd)

if [ -n "${MUTATE_ORIGINAL:-}" ]; then
    abs() { printf '%s/%s' "$(cd "$(dirname "$1")" && pwd)" "$(basename "$1")"; }
    tmp=$(mktemp -d)
    trap 'rm -rf "${tmp}"' EXIT
    printf '{"Replace":{"%s":"%s"}}' "$(abs "${MUTATE_ORIGINAL}")" "$(abs "${MUTATE_CHANGED}")" \
        >"${tmp}/overlay.json"
    run() {
        go -C "$1" test -race -count=1 -overlay="${tmp}/overlay.json" \
            -timeout "${MUTATE_TIMEOUT:-120}s" ./... >"${tmp}/out" 2>&1
    }
    if run "${root}" && run "${root}/proptest"; then
        exit 1
    fi
    if grep -qE '\[(build|setup) failed\]' "${tmp}/out"; then
        exit 2
    fi
    exit 0
fi

# Each package runs in a directory of its own, where mutago writes its reports; the
# agentic reports are merged into ./mutago-agentic.json afterwards.
runs=$(mktemp -d)
trap 'rm -rf "${runs}"' EXIT
pids=()
outs=()
while IFS= read -r dir; do
    out="${runs}/${#outs[@]}"
    mkdir -p "${out}"
    (cd "${out}" && mutago --exec "${root}/bin/mutate.sh" --exec-timeout=120 "$@" "${dir}" \
        >"${out}/log" 2>&1) &
    pids+=($!)
    outs+=("${out}")
    printf '%s\n' "${dir#"${root}"/}" >"${out}/name"
done < <(go -C "${root}" list -f '{{.Dir}}' ./errors/...)

status=0
for i in "${!pids[@]}"; do
    wait "${pids[${i}]}" || status=1
    echo "== $(cat "${outs[${i}]}/name")"
    cat "${outs[${i}]}/log"
done

reports=("${runs}"/*/mutago-agentic.json)
if [ -e "${reports[0]}" ]; then
    jq -s '{escaped_count: (map(.escaped_count) | add), mutants: (map(.mutants) | add)}' \
        "${reports[@]}" >mutago-agentic.json
    echo "wrote mutago-agentic.json"
fi
exit "${status}"
