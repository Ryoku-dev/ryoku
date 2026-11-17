%global debug_package %{nil}
%global hyprland_evr %(rpm -q --qf '%%{EVR}' hyprland)
Name:           imgborders
Version:        0.1
Epoch:          1
Release:        1%{?dist}
Summary:        Image border plugin for Hyprland
License:        GPL-3.0-or-later
URL:            https://codeberg.org/zacoons/imgborders
Source0:        ryoku-%{version}.tar.gz
BuildRequires:  bash
BuildRequires:  cairo-devel
BuildRequires:  cmake
BuildRequires:  coreutils
BuildRequires:  findutils
BuildRequires:  gcc-c++
BuildRequires:  libdrm-devel
BuildRequires:  make
BuildRequires:  pango-devel
BuildRequires:  pixman-devel
BuildRequires:  pkgconf-pkg-config
BuildRequires:  hyprland-devel
Requires:       hyprland%{?_isa} = %{hyprland_evr}

%description
Tiled image borders for Hyprland windows, compiled against the exact compositor
ABI.

%prep
%setup -q -n ryoku-%{version}

%build
export RYOKU_PKGVER=%{version}
bash fedora/packages/rpm/stage-package.sh %{name} "$PWD/stage" %{_libdir}

%install
cp -a stage/. %{buildroot}/
find %{buildroot} -type f -o -type l | sed 's|^%{buildroot}||' > rpm-files

%files -f rpm-files
