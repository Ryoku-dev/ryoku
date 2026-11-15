.pragma library

var title = "Style";
var lead = "The design every Ryoku surface draws in, and the colours the round designs wear.";

var sections = [
    { id: "design", title: "Design" },
    { id: "colours", title: "Colours" }
];

var designs = [
    { value: "lithic", label: "Lithic",
      detail: "Ryoku's own: cut corners, paper drawn in hairlines and one sun for the action" },
    { value: "material", label: "Material",
      detail: "Material 3: soft round shapes and pill controls in your desktop's colour scheme" },
    { value: "adwaita", label: "Adwaita",
      detail: "GNOME's look: calm rounded cards and lists in your desktop's colours" },
    { value: "fluent", label: "Fluent",
      detail: "The Windows 11 look: tight corners and layered surfaces in your desktop's colours" }
];

var rows = [
    // the page draws the choice as a gallery of the designs; this row is how search finds it
    { id: "design", section: "design", label: "Design", template: true,
      detail: "Lithic, Material, Adwaita or Fluent, across Settings and the desktop",
      keywords: ["design", "design language", "style", "look", "theme", "material", "material you", "m3",
                 "lithic", "adwaita", "gnome", "libadwaita", "fluent", "windows 11", "rounded", "corners", "shape"],
      control: "custom", bind: { store: "settings", key: "theme.design", fallback: "lithic" }, "default": "lithic",
      options: designs.map(d => ({ value: d.value, label: d.label })) },
    { id: "colour-scheme", section: "colours", label: "Colour scheme",
      detail: "Your wallpaper's colours or a fixed scheme, set with the wallpaper",
      keywords: ["palette", "colours", "colors", "scheme", "matugen", "accent", "material you", "tonal", "vibrant"],
      control: "handoff", command: ["ryogami", "wallpaper", "settings", "colours"] }
];

var related = ["wallpaper-colours", "window-style", "fonts-icons"];
