#!/bin/bash
# OpenLoadBalancer (OLB) - Homebrew Formula Updater
#
# Updates the Homebrew formula with a new version and SHA256 hash,
# and optionally creates a PR to the homebrew tap repository.
#
# Usage:
#   ./scripts/update-homebrew.sh <version> <sha256>
#   ./scripts/update-homebrew.sh v0.2.0 abc123def456...
#   ./scripts/update-homebrew.sh --version v0.2.0 --sha256 abc123...
#   ./scripts/update-homebrew.sh --version v0.2.0 --auto-sha256
#   ./scripts/update-homebrew.sh --version v0.2.0 --auto-sha256 --push-tap
#
# Options:
#   --version, -v       Release version (e.g., v0.2.0)
#   --sha256, -s        SHA256 hash of the source tarball
#   --auto-sha256       Automatically download tarball and compute SHA256
#   --push-tap          Clone the homebrew tap repo, update, and create a PR
#   --tap-repo          Tap repository (default: openloadbalancer/homebrew-olb)
#   --dry-run           Show what would be changed without writing
#   --help, -h          Show this help message

set -euo pipefail

# ─── Constants ────────────────────────────────────────────────────────
GITHUB_REPO="openloadbalancer/olb"
FORMULA_FILE="Formula/olb.rb"

# ─── Defaults ─────────────────────────────────────────────────────────
VERSION=""
SHA256=""
AUTO_SHA256=0
PUSH_TAP=0
TAP_REPO="openloadbalancer/homebrew-olb"
DRY_RUN=0

# ─── Resolve paths ───────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
FORMULA_PATH="${PROJECT_ROOT}/${FORMULA_FILE}"

# ─── Color output ────────────────────────────────────────────────────
if [ -t 1 ]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[0;33m'
    BLUE='\033[0;34m'
    BOLD='\033[1m'
    RESET='\033[0m'
else
    RED=''
    GREEN=''
    YELLOW=''
    BLUE=''
    BOLD=''
    RESET=''
fi

info()    { printf "${BLUE}==>${RESET} ${BOLD}%s${RESET}\n" "$1"; }
success() { printf "${GREEN}==>${RESET} ${BOLD}%s${RESET}\n" "$1"; }
warn()    { printf "${YELLOW}Warning:${RESET} %s\n" "$1" >&2; }
error()   { printf "${RED}Error:${RESET} %s\n" "$1" >&2; exit 1; }

# ─── Parse arguments ─────────────────────────────────────────────────
# Support both positional and flag-based arguments
if [ $# -ge 1 ] && [[ ! "$1" =~ ^-- ]]; then
    # Positional: <version> [sha256]
    VERSION="$1"
    shift
    if [ $# -ge 1 ] && [[ ! "$1" =~ ^-- ]]; then
        SHA256="$1"
        shift
    fi
fi

while [ $# -gt 0 ]; do
    case "$1" in
        --version|-v)
            VERSION="$2"
            shift 2
            ;;
        --version=*)
            VERSION="${1#*=}"
            shift
            ;;
        --sha256|-s)
            SHA256="$2"
            shift 2
            ;;
        --sha256=*)
            SHA256="${1#*=}"
            shift
            ;;
        --auto-sha256)
            AUTO_SHA256=1
            shift
            ;;
        --push-tap)
            PUSH_TAP=1
            shift
            ;;
        --tap-repo)
            TAP_REPO="$2"
            shift 2
            ;;
        --tap-repo=*)
            TAP_REPO="${1#*=}"
            shift
            ;;
        --dry-run)
            DRY_RUN=1
            shift
            ;;
        --help|-h)
            echo "OpenLoadBalancer Homebrew Formula Updater"
            echo ""
            echo "Usage:"
            echo "  $0 <version> <sha256>"
            echo "  $0 --version <version> --sha256 <sha256>"
            echo "  $0 --version <version> --auto-sha256"
            echo "  $0 --version <version> --auto-sha256 --push-tap"
            echo ""
            echo "Options:"
            echo "  --version, -v <ver>     Release version (e.g., v0.2.0)"
            echo "  --sha256, -s <hash>     SHA256 hash of the source tarball"
            echo "  --auto-sha256           Download tarball and compute SHA256 automatically"
            echo "  --push-tap              Clone tap repo, update formula, and create a PR"
            echo "  --tap-repo <repo>       Tap repository (default: ${TAP_REPO})"
            echo "  --dry-run               Show changes without writing"
            echo "  --help, -h              Show this help"
            exit 0
            ;;
        *)
            error "Unknown option: $1. Use --help for usage."
            ;;
    esac
