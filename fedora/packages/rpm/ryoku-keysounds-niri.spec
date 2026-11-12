Name:           ryoku-keysounds-niri
Version:        0.1
Epoch:          1
Release:        1%{?dist}
Summary:        niri key-press sounds (evdev + libcanberra engine)
License:        GPL-3.0-or-later
URL:            https://ryoku.dev
Source0:        ryoku-%{version}.tar.gz
%global debug_package %{nil}
BuildRequires:  bash
BuildRequires:  gcc
BuildRequires:  coreutils
BuildRequires:  findutils
BuildRequires:  libcanberra-devel
BuildRequires:  rust
Requires:       libcanberra
Requires:       ryoku-keysounds-profiles = %{epoch}:%{version}-%{release}

%description
niri has no plugin system, so its key-press sounds come from this small daemon
instead of the Hyprland plugin: it reads the keyboard from evdev and plays each
press through libcanberra.

%prep
%setup -q -n ryoku-%{version}

%build
export RYOKU_PKGVER=%{version}
bash fedora/packages/rpm/stage-package.sh %{name} "$PWD/stage" %{_libdir}

%install
cp -a stage/. %{buildroot}/
find %{buildroot} -type f -o -type l | sed 's|^%{buildroot}||' > rpm-files

%files -f rpm-files
