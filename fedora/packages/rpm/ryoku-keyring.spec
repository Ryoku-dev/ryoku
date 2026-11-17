Name:           ryoku-keyring
Version:        0.1
Epoch:          1
Release:        1%{?dist}
Summary:        Ryoku release signing key for the Ryoku RPM repository
License:        GPL-3.0-or-later
URL:            https://ryoku.dev
Source0:        ryoku-%{version}.tar.gz
BuildArch:      noarch

%description
The OpenPGP key that signs Ryoku's RPMs and repository metadata. The installer
verifies and imports it first; this package keeps the file that ryoku.repo
names as its gpgkey, which dnf imports a rotated key from, and the list of
revoked keys that ryoku doctor removes from the rpm keyring.

%prep
%setup -q -n ryoku-%{version}

%install
install -Dm644 keys/ryoku-release-key.pub.asc %{buildroot}%{_sysconfdir}/pki/rpm-gpg/RPM-GPG-KEY-ryoku
install -Dm644 release/packages/ryoku-keyring/ryoku-revoked %{buildroot}%{_datadir}/ryoku-keyring/revoked

%files
%{_sysconfdir}/pki/rpm-gpg/RPM-GPG-KEY-ryoku
%{_datadir}/ryoku-keyring/revoked
