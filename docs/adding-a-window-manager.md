# Adding a window manager

So you want Ryoku to run on a compositor it has never met. Good news: that is a
supported thing to do, and it is meant to take one directory rather than a fork.
This is the walkthrough, written by someone who has just done it, including the
parts that went wrong.

If you only want the contract in table form, read `docs/compositors.md`. This
page is the human version: what to do, in what order, and what will bite you.

## What you are actually signing up for

You write one program, `ryoku-wm-<name>`, that answers ten questions about your
compositor. Everything else in Ryoku talks to that program and never learns
which compositor is running. The shell, the bar, the dock, the launcher, the
lockscreen and every settings page are already compositor-neutral. You do not
touch them. If you find yourself editing the shell to make your compositor work,
stop: something is missing from the provider, and that is where it belongs.

Realistically this is a weekend for a compositor with a decent IPC, and most of
that weekend is spent learning your compositor's config language rather than
learning Ryoku.

## Before you start, check it fits

Ryoku needs a Wayland compositor that gives you:

- **layer-shell**, or the bar, dock and wallpaper have nowhere to live.
- **foreign-toplevel**, or the dock and the window list are empty.
- **ext-workspace**, if it has workspaces at all, or the workspace pills go blank.
- **session-lock**, or the lockscreen cannot cover the screen securely.
- **screencopy**, for screenshots and screen recording.
- **some way to talk to it while it runs**: a socket, a CLI, a D-Bus name. You
  need to ask what is focused and tell it to close a window.
- **a config file you can generate**, because that is how settings are applied.

Missing one of the first five is survivable and shows up as a capability that
reports false. Missing the last two is not: without them there is nothing for a
provider to do.

## Step 1: the provider

Make `ryoku/wm/<name>/`, a `package main` in Go. Copy the shape of whichever
existing provider is closer to yours, and do not invent a second shape:
`ryoku/wm/niri/` if your compositor has a socket and a declarative config,
`ryoku/wm/hyprland/` if it has a CLI and a scripted config.

The ten verbs, in plain terms:

|Verb|What it means|
|---|---|
|`caps`|"Here is what I can do." A manifest, printed as JSON.|
|`state`|"Here is everything right now": outputs, workspaces, windows, keyboard.|
|`watch`|The same, streamed, one JSON object per line, forever.|
|`act <id>`|"Do this one thing": close a window, focus a workspace, exit the session.|
|`apply <store>`|"Turn these settings into my config file."|
|`defaults`|"Here is my baseline for those settings."|
|`schema`|"Here are the settings only I have, so the Hub can draw them."|
|`binds <store>`|"Here are the keybinds only I have, so the cheatsheet can list them under my name."|
|`outputs <file>`|"Arrange the displays like this."|
|`session`|The `wayland-session` desktop entry, so a greeter can offer you.|

Two of these carry most of the weight. `watch` is the one the desktop leans on
hardest: the bar, the dock and the overview are all downstream of it, so it has
to be cheap and it has to never lie. `apply` is the one users feel: it is the
difference between a settings page that works and a settings page that is
decoration.

Write a test that pins the exact bytes you send your compositor. This sounds
fussy and it is the single most valuable test in the provider. A wrong action
name usually fails **silently**: the compositor ignores you, the desktop looks
fine, and nothing works. We shipped a dialect bug exactly once and a pinning
test caught it on the second compositor.

## Step 2: register the name

`ryoku/wm/detect.go` is the only file in the whole repo allowed to know your
compositor exists. Add four things:

- `Provider<Name>`, the name string. This is on-disk contract: it names your
  binary (`ryoku-wm-<name>`), your settings namespace (`wm.<name>.*`) and your
  package (`ryoku-desktop-<name>`). Choose it once.
- The environment handle your session exports, and how to turn it into a socket
  path that can be dialled.
- Your config directory under `~/.config`.
- Your seed files: the per-machine files that get written once and then belong to
  the machine or the user.

## Step 3: the config payload

Make `ryoku/<name>/`, holding the config your package ships. This is authored in
**your compositor's own language**, not in a format Ryoku invented. Hyprland's
lives in Lua because Hyprland reads Lua; niri's is KDL because niri reads KDL.
One concern per file, and the top-level file is Ryoku's, sourcing the rest.

Keep a clean split between what you ship and what the machine owns. Shipped
files are yours and get replaced on update. Seeds are written once and then left
alone, because a user's display pins and their hand edits must survive an
upgrade.

## Step 4: your settings page, for free

This is the part that surprises people. You do not write a settings page.

Your provider prints its exclusive settings as rows from `schema`, and the Hub
renders them on the Window Manager page using the same renderer every other
settings page uses. A row looks like this:

```json
{
  "tab": "Layout", "group": "STRUTS",
  "key": "wm.niri.struts.left",
  "label": "Left strut",
  "desc": "Empty space niri keeps clear at the left screen edge, in pixels",
  "ctl": "step", "lo": 0, "hi": 256, "unit": "px",
  "src": "desktop.json", "page": "windowmanager"
}
```

