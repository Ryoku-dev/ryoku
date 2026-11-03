# Shima runtime compatibility patch

The standalone Shima runtime is distributed separately from Ryoku's embedded iRiS source. This patch targets its writable 2.33.0 staging tree before packaging, not user settings or the Nix store.

```sh
git -C "$shima_source" apply --unidiff-zero --check shima-overview-terminal.patch
git -C "$shima_source" apply --unidiff-zero shima-overview-terminal.patch
```

It restores the Niri overview wallpaper layer and makes automatic terminal recolouring opt-in. Existing explicitly enabled settings remain respected. The embedded iRiS default is changed separately in `modules/common/IrisDefaults.js`.

The patch is not applied automatically by this repository. The standalone package builder must apply it to its source before installation.
