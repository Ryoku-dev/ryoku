# Installable profiles

An installable profile is a separate standard account set up for one purpose. It
has its own home folder, menus, dock, defaults and selected tools. Profiles do
not change the account that installs them.

Ryoku accepts official profiles only. They currently work on Arch and CachyOS,
and the Profiles category is visible only while the machine follows an unstable
channel. Fedora, Void and stable-channel machines do not offer them.

## Install and create an account

1. Open Ryostore and choose **Profiles**.
2. Open a profile and choose **Install**. This installs the profile definition,
   not its tools or account.
3. Choose **Create account**. Ryostore opens Ryoku Hub at **Global > Users > Add
   person > From a profile**.
4. Pick the account name and password, choose the core and optional tools, and
   add any extra package names you want.
5. Review the Hub plan. It lists repositories, packages, package removals,
   system changes, files, environment, warnings, blockers and **Also visible in
   other accounts**. Approve it with an administrator password only when the
   plan is correct.
6. When the profile check finishes, log out and choose the new account at the
   login screen.

Applying the same profile to its existing account updates the version and tool
selection. It never converts another ordinary account into a profile account.
Removing a profile definition in Ryostore does not break an existing account,
because applied accounts use a staged system copy.

## Separation and limits

A profile is a separate account, not a sandbox.

Its menus, dock and default apps show only its own tools, but programs installed
for the whole computer can still be started from its terminal. Apps it installs
also appear in the other accounts on this computer. Files in other accounts stay
private: each home folder is readable only by its owner.

Packages and repositories are machine-wide. A profile narrows its own launcher,
but it cannot hide installed programs from a terminal. Desktop apps installed by
the profile also appear in other accounts, and the review plan names them under
**Also visible in other accounts**. Tools and actions that need root access ask
for an administrator password. The profile account remains a standard account.

Apps installed for the whole computer later stay out of the profile account's
menus too: a user service watches the application folders and hides new entries,
while Ryoku's own apps and the profile's tools stay visible.

## The Hacker profile

Hacker is the first official profile. It is for Hack The Box, TryHackMe and
OverTheWire-style study. Core tools are selected by default. Optional tools can
be enabled in the Hub.

### Core tools

- **Recon:** Nmap, RustScan, WhatWeb and the Bind DNS tools.
- **Web:** Burp Suite, ffuf, Feroxbuster, Gobuster, sqlmap and Nikto.
- **Wordlists:** SecLists and DIRB.
- **AD and Windows:** Impacket, NetExec, Evil-WinRM, smbclient,
  enum4linux-ng and Kerbrute.
- **Passwords:** Hashcat, John the Ripper, Hydra and Name That Hash.
- **Exploitation:** Metasploit and ExploitDB.
- **Pivoting and shells:** Chisel, Ligolo-ng, ProxyChains, socat, rlwrap and
  OpenBSD netcat.
- **Post-exploitation:** PEASS and pspy.
- **Network:** OpenVPN, NetworkManager OpenVPN support, Wireshark and tcpdump.
- **Study:** Obsidian, Firefox and the NSS certificate tools.

### Optional tools

- **Recon:** Masscan and Nuclei.
- **Web:** WPScan and ZAP.
- **AD and Windows:** BloodHound Python, Certipy and Responder. Responder needs
  an administrator password to run and pulls in about 1.8 GB of MinGW
  cross-compiler packages.
- **Exploitation:** pwntools, Ghidra, GDB and Binwalk.
- **Pivoting and shells:** sshuttle.

The activation enables BlackArch before resolving the tools that need it. Ryoku
verifies the BlackArch keyring against pinned signing fingerprints, installs its
signed mirror-list package and keeps the marked `[blackarch]` stanza after the
other repositories in `pacman.conf`. It refreshes only the BlackArch database,
not the whole system as a partial upgrade.

### Wordlists and raw scans

The account exposes the usual paths expected by writeups:

- `/usr/share/seclists` is the SecLists package tree.
- `/usr/share/wordlists/seclists` links to that tree.
- `/usr/share/wordlists/dirb` links to `/usr/share/dirb/wordlists`.
- `/usr/share/wordlists/dirbuster/` contains links to the large, medium, small
  and lowercase DirBuster lists from SecLists.
- `/usr/share/wordlists/rockyou.txt` is extracted from SecLists' packaged
  `rockyou.txt.tar.gz` archive.

Ryoku places a profile-only copy of Nmap first on this account's `PATH` and gives
that copy the capabilities needed for raw scans. The system `/usr/bin/nmap` is
not changed for other accounts.

### Browser and notes

The account has normal **Firefox** and **Firefox (Burp proxy)**. The second uses
a dedicated Firefox profile pointed at Burp on `127.0.0.1:8080`. Start Burp and
run `burp-ca` to fetch Burp's current CA and trust it in that browser profile.

