.pragma library

var title = "Fonts & icons";
var lead = "The typefaces and text size apps use, the icon set, the pointer, and the desktop's Japanese glosses.";

var sections = [
    { id: "fonts", title: "Fonts" },
    { id: "icons", title: "Icons" },
    { id: "pointer", title: "Pointer" },
    { id: "glosses", title: "Japanese glosses" }
];

function s(key, fallback) {
    return { store: "settings", key: key, fallback: fallback };
}

function d(key) {
    return { store: "desktop", key: key };
}

var rows = [
    { id: "interface-font", section: "fonts", label: "Interface font",
      detail: "The typeface apps and the desktop use for their text",
      keywords: ["font", "typeface", "system font", "ui font", "family", "gtk font", "qt font"],
      control: "custom", bind: s("fontFamily", "Space Grotesk"), "default": "Space Grotesk" },
    { id: "monospace-font", section: "fonts", label: "Monospace font",
      detail: "The fixed-width typeface for terminals and code",
      keywords: ["terminal font", "mono", "code font", "fixed width", "kitty", "nerd font"],
      control: "custom", bind: s("terminalFontFamily", "JetBrainsMono Nerd Font"), "default": "JetBrainsMono Nerd Font" },
    { id: "text-size", section: "fonts", label: "Text size in apps",
      detail: "The base size of app and terminal text",
      keywords: ["font size", "point size", "bigger text", "smaller text", "readability"],
      control: "stepper", min: 8, max: 24, unit: "pt",
      bind: s("fontSize", 11), "default": 11 },
    { id: "desktop-size", section: "fonts", label: "Desktop size",
      detail: "How large the desktop's own bars, panels and text are, set with your displays",
      keywords: ["interface scale", "ui scale", "shell size", "bigger"],
      control: "link", link: "displays/desktop-size", needs: { cap: "monitorConfig" } },

    { id: "icon-theme", section: "icons", label: "Icon theme",
      detail: "The icon set apps and the desktop draw",
      keywords: ["icons", "icon pack", "papirus", "breeze", "app icons", "folder icons"],
      control: "custom", bind: { store: "fonts-icons", key: "iconTheme" } },

    { id: "pointer-theme", section: "pointer", label: "Pointer style",
      detail: "The look of the mouse pointer; it can take the wallpaper's colours",
      keywords: ["cursor", "cursor theme", "mouse pointer", "bibata", "arrow"],
      control: "custom", bind: d("cursor.theme") },
    { id: "pointer-edit-colours", section: "pointer", label: "Edit colours",
      detail: "The pointer follows the wallpaper palette, set with the wallpaper",
      keywords: ["cursor colour", "palette", "accent", "matugen"],
      control: "handoff", command: ["ryogami", "wallpaper", "settings", "theme"],
      when: { bind: d("cursor.theme"), is: ["DYNAMIC"] } },
    { id: "pointer-size", section: "pointer", label: "Pointer size",
      detail: "How large the pointer is drawn",
      keywords: ["cursor size", "bigger pointer", "accessibility"],
      control: "stepper", min: 12, max: 64, unit: "px",
      bind: d("cursor.size") },

    { id: "glosses", section: "glosses", label: "Show Japanese glosses",
      detail: "Japanese words and marks that decorate the desktop. Off keeps Latin marks; your language and fonts stay as they are.",
      keywords: ["kanji", "japanese", "gloss", "decorative", "cjk", "brand mark"],
      control: "toggle", bind: s("showJapaneseGloss", true), "default": true }
];

var related = ["displays", "mouse-touchpad", "terminal", "wallpaper-colours"];
