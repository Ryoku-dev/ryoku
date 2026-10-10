Name:           sddm-theme-ryoku
Version:        0.1
Release:        1%{?dist}
Summary:        Ryoku orbital greeter theme for SDDM
License:        GPL-3.0-or-later
URL:            https://ryoku.dev
Source0:        ryoku-%{version}.tar.gz
BuildArch:      noarch

Requires:       sddm
Requires:       qt6-qtdeclarative
Requires:       qt6-qt5compat
Requires:       qt6-qtsvg
Requires:       qt6-qtmultimedia

%description
Ryoku clockwork orbital login screen theme for SDDM.

%prep
%setup -q -n ryoku-%{version}

%install
install -d %{buildroot}%{_datadir}/sddm/themes/ryoku
cp -a ryoku/lockscreen/qylock/themes/clockwork/orbital/. %{buildroot}%{_datadir}/sddm/themes/ryoku/

%files
%{_datadir}/sddm/themes/ryoku
