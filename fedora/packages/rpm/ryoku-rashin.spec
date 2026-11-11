Name:           ryoku-rashin
Version:        0.1
Release:        1%{?dist}
Summary:        Ryoku Rashin AI system
License:        GPL-3.0-or-later AND Apache-2.0
URL:            https://ryoku.dev
Source0:        ryoku-%{version}.tar.gz
%global debug_package %{nil}
BuildRequires:  bash
BuildRequires:  coreutils
BuildRequires:  findutils
BuildRequires:  gcc
BuildRequires:  golang >= 1.26.4
BuildRequires:  pkgconf-pkg-config
BuildRequires:  wayland-devel
BuildRequires:  libxkbcommon-devel
BuildRequires:  mesa-libEGL-devel
BuildRequires:  vulkan-headers
Requires:       ryoku-desktop = %{version}-%{release}
Requires:       libwayland-client
Requires:       libwayland-cursor
Requires:       libwayland-egl
Requires:       libxkbcommon
Requires:       mesa-libEGL
Requires:       vulkan-loader
Requires:       bubblewrap
Requires:       hicolor-icon-theme

%description
Rashin provides Ryoku's optional AI daemon, command line and Gio application.

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
systemctl --global disable ryoku-ai-usage.timer >/dev/null 2>&1 || :
rm -f /etc/systemd/user/default.target.wants/ryoku-rashin.service

%postun
if [ "$1" -eq 0 ]; then
  systemctl --global disable rashind.socket rashind.service ryoku-ai-usage.timer rashind-routines.timer >/dev/null 2>&1 || :
fi
