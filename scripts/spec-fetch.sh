#!/usr/bin/env bash
# spec-fetch.sh — download the five OpenAPI documents the CLI is generated from.
#
# These are the enumeration source (cliwright GOAL.md §0 Step 1b): the command surface, the
# embedded operation catalog and the coverage number in api-manifest.json are all derived
# from them rather than from recall. They total ~4MB and change on Atlassian's schedule, so
# they are fetched on demand instead of checked in; the generated catalog is what ships.
#
# Usage: ./scripts/spec-fetch.sh [dest-dir]
set -euo pipefail
DEST="${1:-specs}"
mkdir -p "$DEST"

# file<TAB>url
SPECS=$(cat <<'EOF'
jira-platform.json	https://developer.atlassian.com/cloud/jira/platform/swagger-v3.v3.json
jira-software.json	https://developer.atlassian.com/cloud/jira/software/swagger.v3.json
jira-servicedesk.json	https://developer.atlassian.com/cloud/jira/service-desk/swagger.v3.json
confluence-v2.json	https://developer.atlassian.com/cloud/confluence/openapi-v2.v3.json
confluence-v1.json	https://developer.atlassian.com/cloud/confluence/swagger.v3.json
EOF
)

fail=0
while IFS=$'\t' read -r file url; do
  [ -n "$file" ] || continue
  printf '  ↓ %-24s ' "$file"
  if curl -fsSL --retry 3 --max-time 180 -o "$DEST/$file.tmp" "$url"; then
    # A truncated or HTML error page must not silently become a "spec" — the generator would
    # then report a smaller api_method_total and the completeness gate would pass on a lie.
    if jq -e '.paths | length > 0' "$DEST/$file.tmp" >/dev/null 2>&1; then
      mv "$DEST/$file.tmp" "$DEST/$file"
      printf 'ok (%s, sha256:%s)\n' \
        "$(du -h "$DEST/$file" | cut -f1 | tr -d ' ')" \
        "$(shasum -a 256 "$DEST/$file" | cut -c1-16)"
    else
      rm -f "$DEST/$file.tmp"; printf 'FAILED (not a valid OpenAPI document)\n'; fail=1
    fi
  else
    rm -f "$DEST/$file.tmp"; printf 'FAILED (download)\n'; fail=1
  fi
done <<< "$SPECS"

[ "$fail" -eq 0 ] || { echo "✗ spec-fetch incomplete" >&2; exit 1; }
echo "✓ specs in $DEST — run 'make spec-gen' to regenerate the catalog and manifest"
