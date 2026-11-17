# When something breaks

The user says an app or a part of the desktop stopped working: "the bar is gone",
"my wallpaper is grey", "Firefox won't launch anymore", "sound died after the
update". Do not start editing. Find out what happened first, then make the
smallest reversible fix through the tool that owns the thing.

## 1. Gather the evidence

One command pulls together everything relevant to a target: its systemd units,
the journal, coredumps, the app's own log files, and its recent package history.

```bash
ryoku-rashin logs <target>          # an app, or a component alias
ryoku-rashin logs shell --since 2h  # widen or narrow the window
```

`<target>` is a program name (`firefox`, `mpv`) or a Ryoku component alias:
`shell`, `wallpaper`, `hub`, `idle`, `audio`, `portals`, `updates`, `compositor`,
`rashin`. Each section is headed by the exact command or file it came from, so
you can rerun one and dig deeper. The vault's `logs.md` lists every log location
on this machine if you want to go straight to a source.

What to look at, and why:

- **The journal.** `journalctl --user -u <unit> -e` for a Ryoku unit,
  `journalctl --user _COMM=<binary> -e` for a program. Warnings and errors near
  the time it broke are the story.
- **Coredumps.** `coredumpctl list` shows crashes; `coredumpctl info <binary>`
  heads the newest. A crash is not a config problem.
- **The doctor report** (`~/.local/state/ryoku/doctor-report.txt`) and the update
  log. The doctor already checks the session, the shell, and the wallpaper.
- **`/var/log/pacman.log`.** An upgrade right before a breakage is the classic
  cause. `ryoku-rashin logs <app>` shows the package's recent transactions.

## 2. Diagnose before you touch anything

Form a specific cause from the evidence: a bad value in a config, a package that
regressed, a service that failed to start, a missing file. If the logs do not
point somewhere, gather more (a wider `--since`, the next log the section names)
rather than guessing.

## 3. Fix through the owning command, not a hand edit

Run `ryoku owner <path>` on any file you think you need to change. It names the
writer and where the change belongs. Then:

- If a Ryoku setting is wrong, drive it through its command or GUI (the tool that
  owns it is the one writer): `ryoku-shell` for the bar and dock, `ryogami` for
  wallpaper and colours, Ryoku Settings for the rest.
- If it is a real config edit, put it in the tool's own user file (the
  compositor's `user.*` file, `kitty/user.conf`, `fish/user.fish`; `ryoku owner`
  names it) or a fork under `~/.config/ryoku/user_edits/`, never in a shipped file.
- If a package regressed, that is a system problem, not a config one: see the
  rollback paths below.
- If a systemd user unit needs an override (`systemctl --user edit <unit>`),
  write paths in it with `%h` (the user's home) instead of a literal
  `/home/<name>`: user units and drop-ins can apply to every account, and
  another user cannot read your home.

Then reload with the tool that owns the surface (`ryoku reload` for the shell,
`ryoku-idle apply` for idle, and so on), not by killing processes.

## 4. Verify

Confirm the fix the same way you found the fault: rerun `ryoku-rashin logs
<target>`, watch the unit come up, or reproduce the user's action. A change that
you have not seen work is not done.

## 5. Rollback paths

Some breakage is best undone, not patched:

- **A bad update:** `ryoku rollback` lists the releases and snapshots to go back
  to; `ryoku rollback --to <tag>` puts the Ryoku set back on a release.
- **A broken boot or a wedged filesystem:** the machine keeps Limine/Btrfs
  snapshots; boot the previous snapshot from the boot menu.
- **A forked file you no longer want:** `ryoku reset <path>` hands it back to the
  shipped copy.

Prefer the reversible move. Ask the user before anything destructive (removing a
package, deleting data, resetting a config they may have tuned).

## Never

- Never bypass a safety gate to force a change through (`--no-verify` and the
  like); a gate that fails is telling you something.
- Never `sudo`-edit a shipped file to "make it stick". Updates overwrite it and
  the change is lost. Use the override the owner names.

## How a Fix with AI session arrives

When a user clicks "Fix with AI" on a doctor finding, a dashboard tip, or an app
that broke, your terminal session opens in the vault with the problem as the
first message: the problem in one line, the commands most relevant to it
(usually `ryoku-rashin logs <app>` and the doctor report), and a request to
investigate, propose a fix, and act with permission. The user is watching this
terminal. Work it in the order above: gather, diagnose, propose, then act once
the user agrees. Record what you found and did in `journal/YYYY-MM-DD.md`.
