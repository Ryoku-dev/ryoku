.pragma library

var title = "Backup & import";
var lead = "Bring another desktop's configuration onto Ryoku, save your setup to carry it elsewhere, or start over with Ryoku's own settings.";

var sections = [
    { id: "import", title: "Import a configuration",
      missing: "No reader for other desktops' configuration is installed. A window manager that can import one adds it." },
    { id: "setup", title: "Your setup" },
    { id: "reset", title: "Start over" }
];

// rows whose control the page draws; the import steps follow the import section
var rows = [
    { id: "import-detected", section: "import", label: "Found on this computer",
      detail: "", keywords: ["detected", "existing config", "dotfiles", "migrate", "scan"],
      // gone from the page and from search once the import store has looked and found nothing
      control: "custom", needs: { unless: { bind: { store: "import", key: "found" }, is: [false] } } },
    { id: "import-folder", section: "import", label: "From a folder",
      detail: "A whole .config folder, one app's folder, or a dotfiles checkout. You can also drop a folder on this page.",
      keywords: ["dotfiles", "config folder", "migrate", "drop", "hyprland.conf", "kitty", "fish", "fastfetch", "keybinds"],
      control: "custom" },
    { id: "import-git", section: "import", label: "From a git repository",
      detail: "Copied to a temporary folder and looked through; nothing changes until you apply",
      keywords: ["git", "url", "clone", "github", "repository", "dotfiles"],
      control: "custom" },
    { id: "import-steps", section: "import", label: "Review, conflicts and preview", template: true,
      detail: "Choose what comes over, settle shortcuts that clash, then apply; every file it touches is backed up and the import can be undone",
      keywords: ["conflicts", "keybind clash", "undo import", "backup", "layer", "remap"] },

    { id: "export-setup", section: "setup", label: "Export your setup",
      detail: "Saves your desktop's look and settings to a folder you can keep or take to another computer. Your location, language and displays stay here.",
      keywords: ["export", "backup", "save", "rice", "share", "copy settings", "transfer"],
      control: "custom", activatable: true },
    { id: "import-setup", section: "setup", label: "Use a saved setup",
      detail: "Open a setup someone exported, or one of yours; your current setup is kept so you can put it back",
      keywords: ["restore", "import", "rice", "load setup", "transfer"],
      control: "custom", activatable: true },

    { id: "reset-all", section: "reset", label: "Reset all settings",
      detail: "Puts the desktop's and the window manager's settings back to how Ryoku ships them. Your wallpaper and colours, language, formats, location and displays stay.",
      keywords: ["reset", "factory", "defaults", "start over", "reset everything", "restore defaults"],
      control: "custom", activatable: true }
];

var related = ["shortcuts", "wallpaper-colours", "updates"];
