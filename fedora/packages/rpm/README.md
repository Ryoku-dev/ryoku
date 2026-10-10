# Fedora integration

This directory contains Fedora RPM packaging, DNF host support, and the shell
installer. The installer targets mutable Fedora x86_64 systems using systemd.
Fedora images and ISO build files are intentionally not part of this integration.

## Install from a signed RPM repository

The repository and signing key must be supplied by the repository operator.
This checkout does not configure a public Fedora repository or signing key.

```sh
fedora/install.sh \
  --base-url https://packages.example.org/stable/fedora/\$releasever/\$basearch \
  --gpg-key-url https://packages.example.org/RPM-GPG-KEY-ryoku \
  --gpg-fingerprint 0123456789ABCDEF0123456789ABCDEF01234567 \
  --provider hyprland
```

The installer downloads the signing key, checks its full fingerprint against
the value supplied by the repository operator, imports the verified key, then
creates the DNF repository with package and metadata signature checks enabled.
It refreshes metadata and installs `ryoku`, `ryoku-desktop`, and the selected
compositor package. The desktop specs depend on several packages hosted in COPR.
Review `fedora/packages/rpm/dependency-coprs` and, if you accept those sources,
enable them before installation:

```sh
sudo fedora/packages/rpm/enable-dependencies.sh
```

The installer leaves third-party repository setup as an explicit step.

`ryoku track stable`, `ryoku track testing`, and release tags use the same
repository ID, `ryoku`. RPM packages include a Fedora package mapping table at
`/usr/share/ryoku/packages/fedora.tsv`.

## Build RPMs

On Fedora, install the RPM and build tools, Go, and the development libraries
listed as BuildRequires in the specs. Then prepare self-contained source RPMs:

```sh
RYOKU_SRPM_OUT=/tmp/ryoku-srpms fedora/packages/rpm/prepare-srpms.sh
```

For local edits in an uncommitted worktree, set `RYOKU_RPM_LOCAL=1`. Rebuild
the SRPMs in clean Mock roots with:

```sh
fedora/packages/rpm/rebuild-srpms.sh /tmp/ryoku-srpms /tmp/ryoku-rebuilt
```

Build a local repository with `fedora/packages/rpm/build-rpm-repo.sh`. Set
`RYOKU_RPM_UNSIGNED=1` only for local tests; unsigned output is not accepted by
the shell installer. Signed builds require an RPM signing key and sign both
packages and repository metadata. See the script usage and RPM spec files for
the exact build inputs.

The DNF host implementation is kept in
`fedora/packages/rpm/patches/0001-host-dnf-support.patch` and applied while staging
the RPM source tree. This keeps Fedora-specific build changes under this
directory and leaves shared upstream files untouched in this checkout.

## Scope and validation

The Fedora host supports DNF4 and DNF5 on mutable x86_64 installs. The package
mapping is maintained in `fedora/packages/rpm/payload/fedora.tsv`. Snapshot rollback
and AUR operations are not provided by this Fedora package layer.

Run the host tests from `ryoku/cli` with `go test ./internal/host`. The
extra-package helper has tests in `fedora/tests/fedora-extras.py`.
