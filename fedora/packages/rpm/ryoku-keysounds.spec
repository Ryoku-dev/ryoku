%global debug_package %{nil}
%global hyprland_evr %(rpm -q --qf '%%{EVR}' hyprland)
Name:           ryoku-keysounds
Version:        0.1
Epoch:          1
Release:        1%{?dist}
Summary:        Keyboard sound plugin for Hyprland
License:        GPL-3.0-or-later
URL:            https://ryoku.dev
Source0:        ryoku-%{version}.tar.gz
BuildRequires:  bash
BuildRequires:  coreutils
BuildRequires:  findutils
BuildRequires:  gcc-c++
BuildRequires:  libcanberra-devel
BuildRequires:  libdrm-devel
BuildRequires:  make
BuildRequires:  pkgconf-pkg-config
BuildRequires:  pixman-devel
BuildRequires:  hyprland-devel
Requires:       hyprland%{?_isa} = %{hyprland_evr}
Requires:       libcanberra
Requires:       ryoku-keysounds-profiles = %{epoch}:%{version}-%{release}

%description
Ryoku's Hyprland key sound plugin, built against the exact compositor ABI. The
sample profiles and the importer come from ryoku-keysounds-profiles, shared with
niri's key-sounds engine.

%prep
%setup -q -n ryoku-%{version}

%build
export RYOKU_PKGVER=%{version}
bash fedora/packages/rpm/stage-package.sh %{name} "$PWD/stage" %{_libdir}

%install
cp -a stage/. %{buildroot}/
find %{buildroot} -type f -o -type l | sed 's|^%{buildroot}||' > rpm-files

%files -f rpm-files