Obsidian opens a local `~/notes` vault on the first login. It includes a start
guide, methodology and cheatsheet notes, and a box template. Run `box new <name> [ip]`
to create an engagement tree under `~/engagements`, create a note from the
template and set the target when an IP is supplied. Use `box list` to list work
and `box path <name>` to find its directory.

### Labs, targets and the bar

Use the account helpers for routine lab state:

```bash
lab vpn import ~/Downloads/lab.ovpn
lab vpn up
target set 10.10.10.10 first-box
target hosts first-box.htb
box new first-box 10.10.10.10
```

`lab vpn import` copies the OpenVPN file to `~/vpn`, imports it into
NetworkManager and makes the connection belong to this account. `lab vpn up`,
`down`, `list`, `ip` and `status` manage or report it.

`target set`, `show` and `clear` manage the current target. Run `target hosts <name>...`
to add names for that IP through `ryoku-host hosts`. Allowed names end in
`.htb`, `.thm` or `.vl`, and only Ryoku's managed block in `/etc/hosts` is
changed. The **Lab status** bar widget shows the current target and `tun0`
address on the QS Bar.

Console, SSH and other login shells get the same profile environment, including
`PATH`, `WORDLISTS` and the other variables, because `~/.bash_profile` imports
the systemd user environment, so `target`, `box`, `lab` and the profile copy of
Nmap work there too.

## Change the installed tools

Open **Ryoku Hub > Global > Users**, expand the profile account and choose **Edit
tools**. Core and optional tools use the same grouped switches as account
creation. You can also add a package by its pacman name. The Hub prepares a new
plan and requires another review before applying it.

The same account card provides **Reset settings**, **Checkpoints** and **Remove
profile**. A profile account cannot be promoted to Administrator.

## Reset and troubleshoot

**Reset settings** previews and restores the files managed by the profile from
its staged payload. It never touches user-data folders or anything the profile
did not lay. Hacker's user data includes `~/notes`, `~/engagements`, `~/vpn` and
`~/.BurpSuite`. Reset does not uninstall packages or delete the account.

Run this inside the profile account when something looks incomplete:

```bash
ryoku profile check
```

The check validates selected tools, managed files, environment values, the rice,
wordlists and capability copies, then writes its result to
`~/.local/state/ryoku/profile/check.json`. Login also runs a shorter session
check and logs failures under the `ryoku-profile` journal tag.

## Checkpoints

Checkpoints are available only when `/home` uses btrfs and the account home is a
managed btrfs subvolume. The Hub can save, list, revert and delete them. A revert
requires the profile account to be fully logged out.

A checkpoint restores the account home to the saved snapshot, then copies the
current declared user-data folders back into it. Hacker therefore keeps the
current notes, engagements, VPN files and Burp data while profile settings return
to the checkpoint. Checkpoints do not snapshot system packages or a remote lab
target.

## Remove a profile account

**Remove profile** first shows a removal plan. Choose one account action:

- **Archive home** deletes the account and moves its home to
  `/home/.removed/<account>-<date>`.
- **Keep account** removes profile management but leaves the account and home.
- **Erase home** deletes the account and its home.

The plan removes a package only when this profile introduced it, no other
profile account claims it, and no installed package requires it. Packages that
were already present stay installed. Shared capability copies, wordlists and
other system files stay while another profile claims them. BlackArch is disabled
only when this profile enabled the marked repository and no other profile uses
it. Checkpoints owned by the removed profile are deleted. The account must be
logged out before it can be deleted.

## State and receipts

The store definition and its receipt belong to the installing user:

- `$XDG_DATA_HOME/ryoku/profiles/<id>/`
- `$XDG_STATE_HOME/ryoku/store/profiles/<id>.json`

Applied system state belongs to root:

- `/var/lib/ryoku/profiles/<id>/payload/<version>/` is the staged definition and
  home payload used by existing accounts.
- `/var/lib/ryoku/profiles/<id>/accounts/<account>.json` records package,
  repository, system-file, wordlist and checkpoint ownership.
- `/usr/local/lib/ryoku-profiles/<id>/bin/` contains profile-only capability
  copies such as Nmap.
- `/home/.checkpoints/<account>/` contains read-only btrfs checkpoints.

Each profile account keeps its own receipt at
`~/.local/state/ryoku/profile/receipt.json`. That receipt records managed files,
seeded files, environment ownership, launcher overrides and user-data folders.
The profile check result sits beside it as `check.json`.
The one-time welcome marker is
`~/.local/state/ryoku/profile/first-run-done`. Hacker keeps the current target
IP and name in `$XDG_STATE_HOME/ryoku/lab/target` and `target.name`, and its
saved VPN choice in the same lab state directory.
