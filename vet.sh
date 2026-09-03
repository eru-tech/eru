#!/usr/bin/env bash
# Runs go vet across every module in the workspace.
# Generated parser packages are excluded - they are marked "DO NOT EDIT" and
# regenerating them would discard any edit made to silence a warning.
set -uo pipefail

EXCLUDE='/eru-ql/ds/parser'
status=0

for mod in $(find . -name go.mod -not -path '*/node_modules/*' | sort); do
    dir=$(dirname "$mod")
    pkgs=$(cd "$dir" && go list ./... 2>/dev/null | grep -v "$EXCLUDE")
    [ -z "$pkgs" ] && continue
    if ! (cd "$dir" && go vet $pkgs 2>&1); then
        status=1
    fi
done

if [ $status -eq 0 ]; then
    echo "vet clean"
fi
exit $status
