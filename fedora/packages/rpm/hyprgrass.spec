%global debug_package %{nil}
%global hyprland_evr %(rpm -q --qf '%%{EVR}' hyprland)
Name:           hyprgrass
Version:        0.1
Epoch:          1
Release:        1%{?dist}
Summary:        Touch gesture plugin for Hyprland
License:        Apache-2.0
URL:            https://github.com/horriblename/hyprgrass
Source0:        ryoku-%{version}.tar.gz
BuildRequires:  bash
BuildRequires:  cairo-devel
BuildRequires:  coreutils
BuildRequires:  findutils
BuildRequires:  gcc-c++
BuildRequires:  glm-devel
BuildRequires:  libdrm-devel
BuildRequires:  libinput-devel
BuildRequires:  libxkbcommon-devel
BuildRequires:  lua-devel
BuildRequires:  meson
BuildRequires:  ninja-build
BuildRequires:  pango-devel
BuildRequires:  pixman-devel
BuildRequires:  pkgconf-pkg-config
BuildRequires:  hyprland-devel
BuildRequires:  wayland-devel
Requires:       hyprland%{?_isa} = %{hyprland_evr}

%description
Touchscreen and touchpad gesture support for Hyprland windows and workspaces,
compiled against the exact compositor ABI.

%prep
%setup -q -n ryoku-%{version}

%build
export RYOKU_PKGVER=%{version}
bash fedora/packages/rpm/stage-package.sh %{name} "$PWD/stage" %{_libdir}

%install
cp -a stage/. %{buildroot}/
find %{buildroot} -type f -o -type l | sed 's|^%{buildroot}||' > rpm-files

%files -f rpm-files
