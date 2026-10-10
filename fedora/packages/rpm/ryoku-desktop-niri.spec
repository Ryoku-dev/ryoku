Name:           ryoku-desktop-niri
Version:        0.1
Release:        1%{?dist}
Summary:        ryoku-desktop-niri desktop component
License:        GPL-3.0-or-later
URL:            https://ryoku.dev
Source0:        ryoku-%{version}.tar.gz
%global debug_package %{nil}
BuildRequires:  bash
BuildRequires:  coreutils
BuildRequires:  findutils
BuildRequires:  golang >= 1.26.4
Requires:       ryoku-desktop = %{version}-%{release}
Requires:       niri
Requires:       xwayland-satellite
Requires:       xdg-desktop-portal-gnome
Requires:       wlsunset
Provides:       ryoku-desktop-compositor

%description
ryoku-desktop-niri, built from the shared Ryoku source and package payload recipe.

%prep
%setup -q -n ryoku-%{version}

%build
export RYOKU_PKGVER=%{version}
bash fedora/packages/rpm/stage-package.sh %{name} "$PWD/stage" %{_libdir}

%install
cp -a stage/. %{buildroot}/
find %{buildroot} -type f -o -type l | sed 's|^%{buildroot}||' > rpm-files

%files -f rpm-files
