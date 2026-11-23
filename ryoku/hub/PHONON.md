# Local Phonon-2 preset

The Dictation catalogue offers Phonon-2 directly below Base English. Installing
the card provisions Python 3.13, CPU-only PyTorch, the pinned Fermion runtime,
the model, and its user service. It remains a local English model and needs
no cloud API key. Selecting it writes a `phonon-2` preset marker, directs
Voxtype to localhost port 8010, and starts its server before restarting Voxtype.
Switching away or disabling dictation stops the unused server. A private receipt
records the installed files. Uninstall validates that ownership, stops the
service, and removes the dedicated runtime, unpacked model, and unit. Existing
installations can be adopted without replacing their files. Changed or foreign
files cause uninstall to stop before deleting anything. Shared dependency caches
and other models remain available. Model installation is cancelable and leaves
the current selection intact on failure.

This development checkout builds the dictation backend into
`~/.local/libexec/ryoku-hub-phonon`. The `backend/local-phonon-hub` launcher goes
in `~/.local/bin/ryoku-hub` and delegates all commands except `voxtype` to the
packaged backend. The existing Hub card handles install, cancel, select and
remove. No shell page is forked. This local build uses the installed `uv` command
to provision the isolated Python environment.

Rebuild and deploy with `sh ryoku/hub/backend/deploy-local-phonon.sh` from the
checkout. The script replaces only its own local launcher and atomically
publishes the backend binary. `ryoku-hub voxtype download phonon-2` installs or
adopts the runtime; `ryoku-hub voxtype rmmodel phonon-2` validates and removes it.

To roll back, select Redux first, remove the local launcher, and remove the
Phonon-specific Voxtype service drop-in. The packaged backend then owns the
catalogue again. A package update can replace this local extension by shipping
these source changes normally.
