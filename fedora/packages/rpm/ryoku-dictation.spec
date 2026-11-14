Name:           ryoku-dictation
Version:        1.0.0
Release:        1%{?dist}
Summary:        Ryoku voice typing: local speech-to-text with Parakeet and Whisper
License:        GPL-3.0-or-later
URL:            https://ryoku.dev
Source0:        ryoku-%{version}.tar.gz
%global debug_package %{nil}
BuildRequires:  bash
BuildRequires:  cmake
BuildRequires:  ninja-build
BuildRequires:  gcc-c++
BuildRequires:  pkgconf-pkg-config
BuildRequires:  pipewire-devel
BuildRequires:  wayland-devel
BuildRequires:  libxkbcommon-devel
BuildRequires:  vulkan-loader-devel
BuildRequires:  vulkan-headers
BuildRequires:  glslc
Requires:       glibc
Requires:       libstdc++
Requires:       pipewire
Requires:       libwayland-client
Requires:       libxkbcommon
Requires:       wl-clipboard
Requires:       vulkan-loader

%description
Ryoku's own voice typing engine: a small daemon that records through PipeWire,
transcribes locally with transcribe.cpp and types the words into the focused
window. It builds the pinned engine from source; speech models are downloaded
separately in Settings.

%prep
%setup -q -n ryoku-%{version}

%build
export RYOKU_SRC="$PWD"
export RYOKU_BUILD_DIR="$PWD/.ryoku-build"
mkdir -p "$RYOKU_BUILD_DIR"
bash release/packages/ryoku-dictation/payload.sh build

%install
export RYOKU_SRC="$PWD"
export RYOKU_BUILD_DIR="$PWD/.ryoku-build"
bash release/packages/ryoku-dictation/payload.sh install %{buildroot} systemd

%files
%license %{_licensedir}/ryoku-dictation/NOTICE.md
%doc %{_docdir}/ryoku-dictation/README.md
%{_bindir}/ryoku-dictate
%{_userunitdir}/ryoku-dictate.service
/usr/lib/ryoku/dictation/
