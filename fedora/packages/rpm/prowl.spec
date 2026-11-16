%global prowl_commit 173fc779b17008a1a0d495313eaf2ba31c185103
%global ryoku_vendor go

Name:           prowl
Version:        0.17.3
Release:        1%{?dist}
Summary:        Prowl model gateway and code intelligence for Ryoku Rashin
License:        MIT
URL:            https://github.com/neur0map/prowl
Source0:        https://github.com/neur0map/prowl/archive/%{prowl_commit}.tar.gz#/prowl-%{prowl_commit}.tar.gz
Source100:      %{name}-%{version}-vendor.tar.gz
ExclusiveArch:  x86_64
%global debug_package %{nil}
BuildRequires:  gcc
BuildRequires:  golang >= 1.26.4
BuildRequires:  glibc-devel
BuildRequires:  sqlite-devel
Requires:       glibc
Requires:       libgcc
Requires:       libstdc++
Conflicts:      prowl-agent
Obsoletes:      prowl-agent < 1:0

%description
Prowl is the model gateway and code-intelligence engine behind Ryoku Rashin.
It includes its indexing model so project indexing does not download one at
runtime.

%prep
%autosetup -n prowl-%{prowl_commit}
tar -xf %{SOURCE100}

%build
CGO_ENABLED=1 \
  GOFLAGS='-mod=vendor -modcacherw -buildmode=pie -trimpath' \
  go build -tags sqlite_fts5 \
    -ldflags '-s -w -X main.version=v%{version} -X main.commit=%{prowl_commit} -X main.managedBy=dnf' \
    -o prowl ./cmd/prowl

%install
install -Dm755 prowl %{buildroot}%{_bindir}/prowl

%files
%license LICENSE NOTICE.md
%{_bindir}/prowl
