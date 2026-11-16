%global readex_commit 563dfbb36ae45e52ec50829b016ce724ac2fca70

Name:           ttf-readex-pro
Version:        1.204
Release:        1%{?dist}
Summary:        Readex Pro variable TTF used for Shima titles
License:        OFL-1.1
URL:            https://github.com/ThomasJockin/readexpro
Source0:        %{url}/archive/%{readex_commit}.tar.gz#/readexpro-%{readex_commit}.tar.gz
BuildArch:      noarch
Requires:       fonts-filesystem

%description
Readex Pro is the variable display sans used for titles in Ryoku's Shima frame.

%prep
%autosetup -n readexpro-%{readex_commit}

%build

%install
install -d %{buildroot}%{_datadir}/fonts/%{name}
install -m 0644 'fonts/variable/Readexpro[HEXP,wght].ttf' \
  %{buildroot}%{_datadir}/fonts/%{name}/

%files
%license OFL.txt
%{_datadir}/fonts/%{name}/