`ctl` is the shared control vocabulary: a switch, a stepper, a slider, a colour,
a text field, a segmented picker. Pick from what exists rather than adding a
control type, and your page looks like the rest of Ryoku because it literally is
the rest of Ryoku. Compositors cannot drift apart visually, because there is
one renderer.

Write real labels and descriptions. Terse, concrete, no marketing. A user should
be able to read the description and know what changes.

## Step 5: ship it

`release/packages/ryoku-desktop-<name>/`, mirroring the existing compositor
package. It provides the compositor virtual so only one can be installed at a
time, and depends on your compositor plus whatever it needs for X11 apps and
screencasting.

Then offer it in the two installers, which both build their compositor question
from `wm.Providers()`, so this is usually one line each.

## Capabilities: the part worth getting right

A capability is a promise. If `caps` says you support something, a control
appears in the Hub and a keybind is bound. If you claim something you cannot do,
the user gets a control that silently does nothing, which is worse than not
having the feature at all.

The rule is simple: **claim it only if `act` or `apply` actually honours it.**

When your compositor cannot do something, say so twice, in two different places:

- In `caps`, so the control never renders.
- In `apply`'s unhonored list, with a short human reason, so if a setting does
  arrive it is reported rather than dropped. Those reasons are shown to the user
  verbatim, both when previewing a compositor switch and on the Window Manager
  page under "what this compositor cannot do". Write them for a person: "niri has
  no scratchpad workspace" beats "unsupported".

Name capabilities for **behaviour**, never for a compositor. `outputMirror` is
right; `niriSupportsX` is wrong and the build will reject it.

## Things that will bite you

Every one of these cost real time on the second compositor.

**Probe the live thing, do not trust the docs.** We were about to spawn a helper
that the compositor had integrated natively two releases earlier, which would
have broken X11 apps through a socket race. A thirty second experiment on a
running instance settles questions that an afternoon of reading does not.

**A missing include can cost the user their session.** niri treats an absent
included file as a fatal config error, so every file the top-level config names
has to exist on a deployed box, including the generated ones when they would be
empty. Find out what your compositor does with a missing or broken config
**before** you ship, and know whether it has an emergency mode.

**Know your override order.** We measured it rather than assumed: in niri the
last include wins, which makes the include list an override chain and puts the
user's own file last on purpose. Get this backwards and user edits silently lose.

**Check whether you can remove a binding.** niri has no unbind, and a duplicate
chord is a hard config error, so the generated keybind block has to be total:
defaults, plus rebinds, minus removals, with one winner per chord. If your
compositor is the same, resolve collisions in the generator, because a duplicate
does not degrade, it takes down the whole config.

**Never trust an environment handle to mean a live session.** A terminal or a
service that outlived its compositor still carries the old handle, and on a
switch every action from it goes to a dead socket and reports success. Dial the
socket. Checking that the path exists is not enough: some compositors leave the
directory behind.

**Know who draws the cursor.** A client that uses the modern cursor-shape
protocol only names a shape, and the **compositor** renders it from its own
theme; an older client uploads its own pixels. That difference is invisible
until one surface has no cursor and everything else is fine. Also worth knowing
early: screenshot tools do not capture the cursor at all, so you cannot debug
this from a screenshot.

**Your compositor may not report window positions.** If it does not, say so in
`caps`, and the overview and screenshot picker stand aside instead of drawing
every window on top of each other at the origin.

## Testing without wrecking your desktop

Run your compositor **nested** inside your current session. Both of ours can do
it, and it gives you a real socket, a real config load and a real client to poke
at, with no risk to the session you are working in. A nested instance is also
where you test the lockscreen, because a locker that crashes in a nested
compositor is an inconvenience rather than a lockout.

Validate your generated config with your compositor's own validator on every
run if it has one. It is the cheapest test you will ever write.

Then the gates, which are not optional:

```
bin/ryoku-dev-verify-wm-isolation    no compositor names leak outside the seam
bin/ryoku-dev-verify-delivery        every config reaches users somehow
bin/ryoku-dev-lint-qml <root>        the QML roots you touched
go build ./... && go vet ./... && go test ./...   in every module you touched
```

The isolation gate is the one that keeps this architecture honest. It fails the
build if a compositor name appears in a conditional anywhere outside the seam and
its own payload. When it fires, the fix is almost never to add an exception: it
is that something wants to be a capability.

## You are done when

- `ryoku wm status` names your compositor and lists its capabilities.
- `ryoku wm use <name>` previews the switch, honestly listing what will not carry
  over, and the Hub offers the same flow.
- Logging in through the greeter brings up the full desktop: bar, dock,
  wallpaper, launcher, lockscreen.
- The Hub shows a Window Manager page with your settings on it, and nothing
  anywhere in the Hub that your compositor cannot do.

That last one is the real test. Ryoku's rule is that a control is shown only when
something will write it, so a correct provider produces a Hub that fits your
compositor exactly, without anyone having edited the Hub.
