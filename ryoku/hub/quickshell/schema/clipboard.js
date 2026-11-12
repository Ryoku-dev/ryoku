.pragma library

var title = "Clipboard";
var lead = "The clipboard panel's size and corners, and how long its history keeps what you copy.";

var sections = [
    { id: "panel", title: "Panel size" },
    { id: "corners", title: "Corners" },
    { id: "history", title: "History" }
];

function s(key, fallback) {
    return { store: "settings", key: key, fallback: fallback };
}

// template rows feed search; the page draws them itself
var rows = [
    { id: "panel-width", section: "panel", label: "Width",
      detail: "How much of the screen's width the panel spans",
      keywords: ["panel width", "size", "wide"], control: "stepper", min: 40, max: 90, unit: "%",
      bind: s("clipboard.widthPercent", 65), "default": 65 },
    { id: "panel-height", section: "panel", label: "Height",
      detail: "The most of the screen's height the panel takes",
      keywords: ["panel height", "size", "tall"], control: "stepper", min: 24, max: 72, unit: "%",
      bind: s("clipboard.heightPercent", 42), "default": 42 },
    { id: "panel-bottom", section: "panel", label: "Distance from the bottom",
      detail: "How far above the screen's bottom edge the panel rests; 0 sits it on the edge",
      keywords: ["bottom offset", "margin", "position", "raise"], control: "stepper", min: 0, max: 20, unit: "%",
      bind: s("clipboard.bottomPercent", 0), "default": 0 },

    { id: "panel-corners", section: "corners", label: "Panel corners",
      detail: "How rounded the panel's outer corners are",
      keywords: ["rounding", "radius", "panel rounding"], control: "stepper", min: 0, max: 32, unit: "px",
      bind: s("clipboard.panelRadius", 18), "default": 18 },
    { id: "pane-corners", section: "corners", label: "Pane corners",
      detail: "How rounded the Clipboard and Starred panes inside it are",
      keywords: ["rounding", "radius", "pane rounding"], control: "stepper", min: 0, max: 32, unit: "px",
      bind: s("clipboard.paneRadius", 12), "default": 12 },
    { id: "item-corners", section: "corners", label: "Item corners",
      detail: "How rounded the card of each copied item is",
      keywords: ["rounding", "radius", "card rounding", "cards"], control: "stepper", min: 0, max: 32, unit: "px",
      bind: s("clipboard.cardRadius", 9), "default": 9 },

    { id: "history-size", section: "history", label: "History size", template: true,
      detail: "How many items the history holds and the space they take",
      keywords: ["storage", "space", "disk", "items", "megabytes"] },
    { id: "clear-now", section: "history", label: "Clear history now", template: true,
      detail: "Removes everything you have not starred",
      keywords: ["prune", "delete", "wipe", "empty", "erase", "clear"] },
    { id: "clear-weekly", section: "history", label: "Clear every week", template: true,
      detail: "Once a week, remove everything you have not starred",
      keywords: ["prune weekly", "auto clear", "automatic", "schedule"],
      control: "toggle", bind: s("clipboard.pruneWeekly", false), "default": false }
];

var related = ["shortcuts", "desktop-dock"];

function row(id) {
    for (var i = 0; i < rows.length; i++)
        if (rows[i].id === id)
            return rows[i];
    return null;
}
