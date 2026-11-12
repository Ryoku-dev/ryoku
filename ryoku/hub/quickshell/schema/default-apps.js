.pragma library

var title = "Default apps";
var lead = "The app each job opens when you press its key or pick it in the launcher. Files and links still open in the app set for their type.";

var sections = [
    { id: "apps", title: "Apps", missing: "This window manager cannot choose these apps." },
    { id: "keys", title: "Keys that open them" }
];

var related = ["shortcuts", "terminal", "launcher"];

// roles in the order the page shows them; `id` is the role ryoku-app reads
var roles = [
    { id: "browser", label: "Browser", detail: "Opens web pages" },
    { id: "terminal", label: "Terminal", detail: "Runs commands" },
    { id: "editor", label: "Editor", detail: "Edits text and code" },
    { id: "files", label: "File manager", detail: "Browses your files" },
    { id: "notes", label: "Notes", detail: "Keeps your notes" }
];

// the page draws these rows itself; listed so search reaches each one
var rows = [
    { id: "browser", section: "apps", label: "Browser", detail: "Opens web pages", template: true,
      keywords: ["default browser", "web", "internet", "firefox", "chromium", "zen", "brave", "open with"] },
    { id: "terminal", section: "apps", label: "Terminal", detail: "Runs commands", template: true,
      keywords: ["default terminal", "console", "kitty", "alacritty", "foot", "ghostty", "wezterm", "super enter"] },
    { id: "editor", section: "apps", label: "Editor", detail: "Edits text and code", template: true,
      keywords: ["default editor", "text editor", "code", "neovim", "nvim", "vs code", "zed", "helix"] },
    { id: "files", section: "apps", label: "File manager", detail: "Browses your files", template: true,
      keywords: ["default file manager", "files", "folders", "nautilus", "thunar", "dolphin", "yazi"] },
    { id: "notes", section: "apps", label: "Notes", detail: "Keeps your notes", template: true,
      keywords: ["notes app", "notebook", "obsidian", "logseq", "joplin"] },

    { id: "browser-key", section: "keys", label: "Browser key", detail: "The keys that open your browser", template: true,
      keywords: ["shortcut", "keybind", "hotkey", "super b", "rebind"] },
    { id: "terminal-key", section: "keys", label: "Terminal key", detail: "The keys that open your terminal", template: true,
      keywords: ["shortcut", "keybind", "hotkey", "super enter", "super return", "rebind"] },
    { id: "editor-key", section: "keys", label: "Editor key", detail: "The keys that open your editor", template: true,
      keywords: ["shortcut", "keybind", "hotkey", "super n", "rebind"] },
    { id: "files-key", section: "keys", label: "File manager key", detail: "The keys that open your file manager", template: true,
      keywords: ["shortcut", "keybind", "hotkey", "super e", "rebind"] },
    { id: "notes-key", section: "keys", label: "Notes key", detail: "The keys that open your notes", template: true,
      keywords: ["shortcut", "keybind", "hotkey", "super o", "rebind"] }
];
