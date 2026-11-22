.pragma library

var title = "Clipboard";
var lead = "How the clipboard history opens and how long it keeps what you copy.";

var sections = [
    { id: "panel", title: "The panel" },
    { id: "history", title: "History" }
];

function s(key, fallback) {
    return { store: "settings", key: key, fallback: fallback };
}

// template rows feed search; the page draws them itself
var rows = [
    { id: "open-clipboard", section: "panel", label: "Open the clipboard",
      detail: "Everything you copied, to search, star and copy again. Super+V opens it too",
      keywords: ["open", "show", "super v", "history", "paste"], control: "action",
      command: ["ryoku-shell", "clipboard"], actionText: "Open" },

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
