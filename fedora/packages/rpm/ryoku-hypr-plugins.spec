%global debug_package %{nil}
%global hyprland_evr %(rpm -q --qf '%%{EVR}' hyprland)
Name:           ryoku-hypr-plugins
Version:        0.1
Epoch:          1
Release:        1%{?dist}
Summary:        Official Hyprland plugins selected by Ryoku
License:        BSD-3-Clause
URL:            https://github.com/hyprwm/hyprland-plugins
Source0:        ryoku-%{version}.tar.gz
BuildRequires:  bash
BuildRequires:  coreutils
BuildRequires:  findutils
BuildRequires:  gcc-c++
BuildRequires:  make
BuildRequires:  pkgconf-pkg-config
BuildRequires:  hyprland-devel
BuildRequires:  cairo-devel
BuildRequires:  libdrm-devel
BuildRequires:  libinput-devel
BuildRequires:  libxkbcommon-devel
BuildRequires:  pango-devel
BuildRequires:  pixman-devel
BuildRequires:  systemd-devel
BuildRequires:  wayland-devel
Requires:       hyprland%{?_isa} = %{hyprland_evr}

%description
Hyprbars, hyprfocus and borders-plus-plus built for Ryoku against the exact
Hyprland ABI shipped with this package build.

%prep
%setup -q -n ryoku-%{version}

%build
export RYOKU_PKGVER=%{version}
bash fedora/packages/rpm/stage-package.sh %{name} "$PWD/stage" %{_libdir}

%install
cp -a stage/. %{buildroot}/
find %{buildroot} -type f -o -type l | sed 's|^%{buildroot}||' > rpm-files

%files -f rpm-files
