%global ryoku_vendor cargo

Name:           songrec
Version:        0.8.0
Release:        1%{?dist}
Summary:        Unofficial Shazam client for Linux
License:        GPL-3.0-only
URL:            https://github.com/marin-m/SongRec
Source0:        %{url}/archive/%{version}.tar.gz#/%{name}-%{version}.tar.gz
Source100:      %{name}-%{version}-vendor.tar.gz
ExclusiveArch:  x86_64

BuildRequires:  alsa-lib-devel
BuildRequires:  cargo
BuildRequires:  cairo-devel
BuildRequires:  clang
BuildRequires:  dbus-devel
BuildRequires:  ffmpeg-free-devel
BuildRequires:  gcc
BuildRequires:  gdk-pixbuf2-devel
BuildRequires:  glib2-devel
BuildRequires:  gtk4-devel
BuildRequires:  libadwaita-devel
BuildRequires:  libsoup3-devel
BuildRequires:  openssl-devel
BuildRequires:  pango-devel
BuildRequires:  pipewire-devel
BuildRequires:  pkgconf-pkg-config
BuildRequires:  pulseaudio-libs-devel
BuildRequires:  rust
Requires:       hicolor-icon-theme

%description
SongRec recognizes audio from a microphone or an audio file through the Shazam
service and keeps a local history of matches.

%prep
%autosetup -n SongRec-%{version}
tar -xf %{SOURCE100}
mkdir -p .cargo
cat > .cargo/config.toml <<'EOF'
[source.crates-io]
replace-with = "vendored-sources"

[source.vendored-sources]
directory = "vendor"
EOF

%build
cargo build --offline --locked --release

%install
install -Dpm0755 target/release/songrec %{buildroot}%{_bindir}/songrec
install -Dpm0644 packaging/rootfs/usr/share/applications/re.fossplant.songrec.desktop \
  %{buildroot}%{_datadir}/applications/re.fossplant.songrec.desktop
install -Dpm0644 packaging/rootfs/usr/share/icons/hicolor/scalable/apps/re.fossplant.songrec.svg \
  %{buildroot}%{_datadir}/icons/hicolor/scalable/apps/re.fossplant.songrec.svg
install -Dpm0644 packaging/rootfs/usr/share/metainfo/re.fossplant.songrec.metainfo.xml \
  %{buildroot}%{_metainfodir}/re.fossplant.songrec.metainfo.xml
for translation in translations/locale/*/LC_MESSAGES/songrec.mo; do
  locale_path=${translation#translations/locale/}
  install -Dpm0644 "$translation" \
    "%{buildroot}%{_datadir}/locale/$locale_path"
done

%find_lang songrec

%files -f songrec.lang
%license LICENSE
%doc README.md
%{_bindir}/songrec
%{_datadir}/applications/re.fossplant.songrec.desktop
%{_datadir}/icons/hicolor/scalable/apps/re.fossplant.songrec.svg
%{_metainfodir}/re.fossplant.songrec.metainfo.xml
