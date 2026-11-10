# Install Ryoku without its ISO

The shell installer adds Ryoku to an existing system. It does not repartition a
disk or replace the bootloader. It backs up every configuration file it changes
and puts a `restore.sh` script in the backup directory.

Run it as your normal user. It uses `sudo` when it needs to change the system.
Ryoku packages are built for `x86_64`, and the install needs a working internet
connection.

You can print the plan without changing anything:

```sh
curl -fsSL https://raw.githubusercontent.com/Ryoku-dev/ryoku/main/ryoku-shell-installer/install.sh | bash -s -- --dry-run
```

## Void Linux

Ryoku 1.0 supports the `x86_64` glibc edition of Void with runit. It does not
support Void musl or another init system.

### 1. Install a stock Void base

Download an `x86_64` glibc image from
[voidlinux.org/download](https://voidlinux.org/download/) and boot it. The
standard live image login is `root` with password `voidlinux`.

For the guided path, start the stock installer:

```sh
void-installer
```

Work through its keyboard, network, source, hostname, locale, time zone, root
password, user, bootloader, partition and filesystem screens. Choose DHCP when
it fits your network. Create a normal user during this step. Install the base
system, reboot into it, and confirm that it can reach the Void repositories.
The [Void installation guide](https://docs.voidlinux.org/installation/live-images/guide.html)
explains every screen.

For a manual base install, follow the
[Void manual installation](https://docs.voidlinux.org/installation/guides/chroot.html)
through `base-system`, the chroot setup and the bootloader. Reboot into the
installed system before running Ryoku's installer. Do not run it from the live
image or from the chroot.

### 2. Check the network, user and sudo

Log in as the normal user you created and check that `sudo true` works. If
`sudo` is missing or the user cannot use it, log in as root and run:

```sh
xbps-install -S sudo
usermod -aG wheel <your-user>
visudo
```

In `visudo`, uncomment this line:

```text
%wheel ALL=(ALL) ALL
```

Log out and back in after changing groups. Then refresh XBPS and install the
tools used to fetch the one-line installer:

```sh
sudo xbps-install -Syu
sudo xbps-install -S curl git
```

Keep the network method from the base install running. The Ryoku installer
replaces an enabled `dhcpcd` or `wpa_supplicant` setup with NetworkManager as
part of the reviewed plan.

### 3. Run the Ryoku installer

Run this as the normal user:

```sh
curl -fsSL https://raw.githubusercontent.com/Ryoku-dev/ryoku/main/ryoku-shell-installer/install.sh | bash
```

Review the plan in the terminal UI. Void offers niri only. Choose the browser,
login shell, file manager, optional apps and whether to install Rashin. Rashin
defaults to No and stays off until you turn it on. The run can be resumed if it
is interrupted.

Ryoku is niri-only on Void because Ryoku's Hyprland build needs GCC 15 while
Void currently ships GCC 14. Wayfire also stays out because Void ships 0.10 and
Ryoku needs 0.11.

### 4. What the installer sets up

The installer:

- verifies an `x86_64` Void glibc host booted with runit;
- trusts the Ryoku XBPS signing key, adds the signed repository and installs
  `ryoku-keyring`, `ryoku-desktop` and `ryoku-desktop-niri`;
- installs and registers the niri Wayland session;
- enables the required runit services, including D-Bus, polkit, Turnstile,
  NetworkManager and SDDM;
- configures Turnstile, PAM and the per-user D-Bus session, then provisions
  Ryoku's user services under `~/.config/service/`;
- installs SDDM and selects the Ryoku qylock greeter;
- saves the old configuration, materializes the Ryoku desktop, runs
  `ryoku doctor` and checks the result.

Turnstile is the user-session supervisor on runit. It gives Ryoku's user
services a lifetime tied to the login session. The D-Bus session is set up with
it because Void does not have systemd's user manager to provide either job.

Reboot when the installer finishes. SDDM should offer the niri session and use
the Ryoku greeter.

### 5. Update the installed system

Ryoku and the Void base are separate update lanes:

```sh
ryoku update            # Ryoku packages, configuration and doctor
ryoku update --system   # the same, plus the Void base and kernel
```

Use `ryoku update --system` for normal full updates. XBPS has no transaction
snapshot hook, so this command is the path that can wrap both lanes in Ryoku's
pre/post snapshots when the existing machine already has a btrfs root and
snapper. The shell installer does not create a snapshot layout or install a
bootloader on an existing Void system.

See [`../void/README.md`](../void/README.md) for the Void edition's limits,
runit contract and rollback behavior. See [`updates.md`](updates.md) for
channels and frozen releases.

## Existing Arch Linux

Use an `x86_64` Arch system booted with systemd. Update it, make sure the normal
user has working `sudo`, and install `curl` if it is missing:

```sh
sudo pacman -Syu
sudo pacman -S --needed curl
curl -fsSL https://raw.githubusercontent.com/Ryoku-dev/ryoku/main/ryoku-shell-installer/install.sh | bash
```

The installer adds the signed `[ryoku]` repository and installs the selected
Hyprland, niri or Wayfire desktop package. It can also add the optional CachyOS
gaming layer while keeping the Arch kernel as a fallback.

## Existing CachyOS

Update CachyOS and run the installer as a normal user with `sudo`:

```sh
sudo pacman -Syu
sudo pacman -S --needed curl
curl -fsSL https://raw.githubusercontent.com/Ryoku-dev/ryoku/main/ryoku-shell-installer/install.sh | bash
```

The pacman path keeps the CachyOS base and repositories. Ryoku adds its signed
repository, the selected Hyprland, niri or Wayfire package, SDDM and the desktop
configuration.

## Existing Fedora 44 or 45

Ryoku supports mutable `x86_64` Fedora 44 and 45 Workstation, KDE Plasma,
Server and spins. Atomic and other rpm-ostree systems are not supported. Fedora
uses Hyprland or niri. It does not offer Wayfire.

Update Fedora and run the installer as a normal user with `sudo`:

```sh
sudo dnf5 upgrade --refresh
sudo dnf5 install curl
curl -fsSL https://raw.githubusercontent.com/Ryoku-dev/ryoku/main/ryoku-shell-installer/install.sh | bash
```

Use `dnf` instead of `dnf5` when that is what the host provides. The installer
checks that Ryoku publishes packages for the running Fedora release before it
asks for sudo. It imports the Ryoku release key, writes the signed RPM
repository, adds the package-restricted COPRs, installs the chosen compositor
and configures SDDM. It keeps Fedora's bootloader and leaves NVIDIA on nouveau.
See [`../fedora/README.md`](../fedora/README.md) for Fedora-specific limits.

## NixOS

Do not run the shell installer on NixOS. Use
[Ryoku on NixOS](https://github.com/Ryoku-dev/Ryoku-on-NixOS), which provides
the flake, module and NixOS update path.