done

# ─── Validate inputs ────────────────────────────────────────────────
if [ -z "$VERSION" ]; then
    error "Version is required. Usage: $0 <version> <sha256>"
fi

# Normalize version: strip leading 'v' for formula, keep for URLs
VERSION_RAW="$VERSION"
VERSION="${VERSION#v}"  # e.g., "0.2.0"
VERSION_TAG="v${VERSION}"  # e.g., "v0.2.0"

TARBALL_URL="https://github.com/${GITHUB_REPO}/archive/refs/tags/${VERSION_TAG}.tar.gz"

# ─── Compute SHA256 if --auto-sha256 ────────────────────────────────
if [ "$AUTO_SHA256" -eq 1 ]; then
    if [ -n "$SHA256" ]; then
        warn "Both --sha256 and --auto-sha256 provided; --auto-sha256 takes precedence"
    fi

    info "Downloading tarball to compute SHA256..."
    info "URL: ${TARBALL_URL}"

    TMP_DIR="$(mktemp -d)"
    trap 'rm -rf "$TMP_DIR"' EXIT

    TMP_TARBALL="${TMP_DIR}/olb-${VERSION_TAG}.tar.gz"

    if command -v curl >/dev/null 2>&1; then
        curl -fsSL -o "$TMP_TARBALL" "$TARBALL_URL" || \
            error "Failed to download tarball. Does release ${VERSION_TAG} exist?"
    elif command -v wget >/dev/null 2>&1; then
        wget -qO "$TMP_TARBALL" "$TARBALL_URL" || \
            error "Failed to download tarball. Does release ${VERSION_TAG} exist?"
    else
        error "Neither curl nor wget found. Cannot download tarball."
    fi

    if command -v sha256sum >/dev/null 2>&1; then
        SHA256="$(sha256sum "$TMP_TARBALL" | awk '{print $1}')"
    elif command -v shasum >/dev/null 2>&1; then
        SHA256="$(shasum -a 256 "$TMP_TARBALL" | awk '{print $1}')"
    else
        error "Neither sha256sum nor shasum found. Cannot compute hash."
    fi

    success "SHA256: ${SHA256}"
fi

if [ -z "$SHA256" ]; then
    error "SHA256 is required. Provide --sha256 <hash> or use --auto-sha256."
fi

# Validate SHA256 format (64 hex characters)
if ! echo "$SHA256" | grep -qE '^[0-9a-fA-F]{64}$'; then
    error "Invalid SHA256 hash: '${SHA256}' (expected 64 hex characters)"
fi

# ─── Verify formula file exists ──────────────────────────────────────
if [ ! -f "$FORMULA_PATH" ]; then
    error "Formula file not found at ${FORMULA_PATH}"
fi

# ─── Show planned changes ───────────────────────────────────────────
echo ""
echo "========================================"
echo " Homebrew Formula Update"
echo "========================================"
echo " Version:    ${VERSION_TAG}"
echo " SHA256:     ${SHA256}"
echo " Formula:    ${FORMULA_PATH}"
echo " Tarball:    ${TARBALL_URL}"
if [ "$PUSH_TAP" -eq 1 ]; then
echo " Tap repo:   ${TAP_REPO}"
fi
echo "========================================"
echo ""

# ─── Read current values ────────────────────────────────────────────
CURRENT_URL="$(grep '^\s*url ' "$FORMULA_PATH" | head -1 | sed 's/.*"\(.*\)".*/\1/')"
CURRENT_SHA="$(grep '^\s*sha256 ' "$FORMULA_PATH" | head -1 | sed 's/.*"\(.*\)".*/\1/')"

info "Current URL:    ${CURRENT_URL}"
info "Current SHA256: ${CURRENT_SHA}"
info "New URL:        ${TARBALL_URL}"
info "New SHA256:     ${SHA256}"

if [ "$CURRENT_URL" = "$TARBALL_URL" ] && [ "$CURRENT_SHA" = "$SHA256" ]; then
    success "Formula is already up to date for ${VERSION_TAG}"
    exit 0
fi

# ─── Update formula ─────────────────────────────────────────────────
if [ "$DRY_RUN" -eq 1 ]; then
    info "[DRY RUN] Would update formula with:"
    echo "  url \"${TARBALL_URL}\""
    echo "  sha256 \"${SHA256}\""
    echo ""
    info "[DRY RUN] No files modified."
