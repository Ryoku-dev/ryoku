%global bibata_commit f4ccfe8abb63fddc7b3ce51a866fd8378395cb3d
%global recipe_commit fd19156df0ce100d6a7f02685e6afcafe36a2fa7

Name:           ryoku-cursor-material
Version:        1.5.0
Release:        1%{?dist}
Summary:        Material Bibata cursor following the Ryoku accent
License:        GPL-3.0-or-later AND MIT
URL:            https://github.com/SakibShahariar/material-bibata-cursor
Source0:        https://github.com/rtgiskard/bibata_cursor/archive/%{bibata_commit}.tar.gz#/bibata_cursor-%{bibata_commit}.tar.gz
Source1:        themes.json
Source2:        compile_bibata_material.fish
Source3:        metadata_generator.py
Source4:        ryoku-cursor-material-recolor
Source5:        https://raw.githubusercontent.com/SakibShahariar/material-bibata-cursor/%{recipe_commit}/LICENSE#/material-bibata-LICENSE
Source6:        https://raw.githubusercontent.com/spdx/license-list-data/v3.27.0/text/GPL-3.0-or-later.txt#/GPL-3.0.txt
BuildArch:      noarch
BuildRequires:  fish
BuildRequires:  jq
BuildRequires:  librsvg2-tools
BuildRequires:  python3
BuildRequires:  xcursorgen
Requires:       librsvg2-tools
Requires:       python3
Requires:       xcursorgen

%description
A Material Bibata cursor whose outline follows Ryoku's wallpaper accent. The
package ships a compiled fallback and the sources used for runtime recoloring.

%prep
%autosetup -n bibata_cursor-%{bibata_commit}

sed -i '1s|/usr/bin/env python$|/usr/bin/env python3|' src/cursor_utils.py
%build
mkdir -p build-root/scripts/bibata_cursor
cp -a src svg config build-root/scripts/bibata_cursor/
install -m 0644 %{SOURCE1} build-root/themes.json
install -m 0755 %{SOURCE2} build-root/scripts/compile_bibata_material.fish
install -m 0755 %{SOURCE3} build-root/scripts/metadata_generator.py
BIBATA_MATERIAL_INSTALL_DIR="$PWD/out" \
  fish build-root/scripts/compile_bibata_material.fish Ryoku

%install
install -d %{buildroot}%{_datadir}/icons
cp -a out/Bibata-Material-Ryoku %{buildroot}%{_datadir}/icons/
install -d %{buildroot}%{_datadir}/%{name}/bibata
cp -a src svg config %{buildroot}%{_datadir}/%{name}/bibata/
install -Dpm 0755 %{SOURCE4} %{buildroot}%{_bindir}/ryoku-cursor-material-recolor
install -Dpm 0644 %{SOURCE5} %{buildroot}%{_licensedir}/%{name}/LICENSE.MIT
install -Dpm 0644 %{SOURCE6} %{buildroot}%{_licensedir}/%{name}/LICENSE.GPL-3.0

%files
%license %{_licensedir}/%{name}/LICENSE.MIT
%license %{_licensedir}/%{name}/LICENSE.GPL-3.0
%{_bindir}/ryoku-cursor-material-recolor
%{_datadir}/icons/Bibata-Material-Ryoku/
%{_datadir}/%{name}/
