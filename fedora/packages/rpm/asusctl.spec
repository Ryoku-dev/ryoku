%global ryoku_vendor cargo

Name:           asusctl
Version:        6.5.0
Release:        1%{?dist}
Summary:        ASUS laptop control daemon and Aura lighting CLI
License:        MPL-2.0
URL:            https://github.com/OpenGamingCollective/asusctl
Source0:        %{url}/archive/refs/tags/%{version}.tar.gz#/asusctl-%{version}.tar.gz
Source100:      %{name}-%{version}-vendor.tar.gz
ExclusiveArch:  x86_64

BuildRequires:  cargo
BuildRequires:  rust
BuildRequires:  clang
BuildRequires:  llvm-devel
BuildRequires:  gcc
BuildRequires:  make
BuildRequires:  pkgconfig(libudev)
BuildRequires:  libusb1-devel
BuildRequires:  systemd-rpm-macros
Requires:       systemd-udev
Requires:       libusb1
Conflicts:      tlp

%description
asusctl controls supported ASUS laptop platform features. The package includes
asusd, its systemd units, udev activation rules, and the shutdown helper.

%prep
%autosetup -n %{name}-%{version}
tar -xf %{SOURCE100}
mkdir -p .cargo
cat > .cargo/config.toml <<'EOF'
[source.crates-io]
replace-with = "vendored-sources"

[source.vendored-sources]
directory = "vendor"

[source."git+https://github.com/slint-ui/slint.git"]
git = "https://github.com/slint-ui/slint.git"
replace-with = "vendored-sources"
EOF

%build
cargo build --release --offline --locked -p asusctl -p asusd -p asus-shutdown

%install
make DESTDIR=%{buildroot} install-asusctl install-asusd install-asus-shutdown install-data-asusd

%post
%systemd_post asusd.service asus-shutdown.service
udevadm control --reload >/dev/null 2>&1 || :
udevadm trigger --subsystem-match=platform --action=add >/dev/null 2>&1 || :

%preun
%systemd_preun asusd.service asus-shutdown.service

%postun
%systemd_postun_with_restart asusd.service asus-shutdown.service
udevadm control --reload >/dev/null 2>&1 || :

%files
%license LICENSE
%{_bindir}/asusctl
%{_bindir}/asusd
%{_bindir}/asus-shutdown
%{_unitdir}/asusd.service
%{_unitdir}/asus-shutdown.service
%{_udevrulesdir}/99-asusd.rules
%{_datadir}/asusd/
%{_datadir}/dbus-1/system.d/asusd.conf
