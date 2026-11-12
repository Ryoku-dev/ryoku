.pragma library

var title = "Add-ons";
var lead = "Add-ons and bundles from RyoStore that are on this computer: turn them on, choose where they show, change their options, update or remove them.";

var sections = [
    { id: "installed", title: "Installed add-ons" },
    { id: "bundles", title: "Bundles" },
    { id: "omarchy", title: "Omarchy add-ons" },
    { id: "store", title: "Browse RyoStore" }
];

function store(section) {
    return ["ryoku-host", "app", "--label", "ryostore", "--", "ryostore", "open", section];
}

// template rows feed search only; the page draws the installed items itself
var rows = [
    { id: "installed-list", section: "installed", label: "Installed add-ons", template: true,
      detail: "Shell add-ons on the bar, in pop-outs and on Quick settings",
      keywords: ["plugins", "extensions", "manage", "installed"] },
    { id: "add-on-on", section: "installed", label: "Turn an add-on on or off", template: true,
      detail: "Off keeps it installed without running it",
      keywords: ["enable", "disable", "enabled", "plugin"] },
    { id: "add-on-show-as", section: "installed", label: "Where an add-on shows", template: true,
      detail: "As a pop-out, on the bar or as a Quick settings card",
      keywords: ["show as", "host", "placement", "popout", "bar glyph", "sidebar card", "quick settings card"] },
    { id: "add-on-popout", section: "installed", label: "Where a pop-out opens", template: true,
      detail: "The side of the screen and the place along it, or the centre",
      keywords: ["edge", "align", "position", "frame popout", "placement", "centre", "center"] },
    { id: "add-on-card-order", section: "installed", label: "Order of Quick settings cards", template: true,
      detail: "Arranged on the Quick settings page",
      keywords: ["sidebar card", "order", "tab", "extension cards"] },
    { id: "add-on-options", section: "installed", label: "An add-on's own options", template: true,
      detail: "Each add-on brings its own settings",
      keywords: ["plugin settings", "preferences", "configure"] },
    { id: "add-on-update", section: "installed", label: "Update an add-on", template: true,
      detail: "When RyoStore has a newer version",
      keywords: ["upgrade", "new version", "plugin update"] },
    { id: "add-on-remove", section: "installed", label: "Remove an add-on", template: true,
      detail: "Deletes it and its settings",
      keywords: ["uninstall", "delete", "plugin remove"] },

    { id: "bundle-list", section: "bundles", label: "Bundles", template: true,
      detail: "Sets of apps and tools with parts on this computer",
      keywords: ["bundle", "packages", "components", "parts"] },
    { id: "bundle-install", section: "bundles", label: "Install a bundle's missing parts", template: true,
      detail: "Finishes in a terminal window",
      keywords: ["install missing", "complete", "packages"] },
    { id: "bundle-remove", section: "bundles", label: "Remove a bundle", template: true,
      detail: "Removes its installed parts in a terminal window",
      keywords: ["uninstall", "delete", "packages"] },

    { id: "omarchy-list", section: "omarchy", label: "Omarchy add-ons", template: true,
      detail: "Add-ons from the Omarchy market, while a bar style that uses them is in use",
      keywords: ["omarchy", "nomarchy", "market", "plugins", "enable", "disable", "remove"] },

    { id: "browse-add-ons", section: "store", label: "Find more add-ons",
      detail: "Add-ons for the bar, pop-outs and Quick settings",
      keywords: ["store", "ryostore", "download", "plugins", "install"],
      control: "handoff", command: store("plugins"), actionText: "Open" },
    { id: "browse-bundles", section: "store", label: "Find more bundles",
      detail: "Sets of apps and tools installed together",
      keywords: ["store", "ryostore", "download", "packages", "install"],
      control: "handoff", command: store("bundles"), actionText: "Open" },
    { id: "browse-omarchy", section: "store", label: "Find more Omarchy add-ons", omarchy: true,
      detail: "RyoStore lists them while a bar style that uses them is in use",
      keywords: ["omarchy", "market", "store"],
      control: "handoff", command: store("omarchy-plugins"), actionText: "Open" }
];

var related = ["quick-settings", "desktop-dock", "bar"];
