{
  pkgs,
  src,
  hyprglassSrc,
  bibataMaterialSrc,
  imgbordersSrc,
  ryomanagerSrc,
  ryotunesSrc,
  ryoportPackage,
  skwdPaper,
  version,
}:

let
  qmlRoot = "lib/qt-6/qml";

  shell = import ./ryoku-shell.nix {
    inherit pkgs src;
  };

  desktopData = import ./ryoku-desktop-data.nix {
    inherit pkgs src;
  };

  wmHyprland = import ./ryoku-wm-provider.nix {
    inherit pkgs src;
    provider = "hyprland";
  };

  wmNiri = import ./ryoku-wm-provider.nix {
    inherit pkgs src;
    provider = "niri";
  };

  # Niri 26.04 retains layer-shell surfaces when a physical output is
  # removed, so repeated DPMS/hotplug cycles accumulate their buffers.
  # Upstream PR #4485 fixes #1457 by cleaning each output's layer map before
  # removing the output from the layout. Keep the patch vendored and pinned so
  # Ryoku builds remain reproducible while the upstream fix is still open.
  ryokuNiri = pkgs.niri.overrideAttrs (old: {
    patches = (old.patches or [ ]) ++ [
      ../patches/niri-output-layer-cleanup.patch
    ];
  });

  xwaylandSatellite = import ./xwayland-satellite.nix {
    inherit pkgs;
  };

  cli = import ./ryoku-cli.nix {
    inherit
      pkgs
      src
      version
      desktopData
      paletteBridge
      ;
  };

  hub = import ./ryoku-hub.nix {
    inherit pkgs src paletteBridge desktopData;
  };

  paletteBridge = import ./ryoku-palette-bridge.nix {
    inherit pkgs src;
  };

  rashin = import ./ryoku-rashin.nix {
    inherit pkgs src;
  };


  ryostore = import ./ryoku-ryostore.nix {
    inherit pkgs src;
  };

  ryomotion = import ./ryoku-ryomotion.nix {
    inherit pkgs;
  };

  livewall = import ./ryoku-livewall.nix {
    inherit pkgs src;
  };

  qmkHid = import ./qmk-hid.nix {
    inherit pkgs;
  };

  waifu2x = import ./waifu2x-ncnn-vulkan.nix {
    inherit pkgs;
  };

  ryogami = import ./ryoku-ryogami.nix {
    inherit
      pkgs
      src
      livewall
      waifu2x
      qmlRoot
      paletteBridge
      ;
  };

  ryotunesUnwrapped = import ./ryotunes-unwrapped.nix {
    inherit pkgs ryotunesSrc;
  };

  ryotunes = import ./ryoku-ryotunes.nix {
    inherit pkgs ryotunesUnwrapped;
    ryokuQml = qml;
  };

  ryomanager = import ./ryomanager.nix {
    inherit pkgs ryomanagerSrc;
  };

  keysounds = import ./ryoku-keysounds.nix {
    inherit pkgs src;
  };

  helpers = import ./ryoku-helpers.nix {
    inherit pkgs src;
  };

  hyprglass = import ./ryoku-hyprglass.nix {
    inherit pkgs;
    src = hyprglassSrc;
  };

  imgborders = pkgs.hyprlandPlugins.imgborders.overrideAttrs (_: {
    pname = "ryoku-imgborders";
    version = "unstable";
    src = imgbordersSrc;
  });

  hyprPlugins = import ./ryoku-hypr-plugins.nix {
    inherit
      pkgs
      hyprglass
      imgborders
      keysounds
      ;
  };

  cursorMaterial = import ./ryoku-cursor-material.nix {
    inherit pkgs;
    ryokuSrc = src;
    bibataSrc = bibataMaterialSrc;
  };

  mapleMonoNF = import ./ryoku-maple-mono-nf.nix {
    inherit pkgs;
  };

  blobs = import ./ryoku-blobs.nix {
    inherit pkgs src qmlRoot;
  };


  mkPureQmlModule =
    {
      pname,
      moduleName,
      sourcePath,
      description,
    }:
    pkgs.stdenvNoCC.mkDerivation {
      inherit pname;

      version = "unstable";
      src = src + sourcePath;

      dontBuild = true;

      installPhase = ''
        runHook preInstall

        dest="$out/${qmlRoot}/Ryoku/${moduleName}"

        mkdir -p "$dest"
        cp -a ./. "$dest/"

        rm -f "$dest/install.sh"

        runHook postInstall
      '';

      meta = {
        inherit description;
        homepage = "https://github.com/Ryoku-dev/ryoku-arch";
        license = pkgs.lib.licenses.gpl3Only;
        platforms = [ "x86_64-linux" ];
      };
    };

  ui = import ./ryoku-ui.nix {
    inherit pkgs src qmlRoot;
  };

  pluginKit = mkPureQmlModule {
    pname = "ryoku-plugin-kit";
    moduleName = "PluginKit";
    sourcePath = "/ryoku/shell/quickshell/plugins/kit";
    description = "Ryoku shell plugin Qt/QML framework";
  };

  frameBars = mkPureQmlModule {
    pname = "ryoku-framebars";
    moduleName = "FrameBars";
    sourcePath = "/ryoku/shell/framebars";
    description = "Ryoku FrameBars Qt/QML module";
  };

  wmHyprlandQml = mkPureQmlModule {
    pname = "ryoku-wm-hyprland-qml";
    moduleName = "Wm/Hyprland";
    sourcePath = "/ryoku/wm/hyprland/qml";
    description = "Ryoku Hyprland-specific Qt/QML bridge module";
  };

  qml = pkgs.runCommand "ryoku-qml-modules" {
    meta = {
      description = "Combined Ryoku Qt/QML module tree";
      homepage = "https://github.com/Ryoku-dev/ryoku-arch";
      license = pkgs.lib.licenses.gpl3Only;
      platforms = [ "x86_64-linux" ];
    };
  } ''
    mkdir -p "$out/${qmlRoot}/Ryoku"

    cp -a \
      ${ui}/${qmlRoot}/Ryoku/Ui \
      "$out/${qmlRoot}/Ryoku/Ui"

    cp -a \
      ${pluginKit}/${qmlRoot}/Ryoku/PluginKit \
      "$out/${qmlRoot}/Ryoku/PluginKit"

    cp -a \
      ${frameBars}/${qmlRoot}/Ryoku/FrameBars \
      "$out/${qmlRoot}/Ryoku/FrameBars"

    mkdir -p "$out/${qmlRoot}/Ryoku/Wm"

    cp -a \
      ${wmHyprlandQml}/${qmlRoot}/Ryoku/Wm/Hyprland \
      "$out/${qmlRoot}/Ryoku/Wm/Hyprland"

    cp -a \
      ${blobs}/${qmlRoot}/Ryoku/Blobs \
      ${blobs}/${qmlRoot}/Ryoku/Spatial3D \
      ${blobs}/${qmlRoot}/Ryoku/Bus \
      "$out/${qmlRoot}/Ryoku/"
  '';

  bundle = pkgs.symlinkJoin {
    name = "ryoku-desktop-bundle";

    meta = {
      description = "Complete Ryoku desktop runtime bundle";
      homepage = "https://github.com/Ryoku-dev/ryoku-arch";
      license = pkgs.lib.licenses.gpl3Only;
      platforms = [ "x86_64-linux" ];
    };

    paths = [
      shell
      cli
      hub
      paletteBridge
      rashin
      ryostore
      ryomotion
      ryoportPackage
      livewall
      ryogami
      skwdPaper
      ryotunes
      ryomanager
      qmkHid
      waifu2x
      desktopData
      wmHyprland
      wmNiri
      helpers
      qml
    ];
  };
in
{
  niri = ryokuNiri;

  ryoport = ryoportPackage;

  inherit
    shell
    cli
    hub
    paletteBridge
    rashin
    ryostore
    ryomotion
    livewall
    ryogami
    skwdPaper
    ryotunes
    ryomanager
    ryotunesUnwrapped
    keysounds
    qmkHid
    waifu2x
    desktopData
    wmHyprland
    wmNiri
    helpers
    hyprglass
    imgborders
    hyprPlugins
    cursorMaterial
    mapleMonoNF

    ui
    pluginKit
    frameBars
    wmHyprlandQml
    blobs
    qml

    xwaylandSatellite

    bundle
    ;
}
