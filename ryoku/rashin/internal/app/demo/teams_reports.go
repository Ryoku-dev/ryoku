package demo

import (
	"ryoku-i18n"
)

// what the Repo watch reviewer handed back from this morning's run, written for the maintainer who reads it over coffee
func reviewReport(day string) string {
	return i18n.Tf(`I read the 23 pull requests and 41 issues opened or updated in Ryoku-dev since the last review. Nine of them need a maintainer; everything else waits on its author or on CI.

## Pull requests

1. [#412](https://github.com/Ryoku-dev/ryoku/pull/412) **Dock: keep the dock on the focused display.** Two approvals and CI is green; only a maintainer can merge it.
2. [#415](https://github.com/Ryoku-dev/ryoku/pull/415) **Hub: give the Fedora edition the Flatpak toggle.** CI fails only on the Void lane; you decide whether Void is in scope before the author spends more time on it.
3. [#418](https://github.com/Ryoku-dev/ryoku/pull/418) **Update the vendored Limine packages.** It changes the boot path, so someone has to try it on a real install before it merges.
4. [#421](https://github.com/Ryoku-dev/ryoku/pull/421) **Orpheus: lower idle CPU while music is paused.** It conflicts with last night's bar style change on unstable-dev and the author asked for help with the rebase.
5. [#423](https://github.com/Ryoku-dev/ryoku/pull/423) **Translate the Hub into Portuguese (Brazil).** 1,340 strings; it needs a native speaker you trust to review it.
6. [#426](https://github.com/Ryoku-dev/ryoku/pull/426) **ryoku update: retry a mirror once on a server error.** A first-time contributor touching the publish scripts; a maintainer has to approve the workflow run.

## Issues

1. [#1193](https://github.com/Ryoku-dev/ryoku/issues/1193) **A polkit rule lets any wheel user run ryoku-host svc.** A security report in public; move it to a private advisory today.
2. [#1187](https://github.com/Ryoku-dev/ryoku/issues/1187) **Super key dead after the 0.9.3 update on CachyOS.** Four reports in two days, all on the same update, so it looks like a regression rather than a setup problem.
3. [#1190](https://github.com/Ryoku-dev/ryoku/issues/1190) **Wallpaper goes grey after login on a hybrid NVIDIA laptop.** The same as #1142, which you closed last week; reopen that one or point the reporter to it.

## Nothing needed from you

- 17 pull requests wait on their authors or on CI.
- 38 issues are answered, waiting on the reporter, or duplicates. The Triager grouped them in the file below.

Read through the GitHub API with GITHUB_TOKEN. Nothing was commented, labelled or merged.

Grouped issues: reviews/%s.md`, day)
}

// the run before, when the user told the reviewer not to comment
func previousReviewReport() string {
	return i18n.T(`Two pull requests need you; the other eleven updated since the last review wait on their authors.

1. [#409](https://github.com/Ryoku-dev/ryoku/pull/409) **Hub: show the update channel on the About page.** It is approved but 31 commits behind unstable-dev. I wanted to ask the author to rebase; you said not to comment, so the ask is yours if you want it.
2. [#410](https://github.com/Ryoku-dev/ryoku/pull/410) **Installer: offer zram on machines with 8 GB or less.** It changes the default swap for new installs; that is a product decision, not a review.

No new issue needs a maintainer.`)
}

// how cron and systemd timers handle the runs they miss, the piece the researcher handed back to the editor
func cronReport() string {
	return i18n.T(`## Missed runs

- **cron** runs nothing it missed. If the computer is off at 09:00, the 09:00 job simply does not happen. Debian's cron adds no catch-up either.
- **anacron** exists for exactly this: it keeps a timestamp per job and runs daily, weekly and monthly jobs once after boot when their period has passed. It works in days, not minutes.
- **systemd timers** with ` + "`Persistent=true`" + ` remember the last trigger on disk and fire once at boot when a run was missed; without it they behave like cron.

## Overlapping runs

- cron starts a second copy happily; people wrap jobs in ` + "`flock -n`" + ` to stop that.
- A systemd timer never starts a service that is still running, so a slow run delays or skips the next one by design.

## Sources

- crontab(5) and anacron(8) manual pages.
- systemd.timer(5), the Persistent= and AccuracySec= sections.`)
}

// the research desk's history report, done and read two days ago
func waylandReport() string {
	return i18n.T(`Screen capture on Wayland went through three stages.

1. **No capture at all (2012 to 2017).** The protocol deliberately left screen access to the compositor, so every tool that read the screen under X stopped working.
2. **One protocol per desktop (2017 onwards).** Each desktop added its own screencopy protocol or D-Bus interface, so tools had to support every desktop separately.
3. **The ScreenCast portal (2018 onwards).** xdg-desktop-portal asks the user which screen or window to share and hands the app a PipeWire stream. Browsers, OBS and video calls use it today.

The ext-image-copy-capture protocol, merged in 2024, is the first capture protocol several compositors share; tools are starting to move to it.

The full timeline with links is in the file below.`)
}

// the mail desk's report after the approved mail went out
func thanksReport() string {
	return i18n.T(`Sent the thank-you mail to the print shop at orders@riso.example.pt, after you approved it.

It thanks them for the five merch samples, says the indigo print came out best, and asks for a quote on 200 stickers in that ink by the end of the month. I'll tell you when they answer.`)
}

// the caretaker's last evening check before the team was paused
func eveningReport() string {
	return i18n.T(`Nothing needs you tonight.

| Check | Result |
| --- | --- |
| Disk | / is 61% full, 182 GB free |
| Services | No failed service |
| Journal | 3 errors since yesterday, all the same Bluetooth retry |
| Updates | 14 packages wait, none of them the kernel |

The Bluetooth errors repeat every few minutes while the headset is off. They are harmless; turning the headset on stops them.`)
}

// the news digest's weekly report
func digestReport() string {
	return i18n.T(`## This week in Linux desktops

- **Mesa 26.0** fixes the stutter after waking from sleep that many of you reported on AMD laptops. ([mesa3d.org](https://www.mesa3d.org))
- **Fedora 44 beta** is out and drops the last 32-bit libraries from the default install. ([fedoramagazine.org](https://fedoramagazine.org))
- **PipeWire 1.6** lowers Bluetooth audio latency by a third on most headsets. ([gitlab.freedesktop.org](https://gitlab.freedesktop.org/pipewire/pipewire))
- **Arch Linux** moves its mirrors to a new signing key next month; run a full update before December. ([archlinux.org](https://archlinux.org/news/))

## From the topics you follow

- Two reviews of Ryoku's latest release, both positive about the Hub and both confused by the update channels.

The Mastodon posts you bookmark are not in this digest yet: the team asked you for a Mastodon token.`)
}
