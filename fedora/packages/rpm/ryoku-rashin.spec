Name:           ryoku-rashin
Version:        0.1
Epoch:          1
Release:        1%{?dist}
Summary:        ryoku-rashin desktop component
License:        GPL-3.0-or-later
URL:            https://ryoku.dev
Source0:        ryoku-%{version}.tar.gz
%global debug_package %{nil}
BuildRequires:  bash
BuildRequires:  coreutils
BuildRequires:  findutils
BuildRequires:  golang >= 1.26.4
BuildRequires:  gtk3-devel
BuildRequires:  webkit2gtk4.1-devel
BuildRequires:  cmake
BuildRequires:  ninja-build
BuildRequires:  gcc-c++
Requires:       ryoku-desktop = %{epoch}:%{version}-%{release}
Requires(post): ryoku = %{epoch}:%{version}-%{release}
Requires:       uv
Requires:       nodejs24
Requires:       gcc
Requires:       prowl
Requires:       gtk3
Requires:       webkit2gtk4.1
Requires:       python3
Suggests:       sqlite

%description
ryoku-rashin, built from the shared Ryoku source and package payload recipe.

%prep
%setup -q -n ryoku-%{version}

%build
export RYOKU_PKGVER=%{version}
bash fedora/packages/rpm/stage-package.sh %{name} "$PWD/stage" %{_libdir}

%install
cp -a stage/. %{buildroot}/
find %{buildroot} -type f -o -type l | sed 's|^%{buildroot}||' > rpm-files

%files -f rpm-files

%post
if [ -x /usr/bin/ryoku-host ]; then
  RYOKU_HOST_INIT=systemd /usr/bin/ryoku-host svc reconcile --system >/dev/null 2>&1 || :
fi

%postun
if [ "$1" -eq 0 ]; then
  systemctl --global disable ryoku-rashin.service ryoku-ai-usage.timer >/dev/null 2>&1 || :
fi
