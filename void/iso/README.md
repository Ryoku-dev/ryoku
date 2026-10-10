# Void ISO (future)

This directory is the home of the Void live-image profile, the counterpart of
`installation/iso/` for Arch: the live rootfs skeleton, the XBPS package set
the image bakes, and the image build entry point. It is intentionally empty
until the installer work starts; nothing here is wired to any build today.

When it is built, it must not duplicate what exists:

- the installer TUI (`installation/tui/`) and the shell installer
  (`ryoku-shell-installer/`) are distro-routed already; the Void ISO drives
  the same engines through the `distro` seam, not a second installer;
- the live-image service set (getty, network, the repo bind) is Void's own
  concern: runit services under the image skeleton, not the systemd units
  `installation/iso/airootfs/etc/systemd/system/` carries;
- offline package baking is the XBPS analogue of
  `installation/iso/offline-repo.sh`: a resolved, signed repository closure
  the image installs from without network.
