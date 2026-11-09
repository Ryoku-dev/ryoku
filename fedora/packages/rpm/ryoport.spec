Name:           ryoport
Version:        1.0.0
Release:        1%{?dist}
Summary:        Fleet dashboard for virtual machines, servers, and consoles
License:        GPL-3.0-or-later
URL:            https://github.com/Ryoku-dev/ryoport
Source0:        ryoport-%{version}.tar.gz
ExclusiveArch:  x86_64
%global debug_package %{nil}

BuildRequires:  gcc
BuildRequires:  golang >= 1.26
BuildRequires:  make
BuildRequires:  pkgconf-pkg-config
BuildRequires:  wayland-devel
BuildRequires:  libxkbcommon-devel
BuildRequires:  libxkbcommon-x11-devel
BuildRequires:  libX11-devel
BuildRequires:  libXcursor-devel
BuildRequires:  libXfixes-devel
BuildRequires:  mesa-libEGL-devel
BuildRequires:  mesa-libGLES-devel
BuildRequires:  vulkan-headers
Requires:       hicolor-icon-theme
Suggests:       openssh-clients
Suggests:       quickemu
Suggests:       qemu-kvm
Suggests:       libvirt
Suggests:       edk2-ovmf
Suggests:       swtpm
Suggests:       virt-viewer
Suggests:       spice-gtk
Suggests:       spice-gtk-tools
Suggests:       xorriso
Suggests:       libisoburn
Suggests:       genisoimage
Suggests:       picocom
Suggests:       telnet
Suggests:       freerdp
Suggests:       tigervnc
Suggests:       libsecret
Suggests:       xdg-utils
Provides:       ryovm = %{version}-%{release}
Obsoletes:      ryovm < %{version}-%{release}

%description
Ryoport starts and connects to local virtual machines, GPU-passthrough guests,
SSH servers, and serial, Telnet, RDP, and VNC consoles.

%prep
%autosetup

%build
mkdir -p build
CGO_ENABLED=1 \
  GOFLAGS='-mod=vendor -modcacherw -buildmode=pie' \
  go build -trimpath -ldflags='-s -w' \
    -o build/ryoport ./cmd/ryoport

%install
make install SKIP_BUILD=1 DESTDIR=%{buildroot} PREFIX=%{_prefix}

%files
%{_bindir}/ryoport
%{_bindir}/ryovm
%{_bindir}/ryossh
%{_bindir}/ryovm-mon
%{_bindir}/ryovm-fetch
%{_datadir}/applications/dev.ryoku.ryoport.desktop
%{_datadir}/icons/hicolor/scalable/apps/dev.ryoku.ryoport.svg
%{_metainfodir}/dev.ryoku.ryoport.metainfo.xml
%license %{_datadir}/licenses/ryoport/LICENSE
