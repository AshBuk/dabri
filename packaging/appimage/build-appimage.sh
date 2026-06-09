#!/bin/bash

# Dabri AppImage builder script

set -e  # Exit on error
set -x  # Show commands being executed

# Configuration
APP_NAME="dabri"
APP_VERSION_RAW="${APP_VERSION:-${GITHUB_REF_NAME:-$(git describe --tags --abbrev=0 2>/dev/null || date +%Y%m%d)}}"
APP_VERSION=$(echo "${APP_VERSION_RAW}" | sed 's/^v//')
ARCH="x86_64"
OUTPUT_DIR="dist"
APPDIR="${OUTPUT_DIR}/${APP_NAME}.AppDir"
TOOLS_DIR="$(pwd)/tools"
MODEL_URL="${MODEL_URL:-https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small-q5_1.bin}"
MODEL_SHA256="${MODEL_SHA256:-ae85e4a935d7a567bd102fe55afc16bb595bdb618e11b2fc7591bc08120411bb}"
LINUXDEPLOY_TAG="${LINUXDEPLOY_TAG:-1-alpha-20251107-1}"
LINUXDEPLOY_SHA256="${LINUXDEPLOY_SHA256:-c20cd71e3a4e3b80c3483cef793cda3f4e990aca14014d23c544ca3ce1270b4d}"
APPIMAGETOOL_TAG="${APPIMAGETOOL_TAG:-continuous}"
APPIMAGETOOL_SHA256="${APPIMAGETOOL_SHA256:-b90f4a8b18967545fda78a445b27680a1642f1ef9488ced28b65398f2be7add2}"
GTK_PLUGIN_COMMIT="${GTK_PLUGIN_COMMIT:-3b67a1d1c1b0c8268f57f2bce40fe2d33d409cea}"
GTK_PLUGIN_SHA256="${GTK_PLUGIN_SHA256:-b0f4cbc684a0103a9651f0955b635eaea0096b3a66c0f5a2c2aa337960375171}"
# Determine script directory (works both locally and in Docker)
if [ -f "packaging/appimage/AppRun" ]; then
    SCRIPT_DIR="$(pwd)/packaging/appimage"
else
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
fi

echo "=== Starting AppImage build for ${APP_NAME} v${APP_VERSION} ==="

verify_sha256() {
    local file="$1"
    local expected="$2"

    if [ -z "${expected}" ]; then
        echo "Missing SHA256 for ${file}"
        exit 1
    fi
    echo "${expected}  ${file}" | sha256sum -c -
}

download_verified() {
    local url="$1"
    local output="$2"
    local sha256="$3"

    if [ -f "${output}" ]; then
        verify_sha256 "${output}" "${sha256}"
        return
    fi

    local tmp="${output}.tmp"
    rm -f "${tmp}"
    curl -fsSL --retry 3 --connect-timeout 30 "${url}" -o "${tmp}"
    verify_sha256 "${tmp}" "${sha256}"
    mv "${tmp}" "${output}"
}

# Prepare build environment
prepare_environment() {
    echo "Preparing environment..."
    if [ -f "bash-scripts/dev-env.sh" ]; then
        source bash-scripts/dev-env.sh || true
    fi
    if [ ! -f "lib/whisper.h" ]; then
        make internal-whisper-libs
    fi
}

# Download Whisper model if missing
download_model() {
    if [ -f "sources/language-models/small-q5_1.bin" ]; then
        return
    fi
    echo "Downloading Whisper small-q5_1 model..."
    mkdir -p sources/language-models
    download_verified "${MODEL_URL}" "sources/language-models/small-q5_1.bin" "${MODEL_SHA256}"
}

# Create AppDir structure and build application
build_appdir() {
    echo "Creating AppDir structure..."
    mkdir -p "${APPDIR}"/{usr/bin,usr/lib,usr/share/applications,usr/share/icons/hicolor/256x256/apps,usr/share/metainfo,sources/language-models}

    # Build application
    echo "Building ${APP_NAME}..."
    go build -tags systray,gtk -ldflags "-s -w -X github.com/AshBuk/dabri/v2/internal/version.Version=${APP_VERSION}" -o "${APP_NAME}" ./cmd/dabri
    cp "${APP_NAME}" "${APPDIR}/usr/bin/"

    # Copy Whisper libraries
    cp -a lib/libwhisper.so* lib/libggml*.so* "${APPDIR}/usr/lib/" 2>/dev/null || true

    # Copy Whisper model
    [ -f "sources/language-models/small-q5_1.bin" ] && \
        cp sources/language-models/small-q5_1.bin "${APPDIR}/sources/language-models/"

    # Copy external binaries
    for bin in xsel wl-copy wtype ydotool notify-send arecord xdotool ffmpeg; do
        local path=$(which "$bin" 2>/dev/null || true)
        [ -n "$path" ] && cp "$path" "${APPDIR}/usr/bin/" && echo "Bundled: $bin"
    done

    # Copy AppRun from template
    cp "${SCRIPT_DIR}/AppRun" "${APPDIR}/AppRun"
    chmod +x "${APPDIR}/AppRun"

    # Copy desktop file and icon (use existing files from repo)
    cp "io.github.ashbuk.dabri.desktop" "${APPDIR}/usr/share/applications/${APP_NAME}.desktop"
    cp "io.github.ashbuk.dabri.appdata.xml" "${APPDIR}/usr/share/metainfo/${APP_NAME}.appdata.xml"
    # Copy icon with name matching Icon= field in .desktop file
    mkdir -p "${APPDIR}/usr/share/icons/hicolor/128x128/apps"
    cp "icons/io.github.ashbuk.dabri.png" "${APPDIR}/usr/share/icons/hicolor/128x128/apps/io.github.ashbuk.dabri.png"
    cp "icons/io.github.ashbuk.dabri.png" "${APPDIR}/usr/share/icons/hicolor/128x128/apps/${APP_NAME}.png"

    # Create symlinks for AppImage standard
    ln -sf "usr/share/applications/${APP_NAME}.desktop" "${APPDIR}/${APP_NAME}.desktop"
    ln -sf "usr/share/icons/hicolor/128x128/apps/${APP_NAME}.png" "${APPDIR}/${APP_NAME}.png"
    ln -sf "usr/share/icons/hicolor/128x128/apps/io.github.ashbuk.dabri.png" "${APPDIR}/io.github.ashbuk.dabri.png"
}

