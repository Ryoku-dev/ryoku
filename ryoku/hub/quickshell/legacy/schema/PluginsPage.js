.pragma library

// PluginsPage as data: the Hyprland compositor plugins Ryoku bundles, one tab
// each, paired with the status and rebuild card the page draws for it. The tab
// titles are the names `ryoku-hub desktop plugins list` reports. The settings
// rows now live with the provider (ryoku-wm-hyprland schema, page "plugins"), so
// this file only names which bundled plugins get a card.
//
// A plugin the user adds from git gets its rows from the settings the backend
// detects in its .so; those are built by the page, not listed here.

var plugins = [
    { "id": "hyprbars", "tab": "Title bars" },
    { "id": "hyprglass", "tab": "Glass" },
    { "id": "imgborders", "tab": "Image borders" },
    { "id": "dynamic-cursors", "tab": "Cursor motion" },
    { "id": "hyprfocus", "tab": "Focus flash" },
    { "id": "keysounds", "tab": "Key sounds" },
    { "id": "borders-plus-plus", "tab": "Extra borders" },
    { "id": "hyprgrass", "tab": "Touch gestures" }
];
