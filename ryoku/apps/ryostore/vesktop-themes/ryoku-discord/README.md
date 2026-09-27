# ryoku-discord
Ryoku Discord theme for Vesktop/Vencord with optional live Ryoku Palette Bridge colors and rendering improvements.

Based on [niqqudim/ryoku-discord](https://github.com/niqqudim/ryoku-discord), the original Ryoku theme by Ron. Original theme design and artwork credit belongs to the upstream project. This fork adds palette integration, animation changes, and compatibility documentation.

## Palette modes

Install `Ryoku.theme.css` in Vencord’s Themes folder and enable it in Settings → Vencord → Themes. Disable other full themes, including `midnight-ryoku.theme.css`, to avoid competing layout rules.

Open **Super+W → Settings → Ryoku Palette Bridge → App integrations → Vesktop** to control palette following. **SET UP** enables wallpaper colors; **REMOVE** (then **CONFIRM REMOVE**) turns palette following off and returns to the original red/gold palette. Ryoku remains enabled in Vesktop.

Keep Vencord QuickCSS enabled. [Ryoku Palette Bridge](https://github.com/Sipper1236/ryoku-palette-bridge) writes the palette and its enabled signal into QuickCSS, so wallpaper changes apply while Discord remains open. This requires the bridge integration version that emits `--ryo-bridge-enabled` and preserves an enabled Ryoku theme.

The default follows that menu. Advanced users can override the option at the top of the theme:

```css
:root {
    --ryo-palette-mode: var(--ryo-bridge-enabled, 0); /* follow the menu */
    /* Use 0 to always keep red/gold, or 1 to always consume available palette colors. */
}
```

Live mode renders each decorative illustration in one palette tone (multicolor plates become monochrome). Presence indicators use the bridge’s dedicated status colors. Missing bridge colors fall back to the original palette. The installed theme remains a single CSS file.

## Rendering

Unread indicators animate with a translation; completed message animations release their transform/filter effects. Reduced-motion settings suppress the supported theme animations. Permanent row `will-change` hints are removed. These changes remove unnecessary rendering work. A [live Vesktop check](PERFORMANCE.md) found stutters with and without the theme; it did not establish a measurable speedup.

## Original theme screenshots

These screenshots come from the upstream project and show its original appearance.

![Chatl](https://i.postimg.cc/4y8wMMHN/image.png)

![Settings](https://i.postimg.cc/C1T2HWkW/image.png)

![Friends](https://i.postimg.cc/VzFN5zmt/image.png)
