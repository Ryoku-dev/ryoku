Name:           ryoku-oh-my-zsh
Version:        0.1.0
Epoch:          1
Release:        1%{?dist}
Summary:        Oh My Zsh framework for the Ryoku Zsh integration
License:        MIT
URL:            https://github.com/ohmyzsh/ohmyzsh
Source0:        ryoku-%{version}.tar.gz
BuildArch:      noarch
Requires:       zsh
Provides:       oh-my-zsh = %{epoch}:%{version}-%{release}
Provides:       oh-my-zsh-git = %{epoch}:%{version}-%{release}
Conflicts:      oh-my-zsh
Conflicts:      oh-my-zsh-git
Obsoletes:      oh-my-zsh < %{epoch}:%{version}-%{release}
Obsoletes:      oh-my-zsh-git < %{epoch}:%{version}-%{release}

%description
Oh My Zsh packaged at the system path used by Ryoku's Zsh configuration. Ryoku
disables the framework's self-updater so RPM remains the owner of the tree.

%prep
rm -rf source
mkdir source
tar -xf %{SOURCE0} -C source --strip-components=1

%build

%install
install -d %{buildroot}%{_datadir}/oh-my-zsh
cp -a source/.rpm-ryoku-oh-my-zsh/oh-my-zsh/. \
  %{buildroot}%{_datadir}/oh-my-zsh/
install -Dpm 0644 %{buildroot}%{_datadir}/oh-my-zsh/LICENSE.txt \
  %{buildroot}%{_licensedir}/%{name}/LICENSE.txt
rm %{buildroot}%{_datadir}/oh-my-zsh/LICENSE.txt

%files
%license %{_licensedir}/%{name}/LICENSE.txt
%{_datadir}/oh-my-zsh/
