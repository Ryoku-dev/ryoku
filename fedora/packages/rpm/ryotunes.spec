%global ryoku_vendor cargo
%global debug_package %{nil}

Name:           ryotunes
Epoch:          1
Version:        1.1.6
Release:        1%{?dist}
Summary:        Native music app for the Ryoku desktop
License:        GPL-3.0-or-later
URL:            https://github.com/ryoku-dev/ryotunes
Source0:        ryotunes-%{version}.tar.gz
Source1:        ryotunes-%{version}-node_modules.tar.gz
Source100:      %{name}-%{version}-vendor.tar.gz
ExclusiveArch:  x86_64

BuildRequires:  cargo
BuildRequires:  rust
BuildRequires:  gcc
BuildRequires:  gcc-c++
BuildRequires:  clang
BuildRequires:  nodejs
BuildRequires:  pnpm
BuildRequires:  webkit2gtk4.1-devel
BuildRequires:  libappindicator-gtk3-devel
BuildRequires:  mpv-devel
BuildRequires:  openssl-devel
BuildRequires:  librsvg2-devel
BuildRequires:  desktop-file-utils
BuildRequires:  systemd-rpm-macros
Requires:       webkit2gtk4.1
Requires:       libappindicator-gtk3
Requires:       mpv-libs
Requires:       yt-dlp
Requires:       /usr/bin/ffmpeg
Requires:       openssl-libs
Requires:       librsvg2
Requires:       desktop-file-utils
Requires:       hicolor-icon-theme
Requires:       xdg-utils
Requires:       polkit
Requires:       quickshell >= 0.3.0

%description
Ryotunes is Ryoku's native music client and background playback daemon. It
includes the QML client, command-line client, systemd user socket, and themes.

%prep
%autosetup -n %{name}-%{version}
tar -xf %{SOURCE100}
tar -xzf %{SOURCE1} -C ui
mkdir -p .cargo
cat > .cargo/config.toml <<'EOF'
[source.crates-io]
replace-with = "vendored-sources"

[source."git+https://github.com/nolight132/librespot?rev=e7eb953d5848fd97bacd00e6c0e33500765eaa63"]
git = "https://github.com/nolight132/librespot"
rev = "e7eb953d5848fd97bacd00e6c0e33500765eaa63"
replace-with = "vendored-sources"

[source.vendored-sources]
directory = "vendor"
EOF

%build
RUSTFLAGS="${RUSTFLAGS/-Cdebuginfo=2/-Cdebuginfo=0}"
RUSTFLAGS="${RUSTFLAGS/-Cstrip=none/-Cstrip=symbols}"
export RUSTFLAGS
pnpm --dir ui run build
cargo build --release --offline --locked \
    --package ryotunes \
    --package sync-server \
    --package ryotunesd \
    --package ryotunes-cli

%install
install -Dm0755 target/release/ryotunes %{buildroot}%{_bindir}/ryotunes
install -Dm0755 target/release/ryotunes-sync %{buildroot}%{_bindir}/ryotunes-sync
install -Dm0755 target/release/ryotunesd %{buildroot}%{_bindir}/ryotunesd
install -Dm0755 target/release/ryotunes-cli %{buildroot}%{_bindir}/ryotunes-cli
install -Dm0755 packaging/linux/ryotunes-qml %{buildroot}%{_bindir}/ryotunes-qml
install -Dm0755 scripts/diagnostics.sh %{buildroot}%{_bindir}/ryotunes-diagnostics

install -dm0755 %{buildroot}%{_datadir}/ryotunes/client
cp -a client/. %{buildroot}%{_datadir}/ryotunes/client/
rm -rf %{buildroot}%{_datadir}/ryotunes/client/tests
install -dm0755 %{buildroot}%{_datadir}/ryotunes/skins
cp -a skins/. %{buildroot}%{_datadir}/ryotunes/skins/
install -Dm0644 matugen/ryotunes.json %{buildroot}%{_datadir}/ryotunes/matugen/ryotunes.json

install -Dm0644 packaging/linux/ryotunesd.socket %{buildroot}%{_userunitdir}/ryotunesd.socket
install -Dm0644 packaging/linux/ryotunesd.service %{buildroot}%{_userunitdir}/ryotunesd.service
install -Dm0644 packaging/linux/90-ryotunes.preset %{buildroot}%{_userpresetdir}/90-ryotunes.preset
install -Dm0644 packaging/linux/ryotunes.desktop %{buildroot}%{_datadir}/applications/ryotunes.desktop
install -Dm0644 packaging/linux/dev.ryoku.ryotunes.metainfo.xml \
    %{buildroot}%{_metainfodir}/dev.ryoku.ryotunes.metainfo.xml
install -Dm0644 src-tauri/icons/32x32.png %{buildroot}%{_datadir}/icons/hicolor/32x32/apps/ryotunes.png
install -Dm0644 src-tauri/icons/64x64.png %{buildroot}%{_datadir}/icons/hicolor/64x64/apps/ryotunes.png
install -Dm0644 src-tauri/icons/128x128.png %{buildroot}%{_datadir}/icons/hicolor/128x128/apps/ryotunes.png
install -Dm0644 src-tauri/icons/128x128@2x.png %{buildroot}%{_datadir}/icons/hicolor/256x256/apps/ryotunes.png
install -Dm0644 src-tauri/icons/icon.png %{buildroot}%{_datadir}/icons/hicolor/512x512/apps/ryotunes.png

desktop-file-validate %{buildroot}%{_datadir}/applications/ryotunes.desktop

%post
%systemd_user_post ryotunesd.socket

%preun
%systemd_user_preun ryotunesd.socket

%postun
%systemd_user_postun_with_restart ryotunesd.service

%files
%license LICENSE
%doc README.md UPSTREAM.md
%{_bindir}/ryotunes
%{_bindir}/ryotunes-sync
%{_bindir}/ryotunesd
%{_bindir}/ryotunes-cli
%{_bindir}/ryotunes-qml
%{_bindir}/ryotunes-diagnostics
%{_datadir}/ryotunes/
%{_userunitdir}/ryotunesd.socket
%{_userunitdir}/ryotunesd.service
%{_userpresetdir}/90-ryotunes.preset
%{_datadir}/applications/ryotunes.desktop
%{_metainfodir}/dev.ryoku.ryotunes.metainfo.xml
%{_datadir}/icons/hicolor/*/apps/ryotunes.png
