Name:           mise
Version:        2026.10.25
Release:        1%{?dist}
Summary:        Front end for development environments
License:        MIT
URL:            https://github.com/jdx/mise
Source0:        %{url}/releases/download/v%{version}/mise-v%{version}-linux-x64.tar.gz
ExclusiveArch:  x86_64
%global debug_package %{nil}

BuildRequires:  tar
Conflicts:      rtx
Obsoletes:      rtx < %{version}-%{release}

%description
mise manages development tool versions, environment variables, and project
tasks from one command.

%prep
rm -rf mise
mkdir mise
tar -xzf %{SOURCE0} -C mise --strip-components=1

%build

%install
install -Dpm0755 mise/bin/mise %{buildroot}%{_bindir}/mise
install -Dpm0644 mise/bin/mise.d %{buildroot}%{_libexecdir}/mise/mise.d
install -Dpm0644 mise/man/man1/mise.1 %{buildroot}%{_mandir}/man1/mise.1
install -Dpm0644 mise/share/fish/vendor_conf.d/mise-activate.fish \
  %{buildroot}%{_datadir}/fish/vendor_conf.d/mise-activate.fish

%files
%license mise/LICENSE
%doc mise/README.md
%{_bindir}/mise
%{_libexecdir}/mise/mise.d
%{_mandir}/man1/mise.1*
%{_datadir}/fish/vendor_conf.d/mise-activate.fish
