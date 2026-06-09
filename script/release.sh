#!/bin/bash
set -euo pipefail

# release.sh - build, publish to GitHub, update homebrew formula, update nightly
# Usage: ./script/release.sh [patch|minor|major]
# Default: patch (v0.12.2 -> v0.12.3)

BUMP="${1:-patch}"
REPO="Miragefl/logview"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_DIR"

# check clean working tree
if [[ -n $(git status --porcelain) ]]; then
    echo "ERROR: working tree not clean. commit or stash first."
    git status -s
    exit 1
fi

# get current version from latest tag
CURRENT=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
CURRENT=${CURRENT#v}
IFS='.' read -r MAJOR MINOR PATCH <<< "$CURRENT"

case "$BUMP" in
    major) NEXT="$((MAJOR+1)).0.0" ;;
    minor) NEXT="$MAJOR.$((MINOR+1)).0" ;;
    patch) NEXT="$MAJOR.$MINOR.$((PATCH+1))" ;;
    *)     echo "ERROR: unknown bump type: $BUMP (use patch|minor|major)"; exit 1 ;;
esac

echo "==> Release v$NEXT (current: v$CURRENT)"
echo "    repo:  $REPO"
echo "    bump:  $BUMP"
echo ""
read -rp "    proceed? [y/N] " confirm
[[ "$confirm" != "y" && "$confirm" != "Y" ]] && echo "aborted." && exit 0

# tag and push
echo "==> tagging v$NEXT"
git tag "v$NEXT"
git push origin "v$NEXT"

# goreleaser
echo "==> goreleaser release"
GITHUB_TOKEN=$(gh auth token) goreleaser release --clean

# upgrade local brew
echo "==> upgrading local brew"
brew update && brew upgrade logview

# update nightly
echo "==> updating nightly release"
gh release delete nightly --yes 2>/dev/null || true
git push origin --delete nightly 2>/dev/null || true
git tag -d nightly 2>/dev/null || true
git tag nightly
git push origin nightly

GITHUB_TOKEN=$(gh auth token) goreleaser release --clean --snapshot

SNAPSHOT_ARTIFACTS=()
for f in dist/*.tar.gz; do
    SNAPSHOT_ARTIFACTS+=("$f")
done
if [[ ${#SNAPSHOT_ARTIFACTS[@]} -eq 0 ]]; then
    echo "ERROR: no snapshot artifacts found in dist/"
    exit 1
fi
gh release create nightly \
    --title "Nightly ($(date +%Y-%m-%d))" \
    --prerelease \
    --notes "Automated nightly build from latest main" \
    "${SNAPSHOT_ARTIFACTS[@]}"

echo ""
echo "==> done! v$NEXT published"
echo "    release: https://github.com/$REPO/releases/tag/v$NEXT"
echo "    nightly: https://github.com/$REPO/releases/tag/nightly"
echo "    local:   $(logview version 2>/dev/null || echo 'unknown')"
