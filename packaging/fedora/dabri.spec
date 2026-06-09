# Copyright (c) 2025 Asher Buk
# SPDX-License-Identifier: MIT
# https://github.com/AshBuk/dabri

# =============================================================================
# Version definitions (single source of truth)
# =============================================================================
%global app_version     2.1.1
%global go_version      1.21
%global whisper_version 1.8.6

# Exclude auto-requires for vendored whisper libraries
%global __requires_exclude libwhisper\\.so|libggml.*\\.so

Name:           dabri
Version:        %{app_version}
Release:        1%{?dist}
Summary:        Offline speech-to-text desktop application
License:        MIT
URL:            https://github.com/AshBuk/dabri

# Main source with vendored Go dependencies
Source0:        %{name}-%{version}-vendored.tar.gz
# whisper.cpp sources (pinned version for reproducible builds)
Source1:        https://github.com/ggml-org/whisper.cpp/archive/refs/tags/v%{whisper_version}.tar.gz#/whisper-cpp-%{whisper_version}.tar.gz

ExclusiveArch:  x86_64

# =============================================================================
# Build dependencies
# =============================================================================
BuildRequires:  golang >= %{go_version}
BuildRequires:  gcc
BuildRequires:  gcc-c++
BuildRequires:  cmake
BuildRequires:  make

# CGO/whisper.cpp build requirements
BuildRequires:  pkgconfig

# Systray support (GTK/AppIndicator)
BuildRequires:  pkgconfig(gtk+-3.0)
BuildRequires:  pkgconfig(ayatana-appindicator3-0.1)

# D-Bus for hotkey support
BuildRequires:  pkgconfig(dbus-1)

# Vulkan SDK for GPU acceleration
BuildRequires:  vulkan-devel
BuildRequires:  glslc
# SPIRV-Headers cmake config - required by whisper.cpp >= v1.8.5 Vulkan backend
BuildRequires:  spirv-headers-devel

# =============================================================================
# Runtime dependencies
# =============================================================================
# Audio recording (one of these required)
Requires:       alsa-utils
# Note: ffmpeg available in RPM Fusion, documented as optional
Suggests:       ffmpeg

# Clipboard operations
Requires:       xsel
Recommends:     wl-clipboard

# Text input automation (X11: xdotool, Wayland: ydotool or wtype)
Requires:       xdotool
Recommends:     ydotool
Recommends:     wtype

# Desktop notifications
Requires:       libnotify

# System tray (GNOME needs extension, KDE works out of box)
Requires:       libayatana-appindicator-gtk3
Requires:       gtk3

# D-Bus for hotkeys
Requires:       dbus

# Vulkan runtime for GPU acceleration (optional, falls back to CPU)
Recommends:     vulkan-loader

# =============================================================================
# Bundled provides (Fedora requires declaring all bundled deps)
# =============================================================================
# whisper.cpp C library
Provides:       bundled(whisper-cpp) = %{whisper_version}

# Vendored Go dependencies (generated from vendor/modules.txt)
Provides:       bundled(golang(fyne.io/systray)) = 1.12.0
Provides:       bundled(golang(github.com/ggerganov/whisper.cpp/bindings/go)) = 0.0.0.20260319084013.9386f2394010
Provides:       bundled(golang(github.com/go-audio/audio)) = 1.0.0
Provides:       bundled(golang(github.com/go-audio/riff)) = 1.0.0
Provides:       bundled(golang(github.com/go-audio/wav)) = 1.1.0
Provides:       bundled(golang(github.com/godbus/dbus/v5)) = 5.2.2
Provides:       bundled(golang(github.com/gorilla/websocket)) = 1.5.3
Provides:       bundled(golang(github.com/gotk3/gotk3)) = 0.6.5.0.20251124190141.e7a9e823ca35
Provides:       bundled(golang(github.com/holoplot/go-evdev)) = 0.0.0.20250804134636.ab1d56a1fe83
Provides:       bundled(golang(github.com/kr/text)) = 0.2.0
Provides:       bundled(golang(go.uber.org/goleak)) = 1.3.0
Provides:       bundled(golang(golang.org/x/sys)) = 0.43.0
Provides:       bundled(golang(gopkg.in/yaml.v2)) = 2.4.0

%description
Dabri is a minimalist, privacy-focused desktop application for offline
speech-to-text. It converts voice input directly into any active window
(editors, browsers, IDEs, AI assistants) using the Whisper model locally.

Features:
- Offline speech-to-text with local processing (privacy-first)
- Cross-platform support for X11 and Wayland
- Native integration with GNOME, KDE, and other Linux DEs
- Voice typing or clipboard mode
- Flexible audio recording (ALSA or PulseAudio/PipeWire)
- Multi-language support, custom hotkey binding

The Whisper small-q5_1 model (~181 MB) is downloaded on first run.

