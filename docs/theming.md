# Theming: how colour reaches an app

Ryoku *generates* a palette; Omarchy *ships* one. That difference is smaller than
it looks, and the part worth copying is not the palette at all -- it is how the
palette reaches the applications.

## How Ryoku works today

One palette, rendered outward.

    wallpaper --matugen--> colors.json ------> Quickshell singletons (Scheme)
                              |
                              +--> matugen templates --> ~/.config/<app>/...

`ryoku/shell/ipc/matugen.go` runs matugen against the current wallpaper, writes
`~/.cache/ryoku/colors.json`, and renders the templates in
`ryoku/shell/matugen/` into each app's config. A fixed named theme skips
generation and renders the catalog palette through the same templates.

The core desktop and app suite both use this path. Kitty, btop and the Qt
palette render every time; GTK and the opted-in application templates render
behind the app-suite switch.

Ryotunes, the native music client, joins the suite too: matugen renders its
Material 3 palette as a Ryoku skin at `~/.config/ryotunes/skins/matugen/skin.json`,
which the client loads as its "System" theme.

### Terminal readouts and prompts

Fastfetch and Starship keep layout separate from generated colour.

- Matugen writes `~/.config/fastfetch/ryoku-colors.json`. In **Follow
  wallpaper** mode, `ryoku-fastfetch` asks the Hub backend for a cached effective
  config with the generated title, label, section, rule and percentage colours
  overlaid. `config.jsonc` remains the user's file and is never rewritten by a
  wallpaper change. **Fixed** mode reads it directly.
- Matugen writes `~/.config/starship/ryoku-colors.toml`. The selected preset
  stays under `~/.config/starship/layouts/`, while
  `~/.config/starship.toml` is the active, atomically composed config. Fixed mode
  copies the preset exactly. Follow-wallpaper mode swaps its named palette for
  the generated sibling; Matugen's post-hook refreshes the active config after
  each render.

The modes and Starship layout id live under `~/.config/ryoku/`, so package
updates can refresh the catalogue and templates without replacing the user's
choice.

### KDE apps and the optional platform theme

KDE apps (Dolphin, Ark, Gwenview, Kate) never read the qt6ct palette; they
resolve their colours through KColorScheme and `~/.config/kdeglobals`. The `kde`
matugen template renders the palette into KDE's colour groups, and the daemon
merges them into `kdeglobals` on every repaint, claiming only the colour groups
so a user's fonts, icon theme and widget style there survive.

Those colours are only consulted when the KDE platform theme is active, and
Ryoku keeps `QT_QPA_PLATFORMTHEME=qt6ct` (see `hyprland/modules/env.lua`). qt6ct
is what ships: it hands every plain Qt app the palette, the Papirus icons and the
`qt6ct.conf` font. The `kde` platform theme needs `plasma-integration`, which
Ryoku does not depend on, so switching to it out of the box would strip plain Qt
apps back to Qt's defaults. To opt in, install `plasma-integration` and set
`QT_QPA_PLATFORMTHEME=kde` (through the user_edits overlay or your own
environment); the kdeglobals colours are already in place, so KDE apps follow the
wallpaper the moment the theme is active.

## How Omarchy works

A theme is a *folder* of per-app files. One directory always holds the active
one, at a path that never changes:

    ~/.config/omarchy/themes/<name>/    colors.toml, btop.theme, neovim.lua, ...
    ~/.config/omarchy/current/theme/    the active one, rebuilt in place

It is a real directory, not a symlink. `omarchy-theme-set` builds `next-theme/`
-- copy the official theme, copy the user's same-named theme over it, render
templates into the gaps -- then `rm -rf current/theme && mv next-theme
current/theme`. A half-written theme is never visible.

The stable *path* is what the design rests on. Because `current/theme/btop.theme`
means the same thing forever, an app is wired to it exactly once and never
touched again. Three ways, depending on what the app allows:

- an `@import` / `source` / `include` line in a shipped config (hyprland,
  hyprlock, kitty, alacritty, ghostty, foot, waybar, walker, swayosd)
- a permanent symlink made once at install (btop, mako, helix, neovim)
- a copy or an API call, for apps that will not read a foreign file (vscode,
  chromium, GNOME, obsidian)

