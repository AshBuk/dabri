#!/bin/bash
# Script to create SRPM for dabri with vendored Go dependencies
# Requires: git, curl, rpm-build, go-vendor-tools
# Usage: ./create-srpm.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPEC_FILE="$SCRIPT_DIR/dabri.spec.in"
PROJECT_ROOT="$(dirname "$(dirname "$SCRIPT_DIR")")"

# Get version from git tag (single source of truth)
APP_VERSION_RAW="${APP_VERSION:-${GITHUB_REF_NAME:-$(git -C "$PROJECT_ROOT" describe --tags --abbrev=0 2>/dev/null || echo "0.0.0")}}"
APP_VERSION=$(echo "${APP_VERSION_RAW}" | sed 's/^v//')

# Patch spec file with version from git tag
sed -i "s/^Version:.*/Version:                ${APP_VERSION}/" "$SPEC_FILE"

WHISPER_VERSION=$(grep -E '^%global whisper_version' "$SPEC_FILE" | awk '{print $3}')

echo "=== Creating SRPM for dabri v${APP_VERSION} ==="
echo "whisper.cpp version: ${WHISPER_VERSION}"

# Create rpmbuild directory structure
RPMBUILD_DIR="$HOME/rpmbuild"
mkdir -p "$RPMBUILD_DIR"/{BUILD,RPMS,SOURCES,SPECS,SRPMS}

# Copy spec file
cp "$SPEC_FILE" "$RPMBUILD_DIR/SPECS/dabri.spec"

cd "$RPMBUILD_DIR/SOURCES"

# =============================================================================
# Source0: the upstream tarball, built from HEAD the way GitHub builds it for a tag
# =============================================================================
SOURCE_TARBALL="dabri-${APP_VERSION}.tar.gz"
echo "Creating source tarball: $SOURCE_TARBALL"
git -C "$PROJECT_ROOT" archive --format=tar.gz --prefix="dabri-${APP_VERSION}/" \
    -o "$RPMBUILD_DIR/SOURCES/$SOURCE_TARBALL" HEAD

# =============================================================================
# Source1, Source2: vendored Go modules and the go-vendor-tools config
# =============================================================================
cp "$SCRIPT_DIR/go-vendor-tools.toml" "$RPMBUILD_DIR/SOURCES/"
(cd "$RPMBUILD_DIR/SOURCES" && \
    go_vendor_archive create --config go-vendor-tools.toml \
        -O "dabri-${APP_VERSION}-vendor.tar.bz2" "$SOURCE_TARBALL")
ls -lh "$RPMBUILD_DIR/SOURCES/dabri-${APP_VERSION}"*

# =============================================================================
# Source3: Download whisper.cpp tarball
# =============================================================================
cd "$RPMBUILD_DIR/SOURCES"
WHISPER_TARBALL="whisper-cpp-${WHISPER_VERSION}.tar.gz"
if [[ ! -f "$WHISPER_TARBALL" ]]; then
    echo "Downloading whisper.cpp v${WHISPER_VERSION}..."
    curl -fL -o "$WHISPER_TARBALL" \
        "https://github.com/ggml-org/whisper.cpp/archive/refs/tags/v${WHISPER_VERSION}.tar.gz"
fi

# =============================================================================
# Source4: Download transcribe.cpp tarball
# =============================================================================
TRANSCRIBE_VERSION=$(grep -E '^%global transcribe_version' "$SPEC_FILE" | awk '{print $3}')
TRANSCRIBE_TARBALL="transcribe-cpp-${TRANSCRIBE_VERSION}.tar.gz"
if [[ ! -f "$TRANSCRIBE_TARBALL" ]]; then
    echo "Downloading transcribe.cpp v${TRANSCRIBE_VERSION}..."
    curl -fL -o "$TRANSCRIBE_TARBALL" \
        "https://github.com/handy-computer/transcribe.cpp/archive/refs/tags/v${TRANSCRIBE_VERSION}.tar.gz"
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
