#!/bin/bash
# Script to create SRPM for dabri with vendored Go dependencies
# Usage: ./create-srpm.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPEC_FILE="$SCRIPT_DIR/dabri.spec"
PROJECT_ROOT="$(dirname "$(dirname "$SCRIPT_DIR")")"

# Get version from git tag (single source of truth)
APP_VERSION_RAW="${APP_VERSION:-${GITHUB_REF_NAME:-$(git -C "$PROJECT_ROOT" describe --tags --abbrev=0 2>/dev/null || echo "0.0.0")}}"
APP_VERSION=$(echo "${APP_VERSION_RAW}" | sed 's/^v//')

# Patch spec file with version from git tag
sed -i "s/^%global app_version.*/%global app_version     ${APP_VERSION}/" "$SPEC_FILE"

WHISPER_VERSION=$(grep -E '^%global whisper_version' "$SPEC_FILE" | awk '{print $3}')

echo "=== Creating SRPM for dabri v${APP_VERSION} ==="
echo "whisper.cpp version: ${WHISPER_VERSION}"

# Create rpmbuild directory structure
RPMBUILD_DIR="$HOME/rpmbuild"
mkdir -p "$RPMBUILD_DIR"/{BUILD,RPMS,SOURCES,SPECS,SRPMS}

# Copy spec file
cp "$SPEC_FILE" "$RPMBUILD_DIR/SPECS/"

cd "$RPMBUILD_DIR/SOURCES"

# =============================================================================
# Source0: Create vendored tarball
# =============================================================================
VENDORED_TARBALL="dabri-${APP_VERSION}-vendored.tar.gz"
echo "Creating vendored tarball: $VENDORED_TARBALL"

# Check if vendor directory exists
if [[ ! -d "$PROJECT_ROOT/vendor" ]]; then
    echo "ERROR: vendor/ directory not found!"
    echo "Run 'go mod vendor' or 'docker compose run --rm dev go mod vendor' first"
    exit 1
fi

# Create tarball with vendor/ included
cd "$PROJECT_ROOT"
tar --transform "s,^,dabri-${APP_VERSION}/," \
    --exclude='.git' \
    --exclude='build' \
    --exclude='dist' \
    --exclude='lib' \
    --exclude='*.AppImage' \
    --exclude='rpm/*.src.rpm' \
    -czf "$RPMBUILD_DIR/SOURCES/$VENDORED_TARBALL" \
    cmd/ config/ hotkeys/ audio/ whisper/ output/ websocket/ internal/ \
    vendor/ \
    go.mod go.sum \
    Makefile LICENSE README.md CHANGELOG.md \
    io.github.ashbuk.dabri.desktop \
    io.github.ashbuk.dabri.appdata.xml \
    icons/ docs/

echo "Created: $RPMBUILD_DIR/SOURCES/$VENDORED_TARBALL"
ls -lh "$RPMBUILD_DIR/SOURCES/$VENDORED_TARBALL"

# =============================================================================
# Source1: Download whisper.cpp tarball
# =============================================================================
cd "$RPMBUILD_DIR/SOURCES"
WHISPER_TARBALL="whisper-cpp-${WHISPER_VERSION}.tar.gz"
if [[ ! -f "$WHISPER_TARBALL" ]]; then
    echo "Downloading whisper.cpp v${WHISPER_VERSION}..."
    curl -L -o "$WHISPER_TARBALL" \
        "https://github.com/ggml-org/whisper.cpp/archive/refs/tags/v${WHISPER_VERSION}.tar.gz"
fi

# =============================================================================
# Build SRPM
# =============================================================================
echo "=== Building SRPM ==="
rpmbuild -bs "$RPMBUILD_DIR/SPECS/dabri.spec"

SRPM_PATH=$(ls -t "$RPMBUILD_DIR/SRPMS/dabri-"*.src.rpm 2>/dev/null | head -1)
if [[ -n "$SRPM_PATH" ]]; then
    echo ""
    echo "=== SRPM created successfully ==="
    echo "Path: $SRPM_PATH"
    echo "Size: $(ls -lh "$SRPM_PATH" | awk '{print $5}')"
    echo ""
    echo "Next steps:"
    echo "  1. Test build with mock (offline, like Koji):"
    echo "     mock -r fedora-rawhide-x86_64 $SRPM_PATH"
    echo ""
    echo "  2. Submit to COPR:"
    echo "     copr-cli build dabri $SRPM_PATH"
    echo ""
    echo "  3. For Fedora Review Request:"
    echo "     - Upload SRPM to fedorapeople.org or koji scratch build"
    echo "     - File bug at https://bugzilla.redhat.com/enter_bug.cgi?product=Fedora&component=Package%20Review"
else
    echo "ERROR: SRPM creation failed"
    exit 1
fi