else
    info "Updating formula..."

    # Update the url line
    sed -i.bak "s|^\(\s*\)url \".*\"|\\1url \"${TARBALL_URL}\"|" "$FORMULA_PATH"

    # Update the sha256 line
    sed -i.bak "s|^\(\s*\)sha256 \".*\"|\\1sha256 \"${SHA256}\"|" "$FORMULA_PATH"

    # Clean up backup files
    rm -f "${FORMULA_PATH}.bak"

    success "Formula updated: ${FORMULA_PATH}"

    # Show diff
    if command -v git >/dev/null 2>&1 && git -C "$PROJECT_ROOT" rev-parse --git-dir >/dev/null 2>&1; then
        echo ""
        info "Changes:"
        git -C "$PROJECT_ROOT" diff -- "$FORMULA_FILE" || true
    fi
fi

# ─── Push to homebrew tap (optional) ────────────────────────────────
if [ "$PUSH_TAP" -eq 1 ]; then
    if [ "$DRY_RUN" -eq 1 ]; then
        info "[DRY RUN] Would clone ${TAP_REPO}, update formula, and create PR"
    else
        info "Updating homebrew tap repository..."

        # Verify gh CLI is available
        if ! command -v gh >/dev/null 2>&1; then
            error "GitHub CLI (gh) is required for --push-tap. Install: https://cli.github.com"
        fi

        # Verify authenticated
        if ! gh auth status >/dev/null 2>&1; then
            error "GitHub CLI not authenticated. Run: gh auth login"
        fi

        TAP_TMP="$(mktemp -d)"
        TAP_CLEANUP() { rm -rf "$TAP_TMP"; }

        info "Cloning tap: ${TAP_REPO}..."
        gh repo clone "$TAP_REPO" "${TAP_TMP}/tap" -- --depth 1 || \
            error "Failed to clone tap repository: ${TAP_REPO}"

        TAP_FORMULA="${TAP_TMP}/tap/Formula/olb.rb"

        if [ ! -f "$TAP_FORMULA" ]; then
            # Formula might be at root level in the tap
            TAP_FORMULA="${TAP_TMP}/tap/olb.rb"
            if [ ! -f "$TAP_FORMULA" ]; then
                info "Formula not found in tap; copying from source..."
                mkdir -p "${TAP_TMP}/tap/Formula"
                TAP_FORMULA="${TAP_TMP}/tap/Formula/olb.rb"
                cp "$FORMULA_PATH" "$TAP_FORMULA"
            fi
        fi

        # Update url and sha256 in the tap formula
        sed -i.bak "s|^\(\s*\)url \".*\"|\\1url \"${TARBALL_URL}\"|" "$TAP_FORMULA"
        sed -i.bak "s|^\(\s*\)sha256 \".*\"|\\1sha256 \"${SHA256}\"|" "$TAP_FORMULA"
        rm -f "${TAP_FORMULA}.bak"

        # Create branch, commit, and push
        BRANCH_NAME="update-olb-${VERSION}"

        cd "${TAP_TMP}/tap"
        git checkout -b "$BRANCH_NAME"
        git add -A
        git commit -m "olb ${VERSION_TAG}

Update OLB formula to ${VERSION_TAG}.

Source: ${TARBALL_URL}
SHA256: ${SHA256}"

        git push origin "$BRANCH_NAME"

        # Create PR
        PR_URL="$(gh pr create \
            --title "olb ${VERSION_TAG}" \
            --body "Update OLB Homebrew formula to ${VERSION_TAG}.

- **Version**: ${VERSION_TAG}
- **Source**: ${TARBALL_URL}
- **SHA256**: \`${SHA256}\`

Automated update by \`scripts/update-homebrew.sh\`." \
            --head "$BRANCH_NAME" \
            --base "main" 2>&1)"

        success "Pull request created: ${PR_URL}"
        TAP_CLEANUP

        cd "$PROJECT_ROOT"
    fi
fi

# ─── Summary ─────────────────────────────────────────────────────────
echo ""
echo "========================================"
echo " Update complete"
echo "========================================"
echo ""
echo "  Formula: ${FORMULA_PATH}"
echo "  Version: ${VERSION_TAG}"
echo "  SHA256:  ${SHA256}"
echo ""
if [ "$PUSH_TAP" -eq 0 ] && [ "$DRY_RUN" -eq 0 ]; then
echo "  Next steps:"
echo "    1. Commit the updated formula:"
echo "       git add ${FORMULA_FILE}"
echo "       git commit -m 'brew: update formula to ${VERSION_TAG}'"
echo ""
echo "    2. If using a separate homebrew tap, also update it:"
echo "       $0 --version ${VERSION_TAG} --sha256 ${SHA256} --push-tap"
echo ""
fi
