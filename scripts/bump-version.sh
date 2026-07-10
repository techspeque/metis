#!/bin/bash
set -euo pipefail

# bump-version.sh — Bump version, create tag, and push to trigger release.
#
# Usage:
#   ./scripts/bump-version.sh major    # v0.1.0 → v1.0.0
#   ./scripts/bump-version.sh minor    # v0.1.0 → v0.2.0
#   ./scripts/bump-version.sh fix      # v0.1.0 → v0.1.1
#
# Flags:
#   --dry-run    Show what would happen without doing it
#   --no-push    Create tag locally but don't push

DRY_RUN=false
NO_PUSH=false
BUMP=""

# Parse args
for arg in "$@"; do
  case "$arg" in
    major|minor|fix) BUMP="$arg" ;;
    --dry-run) DRY_RUN=true ;;
    --no-push) NO_PUSH=true ;;
    -h|--help)
      echo "Usage: $0 <major|minor|fix> [--dry-run] [--no-push]"
      echo ""
      echo "Bumps the version, creates a git tag, and pushes to trigger a release."
      echo ""
      echo "  major    v0.1.2 → v1.0.0"
      echo "  minor    v0.1.2 → v0.2.0"
      echo "  fix      v0.1.2 → v0.1.3"
      echo ""
      echo "Flags:"
      echo "  --dry-run    Show what would happen without doing it"
      echo "  --no-push    Create tag locally but don't push"
      exit 0
      ;;
    *)
      echo "Error: unknown argument '$arg'"
      echo "Usage: $0 <major|minor|fix> [--dry-run] [--no-push]"
      exit 1
      ;;
  esac
done

if [ -z "$BUMP" ]; then
  echo "Error: must specify bump type (major, minor, or fix)"
  echo "Usage: $0 <major|minor|fix> [--dry-run] [--no-push]"
  exit 1
fi

# Get current version from latest tag
CURRENT=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
echo "Current version: $CURRENT"

# Strip 'v' prefix and parse
VERSION="${CURRENT#v}"
IFS='.' read -r MAJOR MINOR PATCH <<< "$VERSION"

# Bump
case "$BUMP" in
  major)
    MAJOR=$((MAJOR + 1))
    MINOR=0
    PATCH=0
    ;;
  minor)
    MINOR=$((MINOR + 1))
    PATCH=0
    ;;
  fix)
    PATCH=$((PATCH + 1))
    ;;
esac

NEW_VERSION="v${MAJOR}.${MINOR}.${PATCH}"
echo "New version:     $NEW_VERSION"
echo ""

if [ "$DRY_RUN" = true ]; then
  echo "[dry-run] Would create tag: $NEW_VERSION"
  echo "[dry-run] Would push tag to origin"
  echo "[dry-run] GitHub Actions would build and release"
  exit 0
fi

# Confirm
read -p "Release $NEW_VERSION? (y/n) " -n 1 -r
echo ""
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
  echo "Aborted."
  exit 0
fi

# Ensure we're on main and up to date
BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [ "$BRANCH" != "main" ]; then
  echo "Warning: you're on '$BRANCH', not 'main'."
  read -p "Continue anyway? (y/n) " -n 1 -r
  echo ""
  if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Aborted. Switch to main first: git checkout main"
    exit 0
  fi
fi

# Create tag
echo "Creating tag $NEW_VERSION..."
git tag -a "$NEW_VERSION" -m "Release $NEW_VERSION"

if [ "$NO_PUSH" = true ]; then
  echo "Tag created locally (--no-push). Push manually with:"
  echo "  git push origin $NEW_VERSION"
  exit 0
fi

# Push tag
echo "Pushing tag to origin..."
git push origin "$NEW_VERSION"

echo ""
echo "Done! Release $NEW_VERSION has been triggered."
echo "GitHub Actions will build and publish binaries."
echo "Check: https://github.com/techspeque/metis/actions"
