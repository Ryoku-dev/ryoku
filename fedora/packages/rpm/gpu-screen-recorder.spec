Name:           gpu-screen-recorder
Version:        6.1.3
Release:        1%{?dist}
Summary:        Fast GPU-accelerated screen recorder for Linux
License:        GPL-3.0-only
URL:            https://git.dec05eba.com/gpu-screen-recorder
Source0:        https://dec05eba.com/snapshot/gpu-screen-recorder.git.%{version}.tar.gz#/%{name}-%{version}.tar.gz
ExclusiveArch:  x86_64
%global debug_package %{nil}

BuildRequires:  gcc
BuildRequires:  meson
BuildRequires:  ninja-build
BuildRequires:  pkgconfig
BuildRequires:  dbus-devel
BuildRequires:  ffmpeg-free-devel
BuildRequires:  libX11-devel
BuildRequires:  libXcomposite-devel
BuildRequires:  libXdamage-devel
BuildRequires:  libXfixes-devel
BuildRequires:  libXrandr-devel
BuildRequires:  libcap-devel
BuildRequires:  libdrm-devel
BuildRequires:  libva-devel
BuildRequires:  pipewire-devel
BuildRequires:  pulseaudio-libs-devel
BuildRequires:  vulkan-headers
BuildRequires:  wayland-devel
Requires:       bash
Requires:       libcap
Requires:       libjpeg-turbo
Requires:       mesa-dri-drivers
Recommends:     mesa-va-drivers
Suggests:       intel-media-driver

%description
GPU Screen Recorder captures Wayland and X11 desktops with the GPU. The package
includes the recorder, its IPC client, and the privileged KMS helper.

%prep
%autosetup -c

%build
%meson \
    -Dsystemd=true \
    -Dcapabilities=false \
    -Dnvidia_suspend_fix=true \
    -Dffmpeg_static=false
%meson_build

%install
%meson_install

%files
%license LICENSE
%{_bindir}/gpu-screen-recorder
%{_bindir}/gsr-cli
%caps(cap_sys_admin=ep) %{_bindir}/gsr-kms-server
%{_mandir}/man1/gpu-screen-recorder.1*
%{_mandir}/man1/gsr-cli.1*
%{_mandir}/man1/gsr-kms-server.1*
%{_userunitdir}/gpu-screen-recorder.service
%{_prefix}/lib/modprobe.d/gsr-nvidia.conf
%{_datadir}/gpu-screen-recorder/
%{_includedir}/gsr/
