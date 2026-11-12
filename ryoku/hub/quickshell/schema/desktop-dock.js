.pragma library

var title = "Desktop & dock";
var lead = "What sits on your desktop and on which displays, the dock, and what shows while the desktop reloads.";

var sections = [
    { id: "edit", title: "Edit the desktop" },
    { id: "widgets", title: "Show widgets on" },
    { id: "dock", title: "Dock" },
    { id: "reload", title: "While the desktop reloads" }
];

function editor(section) {
    return ["ryoku-shell", "ipc", "call", "desktop", "editSection", section, ""];
}

function s(key, fallback) {
    return { store: "settings", key: key, fallback: fallback };
}

// template rows feed search; the page draws them itself or builds them per display
var rows = [
    { id: "edit-widgets", section: "edit", label: "Widgets",
      detail: "Add, place and arrange widgets on the desktop",
      keywords: ["stage editor", "desktop widgets", "clock", "weather", "arrange", "edit mode"],
      control: "handoff", command: editor("widgets"), actionText: "Edit" },
    { id: "edit-desktop-apps", section: "edit", label: "Apps on the desktop",
      detail: "Put apps on the desktop, or in folders there",
      keywords: ["shortcuts", "launchers", "folders", "desktop apps"],
      control: "handoff", command: editor("widgets/desktopApps"), actionText: "Edit" },
    { id: "edit-desktop-icons", section: "edit", label: "Desktop icons",
      detail: "Size, spacing, labels and marks of the icons on the desktop",
      keywords: ["icon size", "grid", "labels", "badges", "icons"],
      control: "handoff", command: editor("widgets/desktopIcons"), actionText: "Edit" },
    { id: "edit-wallpaper", section: "edit", label: "Wallpaper framing",
      detail: "How the wallpaper sits on each display while you arrange the desktop",
      keywords: ["wallpaper position", "fit", "crop", "screens", "background"],
      control: "handoff", command: editor("wallpaper"), actionText: "Edit" },
    { id: "edit-visualizer", section: "edit", label: "Visualizer",
      detail: "The music visualizer on the desktop: its look, place, colour and motion",
      keywords: ["audio visualizer", "music", "bars", "cava", "spectrum"],
      control: "handoff", command: editor("visualizer"), actionText: "Edit" },
    { id: "edit-depth", section: "edit", label: "Depth",
      detail: "Cut the wallpaper into layers so widgets can sit behind its subject",
      keywords: ["layers", "parallax", "3d", "cutout", "subject", "depth effect"],
      control: "handoff", command: editor("depth"), actionText: "Edit" },

    { id: "show-widgets", section: "widgets", label: "Show widgets on a display", template: true,
      detail: "Turn desktop widgets off on any connected display",
      keywords: ["monitor", "screen", "per display", "hide widgets", "second monitor", "external"] },

    { id: "dock-show", section: "dock", label: "Show the dock",
      detail: "A dock of your pinned and open apps along an edge of the screen",
      keywords: ["dock", "taskbar", "launcher bar", "pinned apps", "enable dock"],
      control: "toggle", bind: s("dock.enabled", false), "default": false },
    { id: "dock-design", section: "dock", label: "Dock design",
      detail: "Which dock draws when the dock is on",
      keywords: ["dock style", "dock look", "ii", "ryoku dock", "store docks"],
      control: "picker", options: [
          { label: "Ryoku", value: "ryoku" },
          { label: "ii", value: "ii" }
      ], bind: s("dock.design", "ryoku"), "default": "ryoku" },
    { id: "dock-edit", section: "dock", label: "Edit the dock",
      detail: "Edge, size, apps and the design's own options, in the desktop editor",
      keywords: ["dock edge", "dock size", "autohide", "magnify", "pinned apps", "stage editor"],
      control: "handoff", command: editor("dock"), actionText: "Edit" },
    { id: "dock-store", section: "dock", label: "Find more docks",
      detail: "Docks made by the community, in RyoStore",
      keywords: ["store", "download", "docks"], control: "handoff",
      command: ["ryostore", "open", "docks"], actionText: "Open", more: true },

    { id: "reload-cover", section: "reload", label: "Your own picture or video", template: true,
      detail: "Shown on every display while the desktop restarts, instead of the Ryoku mark",
      keywords: ["reload cover", "restart", "splash", "loading screen", "animation", "brand"] },
    { id: "reload-cover-file", section: "reload", label: "Picture or video", template: true,
      detail: "The file shown while the desktop reloads",
      keywords: ["reload cover", "image", "video", "gif", "mp4", "webm", "file"] }
];

var related = ["bar", "quick-settings", "displays", "wallpaper-colours"];

function row(id) {
    for (var i = 0; i < rows.length; i++)
        if (rows[i].id === id)
            return rows[i];
    return null;
}
