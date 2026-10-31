%global debug_package %{nil}
%global hyprland_evr %(rpm -q --qf '%%{EVR}' hyprland)
%global ryoku_vendor cargo
%global ryoku_vendor_dir .rpm-hyprland-preview-share-picker/hyprland-preview-share-picker
Name:           hyprland-preview-share-picker
Version:        0.3.0
Release:        1%{?dist}
Summary:        Preview screen share picker for Hyprland
License:        MIT
URL:            https://github.com/WhySoBad/hyprland-preview-share-picker
Source0:        ryoku-%{version}.tar.gz
Source100:      %{name}-%{version}-vendor.tar.gz
BuildRequires:  bash
BuildRequires:  cargo
BuildRequires:  coreutils
BuildRequires:  findutils
BuildRequires:  gcc
BuildRequires:  git
BuildRequires:  gtk4-devel
BuildRequires:  gtk4-layer-shell-devel
BuildRequires:  pkgconf-pkg-config
BuildRequires:  rust
BuildRequires:  wayland-devel
BuildRequires:  hyprland-devel
Requires:       glibc
Requires:       gtk4
Requires:       gtk4-layer-shell
Requires:       hyprland%{?_isa} = %{hyprland_evr}
Requires:       libgcc
Requires:       libstdc++
Requires:       libwayland-client
Requires:       libwayland-cursor
Requires:       libwayland-egl
Requires:       libwayland-server
Requires:       slurp
Requires:       xdg-desktop-portal-hyprland

%description
A GTK preview picker for choosing Hyprland screen share sources.

%prep
%setup -q -n ryoku-%{version}
tar -xzf %{SOURCE100} -C %{ryoku_vendor_dir}
mkdir -p %{ryoku_vendor_dir}/.cargo
cat > %{ryoku_vendor_dir}/.cargo/config.toml <<'CARGO'
[source.crates-io]
replace-with = "vendored-sources"

[source.vendored-sources]
directory = "vendor"
CARGO

%build
export RYOKU_PKGVER=%{version}
bash fedora/packages/rpm/stage-package.sh %{name} "$PWD/stage" %{_libdir}

%install
cp -a stage/. %{buildroot}/
find %{buildroot} -type f -o -type l | sed 's|^%{buildroot}||' > rpm-files

%files -f rpm-files