Generation and hand-authoring meet in one rule. `omarchy-theme-set-templates`
compiles `colors.toml` into sed substitutions and renders `default/themed/*.tpl`,
but **skips any output file that already exists**. The theme's own files were
copied in first, so a theme shipping its own `waybar.css` suppresses
`waybar.css.tpl` entirely. Per file, hand-authored beats generated.

Colours are never derived from the wallpaper -- there is no matugen, pywal or
wallust anywhere in the tree. Light vs dark is a marker file, `light.mode`.
Omarchy 4 is moving the other way, templating `neovim.lua` and `vscode.json` so
themes shrink toward `colors.toml` plus previews: converging on the generated
model Ryoku already has.

## What is worth adopting

**The stable path, not the palette.** Ryoku renders straight into each app's
final destination. That has two costs:

- `cava -> ~/.config/cava/config` (`ryoku/shell/matugen/apps.toml`) still renders
  the app's *whole* config, not a colour fragment, so a user's own cava settings
  are overwritten on the next wallpaper change -- cava has no include directive to
  split the palette out. ghostty took that split: matugen writes only the palette
  to `~/.config/ghostty/ryoku-colors`, and the shipped `config` pulls it in with
  `config-file = ryoku-colors`, so the user's `~/.config/ghostty/config` stands.
- A theme has no single location. There is nothing to point at, export, or
  install -- which is exactly why a third-party theme folder cannot be supported
  today.

Rendering into one theme directory and pointing app configs at it fixes both,
and makes a generated palette and a downloaded theme interchangeable.

**One manifest.** Omarchy's app set is a directory listing. Ryoku's is spread
across `ryoku/shell/matugen/config.toml`, `ryoku/shell/matugen/apps.toml`,
`templateGroup()` in `ryoku/shell/ipc/matugen.go`, and a second group switch plus
a default roster in `ryoku/hub/backend/matugen.go`. Four places must agree about
what "gtk" means. One manifest -- app, template, destination, how it is included,
how it reloads -- would be the single source the renderer, the roster and the Hub
all read.

**Not** the hand-authoring. Ryoku's whole point is that the palette follows the
wallpaper; Omarchy themes are fixed. The catalog of 57 named palettes already
covers the fixed case.

## Coverage gap

Apps Omarchy themes that Ryoku does not, most visible first:

| App | Note |
|---|---|
| Browser (chromium / brave) | Omarchy sets it through a managed policy file |
| Neovim | `neovim.lua` in the theme dir |
| VS Code / Codium | written into the app's settings, not a config file |
| Fish shell + fzf | `colors.fish`, `fzf.fish` |
| hyprlock | Ryoku has qylock, which takes no palette yet |
| superfile | file manager |

Omarchy also themes waybar, walker, mako and swayosd. Ryoku replaces all four
with its own surfaces, which already read the palette directly -- no gap.

Ryoku covers what Omarchy does not: telegram, heroic, obs, micro, qt5ct/qt6ct.

## Supporting a third-party theme folder (later)

The **palette** half of this already ships. RyoStore's Themes category imports a
theme's colours (its `colors.toml`, pre-converted into Ryoku's palette shape and
hosted under `ryostore/colorschemes`) into a named Ryoku colour scheme, so a
HANCORE theme shows in the Color-scheme picker and matugen fans it into every
app. The rest of this section is the larger, still-future part: using a theme's
own hand-authored per-app files verbatim.

A repo like `HANCORE-linux/omarchy-harbordark-theme` is a flat directory of
per-app files plus `backgrounds/`. Once app configs read from a stable theme
directory, installing one is: clone it into a themes dir, rebuild the active
directory from it, reload.

Omarchy's skip-if-exists rule is the piece worth copying wholesale, because it
makes generated and downloaded themes the same kind of thing: copy the theme's
own files in first, then render Ryoku's matugen templates only into the gaps. A
theme that ships `btop.theme` keeps its own; one that ships only `colors.toml`
gets everything generated. Ryoku's 57-palette catalog and a downloaded folder
then stop being separate mechanisms.

What must land first is the stable directory and the manifest above. The install
step is small after that.
