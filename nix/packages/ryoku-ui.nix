{ pkgs, src, qmlRoot }:

# Ryoku.Ui and Ryoku.Ui.Singletons compiled by qmlcachegen (ryoku/ui/CMakeLists.txt).
# Only the qmldir, qmltypes and plugin of each module are installed; the QML
# sources ride inside the plugin.
pkgs.stdenv.mkDerivation {
  pname = "ryoku-ui";
  version = "unstable";

  dontWrapQtApps = true;

  src = src + "/ryoku/ui";

  nativeBuildInputs = [
    pkgs.cmake
    pkgs.ninja
    pkgs.qt6.qtdeclarative
  ];

  buildInputs = [
    pkgs.qt6.qtbase
    pkgs.qt6.qtdeclarative
  ];

  preConfigure = ''
    rm -rf build
  '';

  installPhase = ''
    runHook preInstall

    for module in Ui Ui/Singletons; do
      dest="$out/${qmlRoot}/Ryoku/$module"
      mkdir -p "$dest"
      cp -a "qml/Ryoku/$module"/*.so "qml/Ryoku/$module/qmldir" \
        "qml/Ryoku/$module"/*.qmltypes "$dest/"
    done

    runHook postInstall
  '';

  meta = {
    description = "Shared Ryoku Qt/QML user interface module";
    homepage = "https://github.com/Ryoku-dev/ryoku-arch";
    license = pkgs.lib.licenses.gpl3Only;
    platforms = [ "x86_64-linux" ];
  };
}
