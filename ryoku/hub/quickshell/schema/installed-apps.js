.pragma library

var title = "Installed apps";
var lead = "Every app you can open from app search, with the space it takes. Remove what you no longer use, or keep an app and leave it out of app search. RyoStore is where you get more.";

var sections = [
    { id: "ryoku", title: "Ryoku apps" },
    { id: "space", title: "Space" },
    { id: "apps", title: "Apps" },
    { id: "more", title: "Get more apps" }
];

// RyoStore at one of its views; RowControl and the page start it as an app of its own
function store(route) {
    return ["ryostore", "open", route];
}

// template rows feed search only; the page draws the apps and the chart itself
var rows = [
    { id: "ryoku-apps", section: "ryoku", label: "Ryoku apps", template: true,
      detail: "Ryotunes, Ryoport, Ryomanager and RyoStore",
      keywords: ["ryotunes", "ryoport", "ryomanager", "ryostore", "music", "own apps", "default apps"] },
    { id: "ryoku-app-update", section: "ryoku", label: "Update a Ryoku app", template: true,
      detail: "When a newer version is ready",
      keywords: ["upgrade", "new version", "ryotunes 2", "preview", "prerelease"] },

    { id: "space-overview", section: "space", label: "Space apps take", template: true,
      detail: "By where they came from and by size",
      keywords: ["disk", "storage", "size", "space", "graph", "chart", "large apps", "free space", "usage"] },

    { id: "app-list", section: "apps", label: "Every installed app", template: true,
      detail: "With its size and where it came from",
      keywords: ["programs", "applications", "packages", "software", "gpk", "list", "manage"] },
    { id: "sort-apps", section: "apps", label: "Sort apps", template: true,
      detail: "Largest first, or by name",
      keywords: ["order", "biggest", "alphabetical", "name", "size"] },
    { id: "group-by-size", section: "apps", label: "Group apps by size", template: true,
      detail: "Over 1 GB, 100 MB to 1 GB, 10 to 100 MB, under 10 MB",
      keywords: ["big", "small", "categories", "size groups"] },
    { id: "filter-apps", section: "apps", label: "Show apps from one source", template: true,
      detail: "System packages, AUR, Flatpak or this account",
      keywords: ["filter", "flatpak", "aur", "pacman", "dnf", "xbps", "flathub", "local", "user apps"] },
    { id: "show-in-search", section: "apps", label: "Show an app in app search", template: true,
      detail: "Off keeps it installed and out of the launcher",
      keywords: ["hide", "hidden apps", "launcher", "app search", "unhide", "exclude"] },
    { id: "remove-app", section: "apps", label: "Remove an app", template: true,
      detail: "Asks for your password once in each Settings session",
      keywords: ["uninstall", "delete", "free space", "purge", "get rid of"] },

    { id: "get-apps", section: "more", label: "Get more apps",
      detail: "Flathub, system packages and the AUR, in one search in RyoStore",
      keywords: ["install", "store", "ryostore", "download", "flathub", "software", "add apps", "new apps"],
      control: "handoff", command: store("apps"), actionText: "Open" },
    { id: "get-ryoku-apps", section: "more", label: "Get Ryoku's own apps",
      detail: "Ryotunes, Ryoport and Ryomanager in RyoStore",
      keywords: ["install", "ryotunes", "ryoport", "ryomanager", "store"],
      control: "handoff", command: store("ryoku-apps"), actionText: "Open" }
];

var related = ["default-apps", "launcher", "add-ons"];