Note: For evdev hotkey support (fallback method), add user to 'input' group:
  sudo usermod -a -G input $USER

%prep
%autosetup -n %{name}-%{version}

# Unpack whisper.cpp into build directory
mkdir -p build
tar -xzf %{SOURCE1} -C build
mv build/whisper.cpp-%{whisper_version} build/whisper.cpp

%build
# =============================================================================
# 1) Build whisper.cpp libraries
# =============================================================================
# Set RPATH for whisper libraries to find each other in private prefix
WHISPER_RPATH='%{_libdir}/%{name}'

pushd build/whisper.cpp
cmake -B build \
    -DCMAKE_BUILD_TYPE=Release \
    -DBUILD_SHARED_LIBS=ON \
    -DCMAKE_INSTALL_RPATH="$WHISPER_RPATH" \
    -DCMAKE_BUILD_WITH_INSTALL_RPATH=ON \
    -DGGML_NATIVE=OFF \
    -DGGML_AVX=ON \
    -DGGML_AVX2=ON \
    -DGGML_FMA=ON \
    -DGGML_F16C=ON \
    -DGGML_VULKAN=ON
cmake --build build --parallel %{_smp_build_ncpus}
popd

# Prepare lib directory for Go build
mkdir -p lib
cp build/whisper.cpp/build/src/libwhisper.so* lib/
cp build/whisper.cpp/include/whisper.h lib/
cp build/whisper.cpp/ggml/include/*.h lib/ 2>/dev/null || :
cp build/whisper.cpp/build/ggml/src/libggml*.so* lib/ 2>/dev/null || :
# Copy Vulkan backend library from subdirectory
cp build/whisper.cpp/build/ggml/src/ggml-vulkan/libggml-vulkan.so* lib/ 2>/dev/null || :

# =============================================================================
# 2) Build Go binary with systray support (using vendored deps)
# =============================================================================
export CGO_ENABLED=1
export C_INCLUDE_PATH=$(pwd)/lib
export LIBRARY_PATH=$(pwd)/lib
export CGO_CFLAGS="-I$(pwd)/lib"
export CGO_LDFLAGS="-L$(pwd)/lib -lwhisper -lggml -lggml-cpu -lggml-vulkan"
export LD_LIBRARY_PATH=$(pwd)/lib

# Set RPATH at build time to find bundled libraries at runtime
VENDOR_RPATH='$ORIGIN/../lib64/%{name}'
go build -v \
    -mod=vendor \
    -tags systray,gtk \
    -ldflags "-s -w -X github.com/AshBuk/dabri/v2/internal/version.Version=%{version} -linkmode=external -extldflags '-Wl,-rpath,${VENDOR_RPATH}'" \
    -o %{name} \
    ./cmd/dabri

%install
# Binary
install -D -m 0755 %{name} %{buildroot}%{_bindir}/%{name}

# Bundled whisper libraries (private prefix to avoid conflicts)
# Install only versioned .so files and create symlinks to avoid duplicating large binaries
install -d %{buildroot}%{_libdir}/%{name}

# Find and install the actual versioned libraries, create symlinks for unversioned names
for lib in lib/libwhisper.so lib/libggml.so lib/libggml-base.so lib/libggml-cpu.so lib/libggml-vulkan.so; do
    [ ! -f "$lib" ] && continue
    base=$(basename "$lib" .so)
    # Find the fully versioned file (e.g., libwhisper.so.1.8.3)
    versioned=$(ls -1 ${lib}.*.* 2>/dev/null | grep -E '\.so\.[0-9]+\.[0-9]+' | head -1)
    if [ -n "$versioned" ]; then
        # Install the versioned library
        install -m 0755 "$versioned" %{buildroot}%{_libdir}/%{name}/
        versioned_name=$(basename "$versioned")
        # Create symlinks: libfoo.so -> libfoo.so.X -> libfoo.so.X.Y.Z
        major=$(echo "$versioned_name" | sed -E 's/.*\.so\.([0-9]+).*/\1/')
        ln -sf "$versioned_name" %{buildroot}%{_libdir}/%{name}/${base}.so.${major}
        ln -sf "${base}.so.${major}" %{buildroot}%{_libdir}/%{name}/${base}.so
    else
        # No versioned file, just install as-is
        install -m 0755 "$lib" %{buildroot}%{_libdir}/%{name}/
    fi
done

# Desktop entry
install -D -m 0644 io.github.ashbuk.dabri.desktop \
    %{buildroot}%{_datadir}/applications/io.github.ashbuk.dabri.desktop

# AppStream metainfo
install -D -m 0644 io.github.ashbuk.dabri.appdata.xml \
    %{buildroot}%{_metainfodir}/io.github.ashbuk.dabri.appdata.xml

# Icons (multiple sizes for HiDPI support)
install -D -m 0644 icons/io.github.ashbuk.dabri.png \
    %{buildroot}%{_datadir}/icons/hicolor/128x128/apps/io.github.ashbuk.dabri.png
