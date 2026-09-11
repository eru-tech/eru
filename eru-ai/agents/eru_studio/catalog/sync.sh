#!/usr/bin/env bash
#
# Pulls the eru-studio component catalog into eru-ai.
#
# The catalog is generated from the Angular library sources, so this is the one
# command that keeps the Eru Studio agent's idea of the component library in
# step with the library itself. Run it after any component change in eru-studio.
#
#   ./sync.sh                                   # uses ERU_STUDIO_PATH or the default checkout
#   ./sync.sh /path/to/eru-studio               # explicit checkout
#   ./sync.sh --check                           # exit 1 if the committed catalog is stale
#
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TARGET="$HERE/component_catalog.json"

CHECK=0
STUDIO=""
for arg in "$@"; do
  case "$arg" in
    --check) CHECK=1 ;;
    -*) echo "unknown flag: $arg" >&2; exit 2 ;;
    *) STUDIO="$arg" ;;
  esac
done

if [[ -z "$STUDIO" ]]; then
  STUDIO="${ERU_STUDIO_PATH:-$HERE/../../../../../../angularworkspace/eru-studio}"
fi

if [[ ! -d "$STUDIO" ]]; then
  cat >&2 <<EOF
eru-studio checkout not found: $STUDIO

Point at it explicitly:
  ERU_STUDIO_PATH=/path/to/eru-studio $0
  $0 /path/to/eru-studio
EOF
  exit 2
fi

EXPORTER="$STUDIO/scripts/export-agent-catalog.mjs"
if [[ ! -f "$EXPORTER" ]]; then
  echo "exporter missing: $EXPORTER (update eru-studio first)" >&2
  exit 2
fi

TMP="$(mktemp -t eru-studio-catalog-XXXXXX.json)"
trap 'rm -f "$TMP"' EXIT

echo "generating catalog from $STUDIO"
(cd "$STUDIO" && node scripts/export-agent-catalog.mjs --out "$TMP" --quiet)

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
(cd "$HERE/../../.." && go test ./agents/eru_studio/... )
