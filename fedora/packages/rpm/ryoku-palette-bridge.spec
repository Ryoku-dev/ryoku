Name:           ryoku-palette-bridge
Version:        0.1
Release:        1%{?dist}
Summary:        Ryoku wallpaper palette event bridge
License:        MIT
URL:            https://ryoku.dev
Source0:        ryoku-%{version}.tar.gz
%global debug_package %{nil}
BuildRequires:  bash
BuildRequires:  coreutils
BuildRequires:  findutils
BuildRequires:  golang >= 1.26.4
Requires:       curl
Requires:       jq
Requires:       systemd

%description
ryoku-palette-bridge, built from the shared Ryoku source and package payload recipe.

%prep
%setup -q -n ryoku-%{version}

%build
export RYOKU_PKGVER=%{version}
bash fedora/packages/rpm/stage-package.sh %{name} "$PWD/stage" %{_libdir}

%install
cp -a stage/. %{buildroot}/
find %{buildroot} -type f -o -type l | sed 's|^%{buildroot}||' > rpm-files

%files -f rpm-files
%license %{_datadir}/ryoku/palette-bridge/LICENSE
