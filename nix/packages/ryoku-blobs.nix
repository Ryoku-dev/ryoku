{ pkgs, src, qmlRoot }:

pkgs.stdenv.mkDerivation {
  pname = "ryoku-blobs";
  version = "unstable";

  dontWrapQtApps = true;

  src = src + "/ryoku/shell/plugin";

  nativeBuildInputs = [
    pkgs.cmake
    pkgs.ninja
    pkgs.pkg-config
    pkgs.qt6.qtshadertools
    pkgs.qt6.qtquick3d
  ];

  buildInputs = [
    pkgs.qt6.qtbase
    pkgs.qt6.qtdeclarative
    pkgs.qt6.qtmultimedia
    pkgs.qt6.qtshadertools
    pkgs.qt6.qtquick3d
  ];

  preConfigure = ''
    rm -rf build
  '';

  installPhase = ''
    runHook preInstall

    mkdir -p "$out/${qmlRoot}/Ryoku"

    cp -a \
      qml/Ryoku/Blobs \
      qml/Ryoku/Spatial3D \
      qml/Ryoku/Bus \
      "$out/${qmlRoot}/Ryoku/"

    rm -f "$out/${qmlRoot}/Ryoku/Blobs/"*.qrc \
      "$out/${qmlRoot}/Ryoku/Spatial3D/"*.qrc \
      "$out/${qmlRoot}/Ryoku/Bus/"*.qrc

    runHook postInstall
  '';

  meta = {
    description = "Ryoku.Blobs, Ryoku.Spatial3D and Ryoku.Bus Qt/QML modules";
    homepage = "https://github.com/Ryoku-dev/ryoku-arch";
    license = pkgs.lib.licenses.gpl3Only;
    platforms = [ "x86_64-linux" ];
  };
}
