Name:           gpk
Version:        0.7.0
Release:        1%{?dist}
Summary:        Cross-distribution package manager dashboard
License:        GPL-3.0-or-later
URL:            https://github.com/neur0map/glazepkg
Source0:        %{url}/releases/download/v%{version}/gpk-linux-amd64#/gpk-%{version}-x86_64
Source1:        https://raw.githubusercontent.com/neur0map/glazepkg/v%{version}/LICENSE#/gpk-LICENSE
ExclusiveArch:  x86_64
%global debug_package %{nil}

Provides:       gpk
Conflicts:      gpk-bin
Obsoletes:      gpk-bin < %{version}-%{release}

%description
GlazePKG provides one searchable terminal dashboard for package managers on
Linux, including dnf and xbps.

%prep
cp -p %{SOURCE1} LICENSE

%build

%install
install -Dpm0755 %{SOURCE0} %{buildroot}%{_bindir}/gpk

%files
%license LICENSE
%{_bindir}/gpk