install -D -m 0644 icons/io.github.ashbuk.dabri.svg \
    %{buildroot}%{_datadir}/icons/hicolor/scalable/apps/io.github.ashbuk.dabri.svg

# Documentation
install -D -m 0644 README.md %{buildroot}%{_docdir}/%{name}/README.md
install -D -m 0644 docs/Desktop_Environment_Support.md \
    %{buildroot}%{_docdir}/%{name}/Desktop_Environment_Support.md

%check
# Sanity check - verify binary runs and shows help
export LD_LIBRARY_PATH=%{buildroot}%{_libdir}/%{name}
%{buildroot}%{_bindir}/%{name} --help 2>&1 | grep -q "dabri"

%post
# Update icon cache
/bin/touch --no-create %{_datadir}/icons/hicolor &>/dev/null || :

%postun
if [ $1 -eq 0 ] ; then
    /bin/touch --no-create %{_datadir}/icons/hicolor &>/dev/null
    /usr/bin/gtk-update-icon-cache %{_datadir}/icons/hicolor &>/dev/null || :
fi

%posttrans
/usr/bin/gtk-update-icon-cache %{_datadir}/icons/hicolor &>/dev/null || :

%files
%license LICENSE
%doc %{_docdir}/%{name}/
# Binary
%{_bindir}/%{name}
# Bundled libraries
%dir %{_libdir}/%{name}
%{_libdir}/%{name}/libwhisper.so*
%{_libdir}/%{name}/libggml*.so*
# Desktop integration
%{_datadir}/applications/io.github.ashbuk.dabri.desktop
%{_metainfodir}/io.github.ashbuk.dabri.appdata.xml
%{_datadir}/icons/hicolor/128x128/apps/io.github.ashbuk.dabri.png
%{_datadir}/icons/hicolor/scalable/apps/io.github.ashbuk.dabri.svg

%changelog
* Wed Jun 10 2026 Asher Buk <AshBuk@users.noreply.github.com> - 2.1.1-1
- Polish GNOME/Wayland portal global shortcuts and a single source of truth for config defaults

* Tue Jun 09 2026 Asher Buk <AshBuk@users.noreply.github.com> - 2.1.0-1
- Sandbox-friendly Flatpak release with a dedicated Wayland portal library and a tray status window

* Thu Jun 04 2026 Asher Buk <AshBuk@users.noreply.github.com> - 2.0.2-1
- Bug fixes and improvements

* Mon May 19 2026 Asher Buk <AshBuk@users.noreply.github.com> - 2.0.1-1
- Project renamed to Dabri (breaking change

* Thu May 15 2026 Asher Buk <AshBuk@users.noreply.github.com> - 1.8.2-1
- Bug fixes and improvements

* Sun Apr 19 2026 Asher Buk <AshBuk@users.noreply.github.com> - 1.8.1-1
- Bump whisper.cpp to v1.8.4, update Go dependencies

* Sat Mar 21 2026 Asher Buk <AshBuk@users.noreply.github.com> - 1.8.0-1
- Cobra CLI framework migration, add Whisper Large v3 Turbo (Q5_0) model

* Sun Mar 15 2026 Asher Buk <AshBuk@users.noreply.github.com> - 1.7.2-1
- Centralized version display, Go 1.25.3, dependency updates (systray, dbus, sys)

* Mon Mar 02 2026 Asher Buk <AshBuk@users.noreply.github.com> - 1.7.1-1
- Whisper model management: switch between base, small, medium, large-v3 via tray or CLI with hot-reload

* Mon Feb 23 2026 Asher Buk <AshBuk@users.noreply.github.com> - 1.6.3-1
- Fix potential crash on rapid hotkey restart, tray refactor, dead code cleanup

* Sat Feb 07 2026 Asher Buk <AshBuk@users.noreply.github.com> - 1.6.2-1
- CLI toggle command, XDG config compliance

* Mon Jan 19 2026 Asher Buk <AshBuk@users.noreply.github.com> - 1.6.1-1
- Enhanced status command, CI/CD improvements

* Wed Jan 14 2026 Asher Buk <AshBuk@users.noreply.github.com> - 1.6.0-1
- GPU acceleration: Vulkan backend support (auto-fallback to CPU)

* Wed Jan 07 2026 Asher Buk <AshBuk@users.noreply.github.com> - 1.5.2-1
- Hotkey and config improvements

* Tue Jan 06 2026 Asher Buk <AshBuk@users.noreply.github.com> - 1.5.1-1
- Auto-download whisper model to ~/.local/share/dabri/models/ on first run

* Tue Jan 06 2026 Asher Buk <AshBuk@users.noreply.github.com> - 1.5.0-1
- Add --version flag

* Sun Jan 04 2026 Asher Buk <AshBuk@users.noreply.github.com> - 1.4.2-1
- Initial RPM package
