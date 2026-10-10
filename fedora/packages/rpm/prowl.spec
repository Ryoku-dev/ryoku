%global prowl_commit 5ba05eb922ff023e3cbc2b9a3768829179bca980
%global prowl_sha256 08fbd327fccd1210b19b621914f134e38eaf196ea216f11f9036e83c40c0212f

Name:           prowl
Version:        0.17.2
Release:        1%{?dist}
Summary:        Prowl model gateway and code intelligence for Ryoku Rashin
License:        MIT
URL:            https://github.com/neur0map/prowl
Source0:        ryoku-prowl-source.tar.gz
ExclusiveArch:  x86_64
%global debug_package %{nil}
BuildRequires:  bash
BuildRequires:  coreutils
BuildRequires:  findutils
BuildRequires:  tar
BuildRequires:  gcc
BuildRequires:  golang >= 1.26.4
BuildRequires:  glibc-devel
BuildRequires:  sqlite-devel
Requires:       glibc
Requires:       libgcc
Conflicts:      prowl-agent
Obsoletes:      prowl-agent < 1:0

%description
Prowl is the model gateway and code-intelligence engine behind Ryoku Rashin.
It includes its indexing model so project indexing does not download one at
runtime.

%prep
rm -rf source
mkdir source
tar -xf %{SOURCE0} -C source --strip-components=1

%build
cd source
export RYOKU_PKGVER=%{version}
export PROWL_COMMIT=%{prowl_commit}
bash fedora/packages/rpm/stage-package.sh %{name} "$PWD/stage" %{_libdir}

%install
cp -a source/stage/. %{buildroot}/
find %{buildroot} -type f -o -type l | sed 's|^%{buildroot}||' > rpm-files

%files -f rpm-files
