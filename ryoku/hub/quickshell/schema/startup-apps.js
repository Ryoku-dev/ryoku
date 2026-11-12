.pragma library

var title = "Startup apps";
var lead = "Apps and commands that start each time you sign in, on top of what Ryoku starts itself. Changes take effect at your next sign-in.";

var sections = [
    { id: "apps", title: "Start at sign-in", missing: "This window manager cannot start apps at sign-in." },
    { id: "startup", title: "When the desktop starts" }
];

var related = ["default-apps", "terminal", "shortcuts"];

// the lists are drawn by the page; these rows let search reach them
var rows = [
    { id: "startup-list", section: "apps", label: "Start an app at sign-in", template: true,
      detail: "Add, change or remove the apps and commands that start when you sign in",
      keywords: ["autostart", "startup", "login", "exec", "exec-once", "spawn-at-startup", "run at login", "launch", "tray"] },
    { id: "remove-all", section: "apps", label: "Remove all startup apps", template: true, more: true,
      detail: "Clear the whole list at once", keywords: ["clear", "reset", "autostart"] },
    { id: "remove-all-variables", section: "apps", label: "Remove all session variables", template: true, more: true,
      detail: "Clear every variable at once", keywords: ["clear", "reset", "environment", "env"] },
    { id: "session-variables", section: "apps", label: "Session variables", template: true, more: true,
      detail: "Names and values exported to every app in your session",
      keywords: ["environment", "env", "variable", "export", "path", "moz_enable_wayland", "qt_qpa_platform", "shell"] }
];
