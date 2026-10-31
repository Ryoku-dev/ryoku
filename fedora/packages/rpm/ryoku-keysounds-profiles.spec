Name:           ryoku-keysounds-profiles
Version:        0.1
Release:        1%{?dist}
Summary:        Key-sound sample profiles and the Mechvibes importer
License:        GPL-3.0-or-later AND MIT
URL:            https://ryoku.dev
Source0:        ryoku-%{version}.tar.gz
%global debug_package %{nil}
BuildRequires:  bash
BuildRequires:  coreutils
BuildRequires:  ffmpeg-free
BuildRequires:  findutils
BuildRequires:  python3
Requires:       /usr/bin/ffmpeg

%description
The real switch recordings both key-sound engines use, cut from the MIT-licensed
Mechvibes packs at build time, plus ryoku-keysounds-import.

%prep
%setup -q -n ryoku-%{version}

%build
export RYOKU_PKGVER=%{version}
bash fedora/packages/rpm/stage-package.sh %{name} "$PWD/stage" %{_libdir}

%install
cp -a stage/. %{buildroot}/
find %{buildroot} -type f -o -type l | sed 's|^%{buildroot}||' > rpm-files

%files -f rpm-files
