Name:           ryoku-desktop-hyprland
Version:        0.1
Epoch:          1
Release:        1%{?dist}
Summary:        ryoku-desktop-hyprland desktop component
License:        GPL-3.0-or-later
URL:            https://ryoku.dev
Source0:        ryoku-%{version}.tar.gz
%global debug_package %{nil}
BuildRequires:  bash
BuildRequires:  coreutils
BuildRequires:  findutils
BuildRequires:  golang >= 1.26.4
Requires:       ryoku-desktop = %{epoch}:%{version}-%{release}
Requires:       hyprland
Recommends:     ryoku-hypr-plugins = %{epoch}:%{version}-%{release}
Recommends:     ryoku-keysounds = %{epoch}:%{version}-%{release}
Recommends:     hypr-dynamic-cursors = %{epoch}:%{version}-%{release}
Recommends:     hyprglass = %{epoch}:%{version}-%{release}
Recommends:     hyprgrass = %{epoch}:%{version}-%{release}
Recommends:     imgborders = %{epoch}:%{version}-%{release}
Requires:       hyprland-preview-share-picker = %{epoch}:%{version}-%{release}
Requires:       hyprpolkitagent
Requires:       xdg-desktop-portal-hyprland
Requires:       hyprsunset
Provides:       ryoku-desktop-compositor

%description
ryoku-desktop-hyprland, built from the shared Ryoku source and package payload recipe.

%prep
%setup -q -n ryoku-%{version}

%build
export RYOKU_PKGVER=%{version}
bash fedora/packages/rpm/stage-package.sh %{name} "$PWD/stage" %{_libdir}

%install
cp -a stage/. %{buildroot}/
find %{buildroot} -type f -o -type l | sed 's|^%{buildroot}||' > rpm-files

%files -f rpm-files
