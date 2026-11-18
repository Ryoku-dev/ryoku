%global debug_package %{nil}
%global hyprland_evr %(rpm -q --qf '%%{EVR}' hyprland)
Name:           ryoku-hyprdrive
Version:        0.1
Epoch:          1
Release:        1%{?dist}
Summary:        Overview, Mosaic layouts and window motion plugin for Hyprland
License:        BSD-3-Clause AND OFL-1.1 AND GPL-3.0-or-later
URL:            https://ryoku.dev
Source0:        ryoku-%{version}.tar.gz
BuildRequires:  bash
BuildRequires:  coreutils
BuildRequires:  findutils
BuildRequires:  gcc-c++
BuildRequires:  golang
BuildRequires:  make
BuildRequires:  pkgconf-pkg-config
BuildRequires:  hyprland-devel
BuildRequires:  pixman-devel
BuildRequires:  libdrm-devel
BuildRequires:  pango-devel
BuildRequires:  cairo-devel
BuildRequires:  fontconfig-devel
BuildRequires:  libinput-devel
BuildRequires:  systemd-devel
BuildRequires:  wayland-devel
BuildRequires:  libxkbcommon-devel
BuildRequires:  glib2-devel
BuildRequires:  lua-devel
BuildRequires:  libxcb-devel
Requires:       hyprland%{?_isa} = %{hyprland_evr}

%description
Hyprdrive for Ryoku: a mission-control overview of every window, Mosaic
layouts and window motion, built against the exact compositor ABI. Off until
it is turned on in Ryoku Settings, Windows, Hyprdrive.

%prep
%setup -q -n ryoku-%{version}

%build
export RYOKU_PKGVER=%{version}
bash fedora/packages/rpm/stage-package.sh %{name} "$PWD/stage" %{_libdir}

%install
cp -a stage/. %{buildroot}/
find %{buildroot} -type f -o -type l | sed 's|^%{buildroot}||' > rpm-files

%files -f rpm-files
