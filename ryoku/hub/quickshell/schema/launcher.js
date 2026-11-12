.pragma library

var title = "App launcher";
var lead = "The launcher Super+Space opens: its style, how it looks, what it lists and the greeting at its top.";

var sections = [
    { id: "style", title: "Launcher style" },
    { id: "look", title: "Look" },
    { id: "results", title: "Results" },
    { id: "greeting", title: "Greeting" }
];

function l(key) {
    return { store: "launcher", key: key };
}

// `offers` names the capability a launcher must declare for the row to show;
// template rows feed search only, the page draws them
var rows = [
    { id: "launcher-style", section: "style", label: "Launcher style", template: true,
      detail: "Choose the launcher Super+Space opens",
      keywords: ["variant", "hero", "main", "okshell", "command palette", "theme", "look", "install", "update"] },
    { id: "browse-launchers", section: "style", label: "Find more launchers",
      detail: "Browse and install launchers in RyoStore",
      keywords: ["store", "download", "ryostore"], control: "handoff",
      command: ["ryostore", "open", "launchers"], actionText: "Open" },

    { id: "corner-radius", section: "look", offers: "shape", label: "Corner radius",
      detail: "How rounded the launcher's corners are; the cards inside follow",
      keywords: ["rounding", "radius", "shape"], control: "stepper", min: 0, max: 28, unit: "px",
      bind: l("radius"), "default": 16 },
    { id: "backdrop-blur", section: "look", offers: "background", label: "Blur behind the results",
      detail: "Frosts the desktop behind the results; 0 leaves it sharp",
      keywords: ["frost", "blur", "background", "glass", "bgBlur"], control: "stepper", min: 0, max: 30, unit: "px",
      bind: l("bgBlur"), "default": 2 },
    { id: "backdrop-opacity", section: "look", offers: "overlay", label: "Backdrop",
      detail: "How solid the dimmed backdrop behind the launcher is",
      keywords: ["overlay", "opacity", "dim", "transparency"], control: "slider", min: 0.3, max: 1, step: 0.01, pct: true,
      bind: l("overlayOpacity"), "default": 0.94 },

    { id: "hidden-apps", section: "results", offers: "hiddenApps", label: "Hidden apps", template: true,
      detail: "Apps the launcher leaves out; they stay installed",
      keywords: ["hide", "exclude", "blacklist", "remove from launcher"] },
    { id: "favourites", section: "results", offers: "pinnedApps", label: "Favourites", template: true,
      detail: "Apps listed first in every section and in search",
      keywords: ["favorites", "pinned", "pin", "star", "starred"] },
    { id: "sections", section: "results", offers: "sections", label: "Sections", template: true,
      detail: "The order of the launcher's sections, and which show",
      keywords: ["categories", "groups", "chips", "order", "tabs"] },
    { id: "columns", section: "results", offers: "grid", label: "Apps per row",
      detail: "How many apps each row of the grid holds; fewer fit on a narrow screen",
      keywords: ["columns", "grid"], control: "stepper", min: 3, max: 16,
      bind: l("columns"), "default": 8 },
    { id: "tile-width", section: "results", offers: "grid", label: "Tile width",
      detail: "How wide each app tile is",
      keywords: ["grid", "size", "matWidth"], control: "stepper", min: 90, max: 240, unit: "px",
      bind: l("matWidth"), "default": 144 },
    { id: "settle", section: "results", offers: "results", label: "Pause before results",
      detail: "How long the launcher waits after you stop typing before the results fade in",
      keywords: ["type settle", "delay", "debounce", "motion"], control: "stepper", min: 120, max: 700, unit: "ms",
      bind: l("resultSettleMs"), "default": 360, more: true },

    { id: "show-greeting", section: "greeting", offers: "hero", label: "Greeting",
      detail: "Good morning, good evening and so on, above the clock",
      keywords: ["hello", "time of day", "hero"], control: "toggle",
      bind: l("showGreeting"), "default": true },
    { id: "show-weather", section: "greeting", offers: "hero", label: "Weather",
      detail: "The weather and temperature at the top; off shows the date",
      keywords: ["forecast", "temperature", "hero"], control: "toggle",
      bind: l("showWeather"), "default": true },
    { id: "weather-units", section: "greeting", offers: "hero", label: "Weather units",
      detail: "Celsius or Fahrenheit is set once for the whole desktop",
      keywords: ["celsius", "fahrenheit", "temperature", "units"], control: "link",
      link: "time-language/units", actionText: "Time & language" },
    { id: "day-night", section: "greeting", offers: "hero", label: "Day and night",
      detail: "A line under the clock that follows the sun through the day",
      keywords: ["solar line", "horizon", "sun", "moon", "line"], control: "segmented",
      options: [{ label: "Palette", value: "auto" }, { label: "One colour", value: "fixed" }, { label: "Off", value: "off" }],
      bind: l("horizonMode"), "default": "auto" },
    { id: "day-night-colour", section: "greeting", offers: "hero", label: "Line colour",
      detail: "The colour of the line and the sun",
      keywords: ["solar line colour", "horizon colour", "color"], control: "color",
      bind: l("horizonColor"), "default": "#ffc777", when: { bind: l("horizonMode"), is: ["fixed"] } },
    { id: "day-night-palette", section: "greeting", offers: "hero", label: "Edit colours",
      detail: "Day and night takes its colours from your palette",
      keywords: ["palette", "colours", "colors", "ryogami"], control: "handoff",
      command: ["ryogami", "wallpaper", "settings", "theme"], actionText: "Open",
      when: { bind: l("horizonMode"), is: ["auto"] } },
    { id: "picture", section: "greeting", offers: "hero", label: "Picture",
      detail: "A wide picture behind the top of the launcher; empty uses Ryoku's own",
      keywords: ["hero image", "banner", "background", "image", "photo"], control: "path", kind: "file", asUrl: true,
      placeholder: "Ryoku's own picture", nameFilters: ["Pictures (*.png *.jpg *.jpeg *.webp *.bmp)", "All files (*)"],
      changeLabel: "Launcher picture", bind: l("heroImage"), "default": "" },
    { id: "more-pictures", section: "greeting", offers: "hero", label: "Find more pictures",
      detail: "Launcher pictures from RyoStore land in your Pictures folder",
      keywords: ["store", "launcher images", "ryostore", "banner"], control: "handoff",
      command: ["ryostore", "open", "launcher-images"], actionText: "Open", more: true },
    { id: "picture-strength", section: "greeting", offers: "hero", label: "Picture strength",
      detail: "How strongly the picture shows; 0% hides it",
      keywords: ["hero strength", "opacity", "visibility"], control: "slider", min: 0, max: 1, step: 0.01, pct: true,
      bind: l("heroStrength"), "default": 0.6 },
    { id: "picture-across", section: "greeting", offers: "hero", label: "Picture focus across",
      detail: "Which part of the picture stays in view, left to right; you can also drag the preview",
      keywords: ["focal point", "crop", "reframe", "position"], control: "slider", min: 0, max: 1, step: 0.01, pct: true,
      bind: l("heroPosX"), "default": 0.5, more: true },
    { id: "picture-down", section: "greeting", offers: "hero", label: "Picture focus down",
      detail: "Which part of the picture stays in view, top to bottom; you can also drag the preview",
      keywords: ["focal point", "crop", "reframe", "position"], control: "slider", min: 0, max: 1, step: 0.01, pct: true,
      bind: l("heroPosY"), "default": 0.5, more: true }
];

var related = ["bar", "time-language", "shortcuts", "default-apps"];
