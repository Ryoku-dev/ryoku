.pragma library

var title = "Window manager";
var lead = "The program that draws your windows: which one runs, switching to another, and what it can do.";

var sections = [
    { id: "current", title: "Which window manager" },
    { id: "cannot", title: "What this one cannot do" },
    { id: "plugins", title: "Plugins" },
    { id: "added", title: "Plugins you added" }
];

// the page draws these itself; they are listed so search reaches them
var rows = [
    { id: "wm-current", section: "current", label: "Window manager in use",
      detail: "The one running now, and the others this computer can use",
      keywords: ["compositor", "wm", "wayland", "session", "current", "installed"], template: true },
    { id: "wm-switch", section: "current", label: "Switch window manager",
      detail: "Change to another one; it starts at your next sign-in",
      keywords: ["change compositor", "switch compositor", "swap", "install", "session", "greeter", "log out"], template: true },
    { id: "wm-stay", section: "current", label: "Stay on the current window manager",
      detail: "Forget a switch you have not signed in to yet",
      keywords: ["cancel switch", "undo switch", "keep"], template: true },
    { id: "wm-cannot", section: "cannot", label: "What this window manager cannot do",
      detail: "Settings it has no equivalent for; they stay saved for the others",
      keywords: ["unsupported", "missing", "unhonored", "limits", "not available"], template: true },
    { id: "plugins-status", section: "plugins", label: "Plugin status",
      detail: "Which plugins run, and which need a rebuild after an update",
      keywords: ["plugins", "hyprpm", "abi", "version", "running", "loaded", "stale", "docs"], template: true,
      needs: { cap: "plugins" } },
    { id: "plugins-rebuild", section: "plugins", label: "Rebuild plugins",
      detail: "Build plugins again for the window manager's current version",
      keywords: ["rebuild", "build", "stale", "abi mismatch", "update", "compile"], template: true,
      needs: { cap: "plugins" } },
    { id: "plugins-add", section: "plugins", label: "Add a plugin from a repository",
      detail: "Build a plugin from its source and load it",
      keywords: ["add plugin", "git", "repository", "url", "hyprpm", "install plugin", "third party"], template: true,
      needs: { cap: "plugins" }, more: true },
    { id: "plugins-remove", section: "plugins", label: "Remove a plugin you added",
      detail: "Delete a plugin you built here",
      keywords: ["remove plugin", "uninstall plugin", "delete"], template: true,
      needs: { cap: "plugins" } },
    { id: "plugins-added", section: "added", label: "Settings of plugins you added",
      detail: "Turn each one on or off and set what it offers",
      keywords: ["plugin settings", "enable plugin", "config"], template: true,
      needs: { cap: "plugins" } }
];

// bundled plugins and the provider tab their settings rows carry
var bundled = {
    "hyprbars": "Title bars",
    "hyprglass": "Glass",
    "imgborders": "Image borders",
    "dynamic-cursors": "Cursor motion",
    "hyprfocus": "Focus flash",
    "keysounds": "Key sounds",
    "borders-plus-plus": "Extra borders",
    "hyprgrass": "Touch gestures"
};

var related = ["window-behaviour", "window-style", "shortcuts", "add-ons"];
