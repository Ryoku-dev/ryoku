{ pkgs, src, qmlRoot }:

# the QML sources and both faces ride inside the plugin, so only qmldir,
# qmltypes and the .so go out; the build reads qml/ and go/fonts/ side by side
pkgs.stdenv.mkDerivation {
  pname = "ryoku-lithic";
  version = "unstable";

  dontWrapQtApps = true;

  src = src + "/docs/lithic/library";
  cmakeDir = "../qml";

  nativeBuildInputs = [
    pkgs.cmake
    pkgs.ninja
    pkgs.qt6.qtdeclarative
  ];

  buildInputs = [
    pkgs.qt6.qtbase
    pkgs.qt6.qtdeclarative
  ];

  installPhase = ''
    runHook preInstall

    dest="$out/${qmlRoot}/Lithic"
    mkdir -p "$dest"
    cp -a qml/Lithic/*.so qml/Lithic/qmldir qml/Lithic/*.qmltypes "$dest/"
    install -Dm644 ../go/fonts/Geologica-OFL.txt "$out/share/licenses/ryoku-lithic/LICENSE.geologica"
    install -Dm644 ../go/fonts/MartianMono-OFL.txt "$out/share/licenses/ryoku-lithic/LICENSE.martian-mono"

    runHook postInstall
  '';

  meta = {
    description = "Lithic design system Qt/QML module";
    homepage = "https://github.com/Ryoku-dev/ryoku-arch";
    license = [
      pkgs.lib.licenses.gpl3Only
      pkgs.lib.licenses.ofl
    ];
    platforms = [ "x86_64-linux" ];
  };
}
