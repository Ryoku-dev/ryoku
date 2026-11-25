# SPDX-License-Identifier: GPL-3.0-only
# Standalone Hyakki, built from its corresponding-source archive made with
# release/source-dist.sh. The package never enables a unit.
Name:           hyakki
Version:        0.1.0
Release:        1%{?dist}
Summary:        Hyakki local-first AI desktop and command line
License:        GPL-3.0-only AND Apache-2.0 AND OFL-1.1 AND MIT AND BSD-3-Clause AND LicenseRef-SCOWL AND Zlib
URL:            https://ryoku.dev
Source0:        hyakki-%{version}.tar.gz
ExclusiveArch:  x86_64
Obsoletes:      ryoku-rashin
Conflicts:      ryoku-rashin
%global debug_package %{nil}

BuildRequires:  gcc
BuildRequires:  golang >= 1.26
BuildRequires:  pkgconf-pkg-config
BuildRequires:  python3
BuildRequires:  wayland-devel
BuildRequires:  libxkbcommon-devel
BuildRequires:  libxkbcommon-x11-devel
BuildRequires:  mesa-libEGL-devel
BuildRequires:  vulkan-headers
BuildRequires:  libX11-devel
BuildRequires:  libXcursor-devel
BuildRequires:  libXfixes-devel

Requires:       libX11
Requires:       libX11-xcb
Requires:       libXcursor
Requires:       libXfixes
Requires:       libxkbcommon
Requires:       libxkbcommon-x11
Requires:       libwayland-client
Requires:       libwayland-cursor
Requires:       libwayland-egl
Requires:       mesa-libEGL
Requires:       mesa-libGL
Requires:       mesa-dri-drivers
Requires:       libglvnd
Requires:       vulkan-loader
Requires:       hicolor-icon-theme
Requires:       python3
Requires:       git
Requires:       xdg-utils
Requires:       bubblewrap

# Generated from release/optional.json; release/check.py keeps them equal.
# Hyakki runs none of these until the feature that names it is turned on.
# chromium: the browser Hands drives in its own profile
Suggests:       chromium
# weston, niri: the separate desktop Hands runs apps on; niri also maximizes
Suggests:       weston
Suggests:       niri
# grim: screenshots without Ryoku's capture tool, and Hands looking at its desktop
Suggests:       grim
# wtype: types dictation, rewrites and Hands keys into the focused window
Suggests:       wtype
# wl-clipboard: reads the highlighted text for a rewrite in place
Suggests:       wl-clipboard
# tesseract: reads words on screen for guides and select-and-ask
Suggests:       tesseract
Suggests:       tesseract-langpack-eng
# espeak-ng: Oni's system voice
Suggests:       espeak-ng
# pipewire-utils: the microphone and Oni's voice (pw-cat)
Suggests:       pipewire-utils
# pulseaudio-utils: the microphone list, volume commands and the PulseAudio fallback
Suggests:       pulseaudio-utils
# uv: sets up Hindsight, the optional memory engine, when asked
Suggests:       uv
# llama-cpp: runs downloaded models; without it Hyakki can fetch llama.cpp when asked
Suggests:       llama-cpp

%description
Hyakki provides a local-first AI daemon, command line and Gio application.
Nothing starts at install: clients start the daemon on demand, and
"hyakki on" arms the optional user units.

%prep
%setup -q -n hyakki-%{version}

%build
cd ryoku/hyakki
HYAKKI_VERSION=%{version} HYAKKI_BUILD_DIR="$PWD/build/native" ./release/build.sh

%install
cd ryoku/hyakki
HYAKKI_BUILD_DIR="$PWD/build/native" DESTDIR=%{buildroot} ./release/install.sh

%check
cd ryoku/hyakki
./release/check.sh %{buildroot}

%files
%license /usr/share/licenses/hyakki
%doc /usr/share/doc/hyakki
/usr/bin/hyakki
/usr/bin/oni
/usr/bin/hyakki-app
/usr/bin/oni-surface
/usr/bin/hyakki-usage
/usr/bin/claude-usage
/usr/bin/codex-usage
/usr/bin/opencode-usage
/usr/lib/systemd/user/oni.service
/usr/lib/systemd/user/oni.socket
/usr/lib/systemd/user/oni-routines.service
/usr/lib/systemd/user/oni-routines.timer
/usr/lib/systemd/user/hyakki-usage.service
/usr/lib/systemd/user/hyakki-usage.timer
/usr/share/hyakki
/usr/share/applications/dev.ryoku.Hyakki.desktop
/usr/share/metainfo/dev.ryoku.Hyakki.metainfo.xml
/usr/share/icons/hicolor/scalable/apps/dev.ryoku.Hyakki.svg

%preun
if [ "$1" -eq 0 ]; then
  echo 'Run "hyakki off" as each user before removing Hyakki, so no user unit or runit service stays linked.'
fi

