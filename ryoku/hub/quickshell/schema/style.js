.pragma library

var title = "Style";
var lead = "How every Ryoku surface looks: its design, its colours, and light or dark.";

var sections = [
    { id: "design", title: "Design" },
    { id: "colours", title: "Colours" },
    { id: "scheme", title: "Colour scheme" }
];

var designs = [
    { value: "lithic", label: "Lithic",
      detail: "Ryoku's own: cut corners, panes drawn in hairlines, one colour for the action" },
    { value: "material", label: "Material",
      detail: "Material 3: pill buttons and switches, large soft cards on tonal surfaces" },
    { value: "adwaita", label: "Adwaita",
      detail: "GNOME's look: rounded cards and lists, soft buttons, pill switches" },
    { value: "fluent", label: "Fluent",
      detail: "The Windows 11 look: small corners and layered, outlined surfaces" }
];

function s(key, fallback) {
    return { store: "settings", key: key, fallback: fallback };
}

var rows = [
    // the page draws the choice as tiles over a live preview; this row is how search finds it
    { id: "design", section: "design", label: "Design", template: true,
      detail: "Lithic, Material, Adwaita or Fluent, across Settings and the desktop",
      keywords: ["design", "design language", "style", "look", "theme", "material", "material you", "m3",
                 "lithic", "adwaita", "gnome", "libadwaita", "fluent", "windows 11", "rounded", "corners", "shape"],
      control: "custom", bind: s("theme.design", "lithic"), "default": "lithic",
      options: designs.map(d => ({ value: d.value, label: d.label })) },

    { id: "colours", section: "colours", label: "Colours",
      detail: "Ryoku's own colours, or the desktop's from your wallpaper or a colour scheme",
      keywords: ["colours", "colors", "palette", "theme", "follow wallpaper", "wallpaper colours", "ryoku colours", "accent"],
      control: "segmented", bind: s("theme.colours", "ryoku"), "default": "ryoku",
      options: [{ value: "ryoku", label: "Ryoku" }, { value: "wallpaper", label: "Wallpaper" }] },
    { id: "mode", section: "colours", label: "Light or dark",
      detail: "For the whole desktop. Automatic follows the wallpaper's brightness, Sun the time of day",
      keywords: ["dark mode", "light mode", "dark theme", "light theme", "white", "black", "night", "day", "brightness"],
      control: "segmented", "default": "smart",
      bind: { store: "command", get: ["ryogami", "settings", "get", "theme.mode"],
              set: ["ryogami", "settings", "set", "theme.mode", "{value}"] },
      options: [{ value: "dark", label: "Dark" }, { value: "light", label: "Light" },
                { value: "smart", label: "Automatic" }, { value: "sun", label: "Sun" }] },
    { id: "contrast", section: "colours", label: "Contrast",
      detail: "High makes text and the lines between things stronger",
      keywords: ["contrast", "readability", "accessibility", "high contrast"],
      control: "segmented", bind: s("theme.contrast", "standard"), "default": "standard",
      options: [{ value: "standard", label: "Standard" }, { value: "high", label: "High" }] },
    // only Lithic tones its surfaces from the wallpaper; the page drops the row elsewhere
    { id: "tint", section: "colours", label: "Tint",
      detail: "How much of the wallpaper's hue Ryoku's surfaces take",
      keywords: ["tint", "hue", "tone", "colourful"],
      control: "segmented", bind: s("theme.tint", "off"), "default": "off",
      options: [{ value: "off", label: "Off" }, { value: "low", label: "Low" }, { value: "medium", label: "Medium" }],
      when: { bind: s("theme.colours", "ryoku"), is: ["wallpaper"] } },

    { id: "colour-scheme", section: "scheme", label: "Change the colour scheme",
      detail: "Your wallpaper's colours or a fixed scheme, set with the wallpaper",
      keywords: ["palette", "scheme", "matugen", "material you", "tonal", "vibrant", "named theme"],
      control: "handoff", command: ["ryogami", "wallpaper", "settings", "colours"] }
];

var related = ["wallpaper-colours", "window-style", "fonts-icons"];
