{ pkgs, src }:

pkgs.stdenv.mkDerivation {
  pname = "ryoku-ryoget";
  version = "unstable";

  src = src + "/ryoku/shell/ryoget";

  nativeBuildInputs = [
    pkgs.go
  ];

  buildPhase = ''
    runHook preBuild

    export HOME="$TMPDIR"
    export GOCACHE="$TMPDIR/go-cache"
    export GOTOOLCHAIN=local
    export CGO_ENABLED=0

    go build \
      -mod=vendor \
      -trimpath \
      -tags nosqlite,purego_utp \
      -o ryoget \
      .

    go build \
      -mod=vendor \
      -trimpath \
      -o ryoseek \
      ./cmd/ryoseek

    runHook postBuild
  '';

  installPhase = ''
    runHook preInstall

    install -Dm755 ryoget "$out/bin/ryoget"
    install -Dm755 ryoseek "$out/bin/ryoseek"

    runHook postInstall
  '';

  meta = {
    description = "Ryoku download engine (ryoget) and search daemon (ryoseek) behind the bar";
    homepage = "https://github.com/Ryoku-dev/ryoku";
    license = pkgs.lib.licenses.gpl3Only;
    platforms = [ "x86_64-linux" ];
  };
}
