%global upstream_version 12.0.1.1105_p5

Name:           broadcom-bt-firmware
Version:        12.0.1.1105
Release:        1%{?dist}
Summary:        Firmware for Broadcom Bluetooth devices
License:        LicenseRef-Broadcom-Firmware AND MIT
URL:            https://github.com/winterheart/broadcom-bt-firmware
Source0:        %{url}/releases/download/v%{upstream_version}/%{name}-%{version}.deb
BuildArch:      noarch
BuildRequires:  binutils
BuildRequires:  tar
Conflicts:      broadcom-bt-firmware-git

%description
Patchram firmware for Broadcom USB Bluetooth adapters not covered by Fedora's
linux-firmware packages.

%prep
%setup -q -c -T
ar x %{SOURCE0}
mkdir payload
tar -xf data.tar.gz -C payload

%build

%install
install -d %{buildroot}%{_prefix}/lib/firmware/brcm
install -m 0644 payload/lib/firmware/brcm/*.hcd \
  %{buildroot}%{_prefix}/lib/firmware/brcm/

%files
%license payload%{_docdir}/%{name}-%{version}/LICENSE.broadcom_bcm20702
%license payload%{_docdir}/%{name}-%{version}/LICENSE.MIT.txt
%doc payload%{_docdir}/%{name}-%{version}/*.md
%{_prefix}/lib/firmware/brcm/*.hcd
