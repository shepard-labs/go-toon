#!/usr/bin/env bash
# Vendors the language-agnostic TOON spec fixtures into toon/testdata/spec/.
#
# Usage: scripts/sync-spec-fixtures.sh [tag]
#
# The tag is pinned below; pass a tag (or set SPEC_TAG) only when intentionally
# bumping the targeted spec version. The script is idempotent: re-running it with
# the same tag leaves the working tree unchanged.
set -euo pipefail

SPEC_REPO="https://github.com/toon-format/spec.git"
SPEC_TAG="${1:-${SPEC_TAG:-v4.4.0}}"

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
dest="$root/toon/testdata/spec"

work="$(mktemp -d "${TMPDIR:-/tmp}/toon-spec.XXXXXX")"
trap 'rm -rf "$work"' EXIT

git -c advice.detachedHead=false clone --quiet --depth 1 --branch "$SPEC_TAG" "$SPEC_REPO" "$work/spec"
commit="$(git -C "$work/spec" rev-parse HEAD)"

for required in tests/fixtures/encode tests/fixtures/decode tests/fixtures.schema.json tests/README.md LICENSE; do
  [ -e "$work/spec/$required" ] || { echo "missing $required in $SPEC_REPO@$SPEC_TAG" >&2; exit 1; }
done

rm -rf "$dest"
mkdir -p "$dest/fixtures"
cp -R "$work/spec/tests/fixtures/encode" "$dest/fixtures/encode"
cp -R "$work/spec/tests/fixtures/decode" "$dest/fixtures/decode"
cp "$work/spec/tests/fixtures.schema.json" "$work/spec/tests/README.md" "$work/spec/LICENSE" "$dest/"
printf 'repo: %s\ntag: %s\ncommit: %s\n' "$SPEC_REPO" "$SPEC_TAG" "$commit" > "$dest/SOURCE"

echo "synced $SPEC_TAG ($commit) into ${dest#"$root"/}"
