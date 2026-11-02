Name:           voxtype
Version:        1.1.1
Release:        1%{?dist}
Summary:        Push-to-talk voice-to-text for Wayland compositors
License:        MIT
URL:            https://github.com/peteonrails/voxtype
Source0:        %{url}/releases/download/v%{version}/voxtype-%{version}-1.x86_64.rpm
ExclusiveArch:  x86_64
%global debug_package %{nil}
# Upstream's engine providers keep their CI and ROCm search paths as RUNPATH;
# the loader skips entries that do not exist, and ROCm users need /opt/rocm.
%global __brp_check_rpaths %{nil}
AutoReqProv:     no

BuildRequires:  cpio
BuildRequires:  rpm
BuildRequires:  systemd-rpm-macros
Requires:       alsa-lib
Requires:       curl
Requires:       glibc
Requires:       pipewire-alsa
Suggests:       libnotify
Suggests:       pipewire
Suggests:       vulkan-loader
Suggests:       wl-clipboard
Suggests:       wtype
Provides:       voxtype-bin = %{version}-%{release}
Conflicts:      voxtype-bin

%description
Voxtype provides local push-to-talk dictation with CPU and optional GPU speech
engines. This package carries upstream's signed-release binary payload.

%prep
mkdir payload
cd payload
rpm2cpio %{SOURCE0} | cpio -idm
rm -rf usr/lib/.build-id

%build

%install
cp -a payload/. %{buildroot}/

%files
%config(noreplace) %{_sysconfdir}/voxtype/config.toml
%{_bindir}/voxtype*
%{_prefix}/lib/voxtype/
%{_userunitdir}/voxtype.service
%{_datadir}/bash-completion/completions/voxtype
%{_datadir}/fish/vendor_completions.d/voxtype.fish
%{_datadir}/voxtype/
%{_datadir}/zsh/site-functions/_voxtype
%license %{_docdir}/voxtype/LICENSE
%doc %{_docdir}/voxtype/README.md
