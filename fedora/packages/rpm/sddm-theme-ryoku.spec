Name:           sddm-theme-ryoku
Version:        0.1
Release:        1%{?dist}
Summary:        Ryoku SDDM greeter directory and the Qt modules its skins need
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
The fixed SDDM theme directory Ryoku lays the picked login skin into, and the Qt
modules those skins import. The skin inside is the user's choice, so this package
owns only the directory; ryoku-desktop seeds the default on first install.

%prep
%setup -q -n ryoku-%{version}

%install
install -d %{buildroot}%{_datadir}/sddm/themes/ryoku

%files
%dir %{_datadir}/sddm/themes/ryoku
