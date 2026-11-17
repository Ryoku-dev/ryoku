%global debug_package %{nil}
%global hyprland_evr %(rpm -q --qf '%%{EVR}' hyprland)
Name:           hypr-dynamic-cursors
Version:        0.1
Epoch:          1
Release:        1%{?dist}
Summary:        Dynamic cursor effects for Hyprland
License:        GPL-3.0-or-later
URL:            https://github.com/VirtCode/hypr-dynamic-cursors
Source0:        ryoku-%{version}.tar.gz
BuildRequires:  bash
BuildRequires:  coreutils
BuildRequires:  findutils
BuildRequires:  gcc-c++
BuildRequires:  make
BuildRequires:  pkgconf-pkg-config
BuildRequires:  hyprcursor-devel
BuildRequires:  libdrm-devel
BuildRequires:  pixman-devel
BuildRequires:  hyprland-devel
Requires:       hyprland%{?_isa} = %{hyprland_evr}

%description
Realistic cursor rotation, tilt, stretch and shake-to-find effects, compiled
for the exact Hyprland ABI installed on the build host.

%prep
%setup -q -n ryoku-%{version}

%build
export RYOKU_PKGVER=%{version}
bash fedora/packages/rpm/stage-package.sh %{name} "$PWD/stage" %{_libdir}

%install
cp -a stage/. %{buildroot}/
find %{buildroot} -type f -o -type l | sed 's|^%{buildroot}||' > rpm-files

%files -f rpm-files
