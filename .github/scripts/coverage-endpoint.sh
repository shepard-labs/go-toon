#!/usr/bin/env bash
set -euo pipefail

coverprofile="${1:?usage: coverage-endpoint.sh <coverprofile> <out-dir>}"

out_dir="${2:?usage: coverage-endpoint.sh <coverprofile> <out-dir>}"
mkdir -p "$out_dir"

pct="$(go tool cover -func="$coverprofile" | awk '/^total:/ {gsub(/%/, "", $3); print $3}')"
pct_int="${pct%.*}"

color=brightgreen
if [ "$pct_int" -lt 60 ]; then
  color=red
elif [ "$pct_int" -lt 80 ]; then
  color=yellow
fi

cat >"$out_dir/endpoint.json" <<EOF
{
  "schemaVersion": 1,
  "label": "coverage",
  "message": "${pct}%",
  "color": "${color}"
}
EOF

echo "coverage=${pct}%"