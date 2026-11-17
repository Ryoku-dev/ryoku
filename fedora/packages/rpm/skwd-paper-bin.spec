%global ryoku_vendor cargo
%global skwd_commit ab6d71d6abaeb0f519003c11ff968c9dc296ed11
%global ffmpeg_version 9.0.1

Name:           skwd-paper-bin
Version:        1.0.0_beta.29
Release:        1%{?dist}
Summary:        Wallpaper Engine renderer backend for Ryogami
License:        GPL-3.0-or-later
URL:            https://github.com/liixini/skwd-paper
Source0:        %{url}/archive/%{skwd_commit}.tar.gz#/skwd-paper-%{version}.tar.gz
Source1:        https://ffmpeg.org/releases/ffmpeg-%{ffmpeg_version}.tar.xz
Source100:      %{name}-%{version}-vendor.tar.gz
ExclusiveArch:  x86_64

BuildRequires:  alsa-lib-devel
BuildRequires:  cargo
BuildRequires:  clang
BuildRequires:  cmake
BuildRequires:  gcc
BuildRequires:  gcc-c++
BuildRequires:  freetype-devel
BuildRequires:  glslc
BuildRequires:  libdav1d-devel
BuildRequires:  libdrm-devel
BuildRequires:  libva-devel
BuildRequires:  libshaderc-devel
BuildRequires:  libyuv-devel
BuildRequires:  libxkbcommon-devel
BuildRequires:  make
BuildRequires:  nasm
BuildRequires:  ninja-build
BuildRequires:  patchelf
BuildRequires:  pkgconf-pkg-config
BuildRequires:  pulseaudio-libs-devel
BuildRequires:  python3
BuildRequires:  rust
BuildRequires:  vulkan-headers
BuildRequires:  vulkan-loader-devel
BuildRequires:  wayland-devel
BuildRequires:  xz
BuildRequires:  zlib-ng-compat-devel
Requires:       alsa-lib
Requires:       dav1d
Requires:       freetype
Requires:       libgcc
Requires:       libstdc++
Requires:       libdrm
Requires:       libglvnd
Requires:       pulseaudio-libs
Requires:       pulseaudio-utils
Requires:       libva
Requires:       libxkbcommon
Requires:       libyuv
Requires:       glslc
Requires:       vulkan-loader
Requires:       libwayland-client
Requires:       libwayland-cursor
Requires:       libwayland-egl
Requires:       libwayland-server
Requires:       zlib-ng-compat
Provides:       skwd-paper = %{version}-%{release}
Conflicts:      skwd-paper

%description
skwd-paper supplies the still, video, and Wallpaper Engine renderers used by
Ryogami. It is built against Fedora libraries and carries its matching private
FFmpeg runtime.

%prep
%autosetup -n skwd-paper-%{skwd_commit}
tar -xf %{SOURCE100}
mkdir -p .cargo vendor-ffmpeg
cp -p %{SOURCE1} vendor-ffmpeg/
cat > .cargo/config.toml <<'EOF'
[source.crates-io]
replace-with = "vendored-sources"

[source.vendored-sources]
directory = "vendor"
EOF

%build
export SKWD_FFMPEG_TARBALL="$PWD/vendor-ffmpeg/ffmpeg-%{ffmpeg_version}.tar.xz"
export SKWD_FFMPEG_JOBS=8
ffmpeg_cflags=${CFLAGS//-flto=auto/}
ffmpeg_cflags=${ffmpeg_cflags//-ffat-lto-objects/}
CFLAGS="$ffmpeg_cflags" ./packaging/build-ffmpeg-runtime.sh "$PWD/.rpm-ffmpeg"
export FFMPEG_DIR="$PWD/.rpm-ffmpeg"
export PKG_CONFIG_PATH="$FFMPEG_DIR/lib/pkgconfig"
cargo build --offline --locked --release --workspace

%install
SKWD_PAPER_BIN_DIR="$PWD/target/release" \
  ./scripts/package-stage.sh "$PWD/.rpm-stage"
cp -a .rpm-stage/. %{buildroot}/
install -d %{buildroot}/usr/lib/skwd-paper
cp -a .rpm-ffmpeg/lib/*.so* %{buildroot}/usr/lib/skwd-paper/
install -Dpm0644 .rpm-ffmpeg/share/licenses/ffmpeg/COPYING.LGPLv2.1 \
  %{buildroot}%{_licensedir}/%{name}/ffmpeg-COPYING.LGPLv2.1
install -Dpm0644 .rpm-ffmpeg/share/licenses/ffmpeg/LICENSE.md \
  %{buildroot}%{_licensedir}/%{name}/ffmpeg-LICENSE.md

%files
%license %{_licensedir}/skwd-paper/LICENSE
%license %{_licensedir}/skwd-paper/third-party/*
%license %{_licensedir}/%{name}/ffmpeg-*
%{_bindir}/skwd-paper-v2
%{_bindir}/skwd-wall-still
%{_bindir}/skwd-wall-vk
/usr/lib/skwd-paper/
