Name:           ryoku-extras
Version:        0.1
Release:        1%{?dist}
Summary:        Verified desktop tools, fonts and cursor assets
License:        GPL-3.0-or-later AND OFL-1.1 AND BSD-3-Clause
URL:            https://ryoku.dev
Source0:        ryoku-%{version}.tar.gz
BuildRequires:  python3
%global debug_package %{nil}

%description
Pinned and checksum-verified desktop extras, included in the source payload.

%prep
%setup -q -n ryoku-%{version}

%build
export RYOKU_EXTRA_CACHE="$PWD/.rpm-extras"
python3 - <<'PY'
import importlib.machinery
from pathlib import Path
extra = importlib.machinery.SourceFileLoader('extra', 'fedora/system/extras/ryoku-install-extra').load_module()
# The buildroot has no font cache consumers; RPM's font trigger updates it.
extra.shutil.which = lambda name: None
for name in extra.RELEASES:
    extra.install(name, Path('stage/usr'))
PY
mkdir -p stage/usr/share/ryoku
mv stage/usr/state/ryoku/extras stage/usr/share/ryoku/extras
rm -r stage/usr/state

%install
cp -a stage/. %{buildroot}/

%files
%{_bindir}/prowl-agent
%{_bindir}/matugen
%{_bindir}/qmk_hid
%{_datadir}/fonts/*
%{_datadir}/icons/Bibata-*
%{_datadir}/ryoku/extras
%license %{_datadir}/licenses/Bibata/LICENSE
