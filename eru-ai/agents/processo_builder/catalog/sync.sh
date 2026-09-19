#!/usr/bin/env bash
#
# Pulls the processo entity/field catalog into eru-ai.
#
# The catalog is generated from the processo app's own sources - the datatype
# list the field editor offers and the payload its save builder emits - so this
# is the one command that keeps the processo_builder agent's idea of the data
# model in step with the app. Run it after any change to field-editor or
# data-model.
#
#   ./sync.sh                                # uses PROCESSO_PATH or the default checkout
#   ./sync.sh /path/to/processo              # explicit checkout
#   ./sync.sh --check                        # exit 1 if the committed catalog is stale
#
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TARGET="$HERE/processo_catalog.json"

CHECK=0
PROCESSO=""
for arg in "$@"; do
  case "$arg" in
    --check) CHECK=1 ;;
    -*) echo "unknown flag: $arg" >&2; exit 2 ;;
    *) PROCESSO="$arg" ;;
  esac
done

if [[ -z "$PROCESSO" ]]; then
  PROCESSO="${PROCESSO_PATH:-$HERE/../../../../../../angularworkspace/processo}"
fi

if [[ ! -d "$PROCESSO" ]]; then
  cat >&2 <<EOF
processo checkout not found: $PROCESSO

Point at it explicitly:
  PROCESSO_PATH=/path/to/processo $0
  $0 /path/to/processo
EOF
  exit 2
fi

EXPORTER="$PROCESSO/scripts/export-processo-catalog.mjs"
if [[ ! -f "$EXPORTER" ]]; then
  echo "exporter missing: $EXPORTER (update processo first)" >&2
  exit 2
fi

TMP="$(mktemp -t processo-catalog-XXXXXX.json)"
trap 'rm -f "$TMP"' EXIT

echo "generating catalog from $PROCESSO"
(cd "$PROCESSO" && node scripts/export-processo-catalog.mjs --out "$TMP" --quiet)

if [[ $CHECK -eq 1 ]]; then
  if cmp -s "$TMP" "$TARGET"; then
    echo "catalog is in sync"
    exit 0
  fi
  echo "catalog is STALE - run $0 and commit the result" >&2
  diff <(python3 -c 'import json,sys;print(json.dumps(json.load(open(sys.argv[1])),indent=1,sort_keys=True))' "$TARGET") \
       <(python3 -c 'import json,sys;print(json.dumps(json.load(open(sys.argv[1])),indent=1,sort_keys=True))' "$TMP") \
       | head -60 >&2 || true
  exit 1
fi

OLD_FP=""
if [[ -f "$TARGET" ]]; then
  OLD_FP="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["source"]["fingerprint"])' "$TARGET" 2>/dev/null || true)"
fi
cp "$TMP" "$TARGET"
NEW_FP="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["source"]["fingerprint"])' "$TARGET")"

if [[ "$OLD_FP" == "$NEW_FP" ]]; then
  echo "catalog unchanged ($NEW_FP)"
else
  echo "catalog updated: ${OLD_FP:-none} -> $NEW_FP"
fi

echo "verifying against the agent"
(cd "$HERE/../../.." && go test ./agents/processo_builder/... )
