%global module_name hid-xpadneo
%global module_version v%{version}

Name:           xpadneo-dkms
Version:        0.10.5
Release:        1%{?dist}
Summary:        Advanced Linux driver for Xbox One wireless gamepads
License:        GPL-2.0-only AND GPL-3.0-or-later
URL:            https://github.com/atar-axis/xpadneo
Source0:        %{url}/archive/refs/tags/v%{version}.tar.gz#/xpadneo-%{version}.tar.gz
BuildArch:      noarch

BuildRequires:  make
Requires:       dkms
Requires:       make
Requires:       bluez
Requires:       bluez-tools
Requires:       kernel-devel
Requires:       kernel-headers

%description
xpadneo is a DKMS driver for Xbox One and Xbox Series controllers connected
over Bluetooth. Fedora DKMS signs the module with its local Machine Owner Key
when Secure Boot is enabled; that key must be enrolled before the kernel can
load the module.

%prep
%autosetup -n xpadneo-%{version}

%build
make VERSION=%{module_version} build

%install
make PREFIX=%{buildroot} ETC_PREFIX=/usr/lib VERSION=%{module_version} install

install -dm0755 %{buildroot}%{_usrsrc}/%{module_name}-%{module_version}
install -m0644 hid-xpadneo/Makefile hid-xpadneo/dkms.conf \
    %{buildroot}%{_usrsrc}/%{module_name}-%{module_version}/
install -m0755 hid-xpadneo/dkms.post_install hid-xpadneo/dkms.post_remove \
    %{buildroot}%{_usrsrc}/%{module_name}-%{module_version}/
cp -a hid-xpadneo/src %{buildroot}%{_usrsrc}/%{module_name}-%{module_version}/

%post
if [ "$1" -gt 1 ]; then
    dkms status -m %{module_name} 2>/dev/null |
    while IFS= read -r registration; do
        case "$registration" in
            %{module_name}/*)
                old_version=${registration#*/}
                old_version=${old_version%%%%,*}
                old_version=${old_version%%%%:*}
                old_version=${old_version%%%% *}
                if [ -n "$old_version" ] && [ "$old_version" != "%{module_version}" ]; then
                    dkms remove -m %{module_name} -v "$old_version" --all || :
                fi
                ;;
        esac
    done
fi
dkms add -m %{module_name} -v %{module_version} || :
dkms build -m %{module_name} -v %{module_version} || :
dkms install -m %{module_name} -v %{module_version} || :
udevadm control --reload >/dev/null 2>&1 || :
udevadm trigger --subsystem-match=hid --action=add >/dev/null 2>&1 || :

%preun
if [ "$1" -eq 0 ]; then
    dkms remove -m %{module_name} -v %{module_version} --all || :
fi

%postun
udevadm control --reload >/dev/null 2>&1 || :

%files
%license LICENSE.md LICENSES/*.txt
%doc %{_datadir}/doc/xpadneo/
%{_usrsrc}/%{module_name}-%{module_version}/
%{_prefix}/lib/modprobe.d/xpadneo.conf
%{_prefix}/lib/udev/rules.d/60-xpadneo.rules
%{_prefix}/lib/udev/rules.d/70-xpadneo-disable-hidraw.rules
