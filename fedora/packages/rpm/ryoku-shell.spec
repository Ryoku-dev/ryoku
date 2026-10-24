Name:           ryoku-shell
Version:        0.1
Release:        1%{?dist}
Summary:        ryoku-shell desktop component
License:        GPL-3.0-or-later
URL:            https://ryoku.dev
Source0:        ryoku-%{version}.tar.gz
%global debug_package %{nil}
BuildRequires:  bash
BuildRequires:  coreutils
BuildRequires:  findutils
BuildRequires:  golang >= 1.26.4
BuildRequires:  gcc
BuildRequires:  pkgconf-pkg-config
BuildRequires:  wayland-devel
BuildRequires:  wayland-protocols-devel
BuildRequires:  ffmpeg-free-devel
Requires:       quickshell >= 0.3.0
Requires:       ffmpeg-free
Requires:       libwayland-client
Requires:       libwayland-cursor
Requires:       libwayland-egl
Requires:       libwayland-server
Requires:       jq
Requires:       python3
Requires:       uv
Requires:       socat
Requires:       gum
Requires:       qrencode
Requires:       upower
Requires:       brightnessctl
Requires:       ddcutil
Requires:       wtype
Requires:       wl-clipboard
Requires:       zenity
Requires:       ppd-service
Requires:       NetworkManager
Requires:       NetworkManager-wifi
Requires:       curl
Requires:       libnotify
Requires:       inotify-tools
Requires:       easyeffects
Requires:       lsp-plugins-lv2
Requires:       lv2-calf-plugins

%description
ryoku-shell, built from the shared Ryoku source and package payload recipe.

%prep
%setup -q -n ryoku-%{version}

%build
export RYOKU_PKGVER=%{version}
bash fedora/packages/rpm/stage-package.sh %{name} "$PWD/stage" %{_libdir}

%install
cp -a stage/. %{buildroot}/
find %{buildroot} -type f -o -type l | sed 's|^%{buildroot}||' > rpm-files

%files -f rpm-files
