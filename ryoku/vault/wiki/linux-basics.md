# Linux Basics on Ryoku

Ryoku is Arch Linux underneath the desktop. Arch manages the base system;
Ryoku manages the desktop packages and config layered on top. Keeping those two
lanes clear makes updates and recovery much easier to understand.

## Packages

`pacman` is Arch's package manager:

```bash
pacman -Ss firefox          # search the repositories
pacman -Q firefox           # ask whether a package is installed
sudo pacman -S firefox      # install one package
sudo pacman -Rns firefox    # remove it and unneeded dependencies
sudo pacman -Syu            # update the Arch system and kernel
```

Read the proposed package list before accepting a transaction. Use
`ryoku update` for the Ryoku lane:

```bash
ryoku status
ryoku update
```

`ryoku update` snapshots first when Snapper is available, updates Ryoku's signed
package set or checkout channel, materializes config, reloads the desktop, and
runs the doctor. It does not move the kernel. `ryoku update --system` runs both
the Ryoku and distribution lanes when you deliberately want one combined run.

## Snapshots and rollback

List filesystem snapshots with:

```bash
ryoku snapshots
ryoku rollback
```

Bare `ryoku rollback` lists releases and snapshots. To return only the Ryoku
package set to a named release, use `ryoku rollback --to <tag>`. If the machine
cannot boot normally, choose a previous snapshot from the Limine boot menu, boot
it, and follow `ryoku rollback`'s restore guidance. A rollback of Ryoku packages
does not roll Arch or its kernel backward.

## User services and logs

Desktop daemons run as systemd user services. They belong to your login, so use
`--user` and do not add `sudo`:

```bash
systemctl --user status ryoku-shell
systemctl --user list-units
systemctl --user status rashind.socket
journalctl --user -u ryoku-shell -e
journalctl --user -u rashind.service --since today
```

`status` tells you whether a service is active and shows its newest log lines.
`journalctl --user -u ... -e` opens the journal at the end. Restart only after
you understand the error. The vault's `logs.md` gives the exact command for
each desktop subsystem without requiring Rashin.

## Where config and state live

- `~/.config/` holds per-user configuration.
- `~/.config/ryoku/` holds stores written by Ryoku tools. Use their GUI or
  command rather than editing the JSON by hand.
- `~/.config/ryoku/user_edits/` is the sparse override tree. A path there mirrors
  the same path under `~/.config` and is laid on top after every update.
- `~/.local/state/ryoku/` holds logs and update state.
- `~/.local/share/ryoku/vault/` is the Ryoku vault and this wiki.
- `/usr/share/ryoku/config/` is the packaged base. It is replaced by updates.

Before changing a config, ask its owner:

```bash
ryoku owner ~/.config/niri/user.kdl
ryoku owner ~/.config/hypr/user.lua
ryoku owner --map
```

The answer names the class, writer, advice, and exact edit path. A seed is written
once and then yours. A generated file belongs to a tool. A Ryoku file is shipped
again on update. An overlay or user override is yours. Follow the printed edit
path instead of guessing.

Next: take the [desktop tour](desktop.md), or open the compositor guide for
[Hyprland](hyprland-lua.md) or [niri](niri-kdl.md).