# Download AppImage tools
download_tools() {
    mkdir -p "${TOOLS_DIR}"
    local base_url="https://github.com"

    download_verified \
        "${base_url}/linuxdeploy/linuxdeploy/releases/download/${LINUXDEPLOY_TAG}/linuxdeploy-${ARCH}.AppImage" \
        "${TOOLS_DIR}/linuxdeploy-${ARCH}.AppImage" \
        "${LINUXDEPLOY_SHA256}"
    chmod +x "${TOOLS_DIR}/linuxdeploy-${ARCH}.AppImage"

    download_verified \
        "${base_url}/AppImage/AppImageKit/releases/download/${APPIMAGETOOL_TAG}/appimagetool-${ARCH}.AppImage" \
        "${TOOLS_DIR}/appimagetool-${ARCH}.AppImage" \
        "${APPIMAGETOOL_SHA256}"
    chmod +x "${TOOLS_DIR}/appimagetool-${ARCH}.AppImage"

    download_verified \
        "https://raw.githubusercontent.com/linuxdeploy/linuxdeploy-plugin-gtk/${GTK_PLUGIN_COMMIT}/linuxdeploy-plugin-gtk.sh" \
        "${TOOLS_DIR}/linuxdeploy-plugin-gtk.sh" \
        "${GTK_PLUGIN_SHA256}"
    chmod +x "${TOOLS_DIR}/linuxdeploy-plugin-gtk.sh"
}

# Find and bundle system library
find_lib() {
    local pattern="$1"
    for d in /usr/lib/x86_64-linux-gnu /usr/lib64 /usr/lib; do
        local lib=$(ls -1 $d/${pattern}* 2>/dev/null | sort -V | tail -n1 || true)
        [ -n "$lib" ] && echo "$lib" && return
    done
}

# Build final AppImage
build_appimage() {
    cd "${OUTPUT_DIR}"
    export ARCH VERSION="${APP_VERSION}"

    # Build executable arguments for linuxdeploy
    local exec_args=""
    for exe in ${APP_NAME} xdotool wtype ydotool wl-copy xsel arecord notify-send; do
        [ -f "${APP_NAME}.AppDir/usr/bin/${exe}" ] && \
            exec_args="$exec_args --executable ${APP_NAME}.AppDir/usr/bin/${exe}"
    done

    # Build library arguments for system tray support
    local lib_args=""
    for lib in libayatana-appindicator3.so libayatana-indicator3.so libdbusmenu-gtk3.so libdbusmenu-glib.so libappindicator3.so libindicator3.so; do
        local found=$(find_lib "$lib")
        [ -n "$found" ] && lib_args="$lib_args --library $found" && echo "Will bundle: $found"
    done

    # GTK version must be set explicitly — auto-detection fails on Go binaries
    export DEPLOY_GTK_VERSION=3

    # Step 1: Use linuxdeploy to gather dependencies
    "${TOOLS_DIR}/linuxdeploy-${ARCH}.AppImage" --appimage-extract-and-run \
        --appdir "${APP_NAME}.AppDir" \
        $exec_args \
        $lib_args \
        --desktop-file "${APP_NAME}.AppDir/${APP_NAME}.desktop" \
        --icon-file "${APP_NAME}.AppDir/io.github.ashbuk.dabri.png" \
        --plugin gtk

    # Remove unnecessary docs (licenses available in source repos)
    rm -rf "${APP_NAME}.AppDir/usr/share/doc"

    # Create final AppImage
    echo "Creating AppImage..."
    "${TOOLS_DIR}/appimagetool-${ARCH}.AppImage" --appimage-extract-and-run "${APP_NAME}.AppDir"

    # Rename output
    local output=$(find . -maxdepth 1 -name "*.AppImage" ! -name "appimagetool*" -type f | head -n1)
    if [ -n "$output" ]; then
        mv "$output" "${APP_NAME}-${APP_VERSION}-${ARCH}.AppImage"
        chmod +x "${APP_NAME}-${APP_VERSION}-${ARCH}.AppImage"
        ls -lh "${APP_NAME}-${APP_VERSION}-${ARCH}.AppImage"
        echo "=== Build completed successfully! ==="
    else
        echo "Error: AppImage not found"
        exit 1
    fi
}

# Main
main() {
    prepare_environment
    download_model
    build_appdir
    download_tools
    build_appimage
}

main "$@"
