{
  description = "Ryoku desktop packages for Nix";

  inputs = {
    # Match the isolated package universe used by Ryoku-on-NixOS until
    # upstream package compatibility is independently validated.
    nixpkgs.url = "github:NixOS/nixpkgs/56c02bc00adcf003215cc4bd996d6efaf4cff188";

    hyprglassSrc = {
      url = "github:hyprnux/hyprglass/v0.7.0";
      flake = false;
    };

    bibataMaterialSrc = {
      url = "github:rtgiskard/bibata_cursor/f4ccfe8abb63fddc7b3ce51a866fd8378395cb3d";
      flake = false;
    };

    imgbordersSrc = {
      url = "git+https://codeberg.org/zacoons/imgborders.git?ref=master";
      flake = false;
    };

    skwdWall = {
      url = "github:liixini/skwd-wall/nix";
    };

    ryomanagerSrc = {
      url = "github:aethctl/ryomanager/459c6714e31276f73025369e722cf4ab48eccbc3";
      flake = false;
    };

    ryotunesSrc = {
      url = "github:Ryoku-dev/ryotunes/ade86b744651b908f3946b6769853e5084a59ce1";
      flake = false;
    };

    ryoport = {
      url = "github:Ryoku-dev/ryoport";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = { self, nixpkgs, hyprglassSrc, bibataMaterialSrc, imgbordersSrc, ryomanagerSrc, skwdWall, ryotunesSrc, ryoport, ... }:
    let
      system = "x86_64-linux";

      # Temporary ABI compatibility with the currently working NixOS port.
      quickshellScreencopyFixOverlay = final: prev: {
        quickshell = prev.quickshell.overrideAttrs (old: {
          patches = (old.patches or [ ]) ++ [
            (prev.fetchurl {
              url = "https://github.com/quickshell-mirror/quickshell/commit/851ab0eac6bdfa9201a1a50e7bc3a6d29af25949.patch";
              hash = "sha256-g+8NQeFyGK9v76cPM9ogkMSfLH0SGPOyREFkTf6MJ7I=";
            })
          ];
        });
      };

      pkgs = import nixpkgs {
        inherit system;
        overlays = [ quickshellScreencopyFixOverlay ];
      };

      version = builtins.replaceStrings [ "\n" "\r" ] [ "" "" ]
        (builtins.readFile ./VERSION);

      skwdPaper = skwdWall.packages.${system}.skwd-paper;

      # Packaging edits must not invalidate the application source.
      ryokuSource = builtins.path {
        name = "ryoku-upstream-source";
        path = ./.;

        filter = path: type:
          let
            relative = pkgs.lib.removePrefix
              "${toString ./.}/"
              (toString path);
          in
            !(relative == "nix"
              || pkgs.lib.hasPrefix "nix/" relative
              || builtins.elem relative [ "flake.nix" "flake.lock" ]);
      };

      ryoku = import ./nix/packages {
        inherit pkgs version hyprglassSrc bibataMaterialSrc imgbordersSrc ryomanagerSrc ryotunesSrc skwdPaper;
        ryoportPackage = ryoport.packages.${system}.default;
        src = ryokuSource;
      };
    in
    {
      lib.version = version;

      packages.${system} = {
        ryoku-desktop = ryoku.bundle;
        ryoku-shell = ryoku.shell;
        ryoku-cli = ryoku.cli;
        ryoku-hub = ryoku.hub;
        ryoku-desktop-data = ryoku.desktopData;
        ryoku-qml = ryoku.qml;
        ryoku-wm-hyprland = ryoku.wmHyprland;
        ryoku-wm-niri = ryoku.wmNiri;
        ryoku-ryostore = ryoku.ryostore;
        ryoku-ryogami = ryoku.ryogami;
        ryoport = ryoku.ryoport;
        default = ryoku.bundle;
      };
    };
}
