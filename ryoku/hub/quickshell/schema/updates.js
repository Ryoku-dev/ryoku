.pragma library

var title = "Updates";
var lead = "Keep Ryoku and the rest of your system up to date, see what each update brings, and how to go back.";

var sections = [
    { id: "status", title: "Status" },
    { id: "whats-new", title: "What's new" },
    { id: "system", title: "Your system" },
    { id: "checks", title: "Automatic checks" },
    { id: "rollback", title: "Rolling back" }
];

function store(key, fallback) {
    return { store: "updates", key: key, fallback: fallback };
}

// template rows feed search only; the page draws them itself
var rows = [
    { id: "update-now", section: "status", label: "Update now", template: true,
      detail: "Install the waiting update and follow it step by step",
      keywords: ["upgrade", "install", "ryoku update", "run", "log", "details", "stop", "password"] },
    { id: "check-now", section: "status", label: "Check for updates", template: true,
      detail: "Look for a new version now",
      keywords: ["check again", "refresh", "look for updates", "ryoku status"] },
    { id: "version", section: "status", label: "Installed version", template: true,
      detail: "The Ryoku release this computer runs and the one it follows",
      keywords: ["release", "version", "codename", "build", "branch", "stable", "unstable"] },

    { id: "whats-new", section: "whats-new", label: "What's new", template: true,
      detail: "The changes a waiting update brings, or the latest ones already here",
      keywords: ["changelog", "release notes", "changes", "commits", "history", "recent changes"] },

    { id: "whole-system", section: "system", label: "Update the whole system too",
      detail: "Update now also installs your distribution's updates, kernel included",
      keywords: ["system packages", "full upgrade", "distribution", "kernel", "pacman", "dnf", "xbps", "--system", "include system"],
      control: "toggle", bind: store("system", false), "default": false },
    { id: "system-packages", section: "system", label: "Waiting system updates", template: true,
      detail: "Packages your distribution has ready, kernel included",
      keywords: ["packages", "kernel", "pacman -Syu", "distribution updates"] },

    { id: "auto-check", section: "checks", label: "Look for updates",
      detail: "How often Ryoku checks in the background. Off means only when you check here.",
      keywords: ["automatic", "schedule", "interval", "hourly", "daily", "weekly", "background check", "notify"],
      control: "segmented", options: [
          { label: "Off", value: "off" }, { label: "Hourly", value: "hourly" },
          { label: "Daily", value: "daily" }, { label: "Weekly", value: "weekly" }
      ], bind: store("interval", "daily"), "default": "daily" },

    { id: "snapshots", section: "rollback", label: "Snapshots", template: true,
      detail: "A copy of the whole system saved before each update, restored from the boot menu",
      keywords: ["rollback", "roll back", "undo update", "restore", "snapper", "boot menu", "limine", "btrfs"] },
    { id: "ways-back", section: "rollback", label: "See the ways back",
      detail: "Lists the releases and snapshots you can return to, in a terminal. Nothing changes until you choose one.",
      keywords: ["rollback", "ryoku rollback", "older release", "downgrade", "revert"],
      control: "action", actionText: "Show" }
];

var related = ["about", "backup-import", "rashin"];
