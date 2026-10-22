Name:           ryoku-caelestia
Version:        0.1
Release:        1%{?dist}
Summary:        ryoku-caelestia desktop component
License:        GPL-3.0-or-later
URL:            https://ryoku.dev
Source0:        ryoku-%{version}.tar.gz
%global debug_package %{nil}
BuildRequires:  bash
BuildRequires:  coreutils
BuildRequires:  findutils
BuildRequires:  cmake
BuildRequires:  ninja-build
BuildRequires:  gcc-c++
BuildRequires:  qt6-qtbase-devel
BuildRequires:  qt6-qtdeclarative-devel
BuildRequires:  libqalculate-devel
BuildRequires:  pipewire-devel
BuildRequires:  aubio-devel
BuildRequires:  lm_sensors-devel
BuildRequires:  qt6-qtshadertools-devel
Requires:       qt6-qtbase
Requires:       qt6-qtdeclarative
Requires:       libqalculate
Requires:       pipewire
Requires:       aubio
Requires:       lm_sensors-libs
Requires:       cava

%description
ryoku-caelestia, built from the shared Ryoku source and package payload recipe.

%prep
%setup -q -n ryoku-%{version}

%build
export RYOKU_PKGVER=%{version}
bash fedora/packages/rpm/stage-package.sh %{name} "$PWD/stage" %{_libdir}

%install
cp -a stage/. %{buildroot}/
find %{buildroot} -type f -o -type l | sed 's|^%{buildroot}||' > rpm-files

%files -f rpm-files
