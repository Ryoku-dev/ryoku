# Fedora RPM packaging

These specs build the Fedora edition from the shared Ryoku source tree. Payload
recipes reuse the package payloads under `release/packages`, with the small
Fedora-specific additions kept here.

Release builds are driven by the same version and channel values published to
users:

```sh
RYOKU_PKGVER=1.2.3 \
RYOKU_RELEASE=v1.2.3 \
RYOKU_CHANNEL=stable \
RYOKU_SRPM_OUT=/tmp/ryoku-srpms \
  fedora/packages/rpm/prepare-srpms.sh
```

The source preparation step copies tracked files only and vendors Go modules so
the RPM rebuilds do not need the network. For an uncommitted local build, set
`RYOKU_RPM_LOCAL=1`; only local builds may derive their own version and release
values.

Rebuild and assemble the signed repository with:

```sh
RYOKU_SRPM_IN=/tmp/ryoku-srpms \
RYOKU_RPM_OUT=/tmp/ryoku-rpm-repo \
RYOKU_RPM_SIGNING_KEY=EB6D3C0F55A7B3CABA6B2838847B274F025DD6E3 \
  fedora/packages/rpm/build-rpm-repo.sh
```

The output directory is ready to publish: binary RPMs, `repodata/`, the armored
`repomd.xml` signature and `release.json`. Source RPMs stay in the build
artifact and are not part of the public repository.
