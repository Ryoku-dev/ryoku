# When something breaks

The user says an app or a part of the desktop stopped working: "the bar is gone",
"my wallpaper is grey", "Firefox won't launch anymore", "sound died after the
update". Do not start editing. Find out what happened first, then make the
smallest reversible fix through the tool that owns the thing.

## 1. Gather the evidence

The vault's `logs.md` gives the exact journal, Quickshell, update, doctor, and
crash commands for this machine. Read it first, then run the narrow command for
the affected subsystem. It remains available when Rashin is off.

What to look at, and why:

- **The journal.** `journalctl --user -u <unit> -e` for a Ryoku unit,
  `journalctl --user _COMM=<binary> -e` for a program. Warnings and errors near
  the time it broke are the story.
- **Coredumps.** `coredumpctl list` shows crashes; `coredumpctl info <binary>`
  heads the newest. A crash is not a config problem.
- **The doctor report** (`~/.local/state/ryoku/doctor-report.txt`) and the update
  log. The doctor already checks the session, the shell, and the wallpaper.
- **The native package log.** An upgrade right before a breakage is the classic
  cause. `logs.md` names the package history source for this edition.

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
- If the bar is gone or a different one came up, check which item is selected
  (desktop.md's "Bar and dock" section). A selected subshell or bar style that
  is not installed, or that failed to load, falls back to Orpheus; `ryoku doctor`
  installs a missing selected item when Ryostore offers it and otherwise moves
  the selection back to Orpheus, then restarts the shell on it.
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

Confirm the fix the same way you found the fault: rerun the command from
`logs.md`, watch the unit come up, or reproduce the user's action. A change that
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

When a user chooses Fix with AI on a doctor finding, Rashin opens an Agent
session with the problem and doctor report as context. Gather evidence from
`logs.md`, diagnose, propose, and act only with the user's permission. Record
what you found and did in `journal/YYYY-MM-DD.md`.
