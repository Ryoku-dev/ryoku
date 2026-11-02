Name:           game-devices-udev
Version:        1.1
Release:        1%{?dist}
Summary:        Udev rules for game controllers
License:        MIT
URL:            https://codeberg.org/fabiscafe/game-devices-udev
Source0:        %{url}/archive/%{version}.tar.gz#/%{name}-%{version}.tar.gz
BuildArch:      noarch
BuildRequires:  meson >= 1.0
BuildRequires:  systemd-rpm-macros
Requires:       systemd-udev
Provides:       nintendo-udev = %{version}-%{release}

%description
Udev rules granting userspace access and device metadata for game controllers
from Nintendo, Sony, Microsoft, Valve, 8BitDo, and other vendors.

%prep
%autosetup -n %{name}

%build
%meson
%meson_build

%install
%meson_install

%files
%license LICENSE
%{_udevrulesdir}/*.rules
