import assert from "node:assert/strict";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const Store = require("./store.js");

const eq = (got, want, message) => assert.deepEqual(got, want, message);
const items = [
    { id: "market", category: "plugins", name: "Market", summary: "Widget source", installed: true, enabled: true, updateAvailable: true, tags: ["shell"] },
    { id: "clock", category: "lockscreens", name: "Clockwork", summary: "Mechanical clock", installed: true, active: false, tags: ["time"] },
    { id: "bundle", category: "bundles", name: "Creator", installedCount: 2, totalCount: 4 },
    { id: "plain", category: "plugins", name: "Plain", installed: false }
];

const products = [
    { id: "hero", category: "rices", name: "Hero", art: "hero.jpg", installed: false },
    { id: "lock", category: "lockscreens", name: "Lock", art: "lock.jpg", installed: true },
    { id: "partial", category: "bundles", installedCount: 2, totalCount: 4 }
];

eq(Store.itemKey(products[0]), "rices:hero", "stable item key");
eq(Store.collection(products, { view: "discover", categoryID: "lockscreens" }).map(Store.itemKey), ["lockscreens:lock"], "category collection");
eq(Store.collection(products, { view: "library" }).map(Store.itemKey), ["lockscreens:lock", "bundles:partial"], "library collection");
eq(Store.collection(products, { view: "discover", query: "hero" }).map(Store.itemKey), ["rices:hero"], "search collection");
eq(Store.selectionKey(products, "lockscreens:lock", 0), "lockscreens:lock", "preserve valid selection");
eq(Store.selectionKey(products, "missing:item", 1), "lockscreens:lock", "fallback by bounded index");
eq(Store.selectionKey([], "missing:item", 0), "", "empty selection");

const activeUpdate = { active: true, installed: true, updateAvailable: true };
eq(Store.statusLabels(activeUpdate), ["UPDATE", "ACTIVE"], "update and active are both explicit");
eq(Store.statusLabels(items[2]), ["2 / 4 INSTALLED"], "partial bundle is explicit");
eq(Store.statusLabels(items[3]), ["AVAILABLE"], "available state is explicit");

eq(
    Store.filter(items, { category: "plugins", installedOnly: true, query: "market" }).map(item => item.id),
    ["market"],
    "combined filter"
);
eq(Store.groupSearch(items, "installed clock").map(item => item.id), ["clock"], "state words participate in search");

const schemes = [
    { id: "hancore-kanso", category: "colorschemes", name: "Kanso", installed: false, metadata: { provider: "HANCORE-linux" }, tags: ["dark"] },
    { id: "noctalia-adw", category: "colorschemes", name: "ADW", installed: true, metadata: { provider: "Noctalia" }, tags: ["dark", "light"] },
    { id: "hancore-demon", category: "colorschemes", name: "Demon", installed: true, metadata: { provider: "HANCORE-linux" }, tags: ["dark"] }
];
eq(Store.filter(schemes, { category: "colorschemes", provider: "HANCORE-linux" }).map(i => i.id), ["hancore-kanso", "hancore-demon"], "provider filter narrows to one provider");
eq(Store.collection(schemes, { view: "discover", categoryID: "colorschemes", provider: "Noctalia" }).map(Store.itemKey), ["colorschemes:noctalia-adw"], "provider collection");
eq(Store.collection(schemes, { view: "discover", categoryID: "colorschemes", installedOnly: true }).map(i => i.id), ["noctalia-adw", "hancore-demon"], "my themes collection is installed-only");
eq(Store.filter(schemes, { category: "colorschemes", provider: "HANCORE-linux", installedOnly: true }).map(i => i.id), ["hancore-demon"], "provider and installed filter compose");

eq(
    Store.sortCategories([
        { id: "plugins", group: "extend" },
        { id: "rices", group: "wear" },
        { id: "installed", group: "find" },
        { id: "bundles", group: "extend" },
        { id: "locks", group: "wear" }
    ]).map(category => category.id),
    ["installed", "rices", "locks", "plugins", "bundles"],
    "category groups remain stable"
);

eq(
    Store.featured([
        { id: "broken", category: "rices", name: "Broken", art: "poster.png", sourceError: true },
        { id: "owned", category: "plugins", name: "Owned", art: "owned.png", installed: true },
        { id: "archive", category: "lockscreens", name: "Archive", art: "archive.png", installed: false }
    ]).id,
    "archive",
    "featured is deterministic and prefers available real art"
);
eq(Store.installed(items).map(item => item.id), ["market", "clock", "bundle"], "installed projection includes active, enabled, and partial state");
eq(
    Store.categoryPlates([{ id: "plugins", count: 12, installedCount: 3 }])[0],
    { id: "plugins", count: 12, installedCount: 3 },
    "category plates preserve backend counts"
);

eq(Store.primaryAction({ installed: false }), "INSTALL", "available action");
eq(Store.primaryAction({ installed: true }), "INSTALLED", "installed action");
eq(Store.primaryAction({ installed: true, updateAvailable: true }), "UPDATE", "installed update action");
eq(Store.primaryAction({ installed: false, busy: true }), "INSTALLING", "busy action");
eq(Store.secondaryAction({ installed: true, hasSettings: true }), "OPEN IN SETTINGS", "management handoff when the product has a settings page");
eq(Store.secondaryAction({ installed: true }), "", "installed product without a settings page has no handoff");
eq(Store.secondaryAction({ installed: false }), "", "available item has no management handoff");
eq(
    Store.filter(items, { query: "plugin update" }).map(item => item.id),
    ["market"],
    "multiword search preserves source order"
);

// A paused item stays listed but cannot be downloaded: every action surface
// reads it as under construction and shows the catalogue's reason, while the
// underlying install state stays visible so an installed copy is still removable.
const paused = { installed: false, downloadPaused: true, downloadPauseReason: "Has known issues. The developer is working on fixes." };
const pausedInstalled = { installed: true, active: true, downloadPaused: true, downloadPauseReason: "Broken on Wayland." };
eq(Store.isDownloadPaused(paused), true, "paused item is detected");
eq(Store.isDownloadPaused({ installed: false }), false, "unpaused item is not paused");
eq(Store.downloadPauseReason(paused), "Has known issues. The developer is working on fixes.", "pause reason is surfaced when paused");
eq(Store.downloadPauseReason({ downloadPauseReason: "stale" }), "", "reason is empty unless the item is paused");
eq(Store.primaryAction(paused), "UNDER CONSTRUCTION", "paused item cannot be installed");
eq(Store.primaryAction({ installed: true, busy: true, downloadPaused: true }), "UNDER CONSTRUCTION", "pause overrides every other action state");
eq(Store.statusLabels(paused), ["UNDER CONSTRUCTION"], "paused uninstalled item drops the contradictory AVAILABLE label");
eq(Store.statusLabels(pausedInstalled), ["UNDER CONSTRUCTION", "ACTIVE"], "paused installed item keeps its state so remove stays reachable");

// A product written for another window manager is greyed out, never offered: its
// chip and its disabled action both name the window manager it wants, taken from
// the product rather than from a list in the UI. The backend refuses the install
// as well, so this is presentation only; an installed copy stays removable.
const foreign = { installed: false, unavailable: true, requiredWindowManager: "wm-a", unavailableReason: "Built for wm-a." };
const foreignInstalled = { installed: true, active: true, unavailable: true, requiredWindowManager: "wm-a", unavailableReason: "Built for wm-a." };
eq(Store.isUnavailable(foreign), true, "a product for another window manager is detected");
eq(Store.isUnavailable({ installed: false }), false, "an ordinary product is not unavailable");
eq(Store.unavailableLabel(foreign), "WM-A ONLY", "the chip names the window manager the product wants");
eq(Store.unavailableLabel({ unavailable: true }), "UNAVAILABLE", "a product that names none still reads as unavailable");
eq(Store.unavailableReason(foreign), "Built for wm-a.", "the catalogue's reason is surfaced");
eq(Store.unavailableReason({ unavailableReason: "stale" }), "", "reason is empty unless the product is unavailable");
eq(Store.primaryAction(foreign), "WM-A ONLY", "the disabled action names the window manager too");
eq(Store.statusLabels(foreign), ["WM-A ONLY"], "unavailable uninstalled item drops the contradictory AVAILABLE label");
eq(Store.statusLabels(foreignInstalled), ["WM-A ONLY", "ACTIVE"], "unavailable installed item keeps its state so remove stays reachable");

// Discover rotates on a daily seed: stable within a day, varies across days, and
// never drops or invents an item. No seed keeps the legacy deterministic order.
const disc = [
    { id: "a", category: "rices", name: "A", art: "a.jpg", installed: false },
    { id: "b", category: "rices", name: "B", art: "b.jpg", installed: false },
    { id: "c", category: "rices", name: "C", art: "c.jpg", installed: false },
    { id: "d", category: "rices", name: "D", art: "d.jpg", installed: false },
    { id: "e", category: "rices", name: "E", art: "e.jpg", installed: false },
    { id: "f", category: "rices", name: "F", art: "f.jpg", installed: false }
];
const discKeys = disc.map(Store.itemKey);
const order = seed => Store.collection(disc, { view: "discover", seed: seed }).map(Store.itemKey);
eq(order(0), discKeys, "no seed keeps the deterministic order");
eq(order(20260101), order(20260101), "the same day is stable");
eq(order(20260101).slice().sort(), discKeys.slice().sort(), "rotation preserves the full set");
assert.ok([1, 2, 3, 4, 5].map(order).some(o => JSON.stringify(o) !== JSON.stringify(order(1))), "order changes across days");
assert.ok(order(20260101).every(k => discKeys.includes(k)), "rotation invents no items");
eq(Store.featured(disc, 20260101).id, Store.featured(disc, 20260101).id, "featured is stable per seed");
assert.ok(discKeys.includes(Store.itemKey(Store.featured(disc, 20260101))), "seeded hero is a real eligible item");
const discSnap = JSON.stringify(disc);
Store.shuffleSeeded(disc, 7);
eq(JSON.stringify(disc), discSnap, "shuffleSeeded does not mutate its input");
eq(Store.shuffleSeeded(discKeys, 7), Store.shuffleSeeded(discKeys, 7), "shuffleSeeded is stable per seed");

const snapshot = JSON.stringify(items);
Store.filter(items, { query: "installed" });
Store.groupSearch(items, "clock");
eq(JSON.stringify(items), snapshot, "helpers do not mutate source arrays");

// Plugin classification drives the Plugins subtab strip: a plugin is a BAR plugin
// when its manifest hosts include topbarGlyph, otherwise it is a DESKTOP plugin.
const plugins = [
    { id: "vpn", category: "plugins", name: "VPN", metadata: { hosts: ["topbarGlyph"] } },
    { id: "clockbar", category: "plugins", name: "Clock", metadata: { hosts: ["topbarGlyph", "desktopWidget"] } },
    { id: "widget", category: "plugins", name: "Widget", metadata: { hosts: ["desktopWidget"] } },
    { id: "popout", category: "plugins", name: "Popout", metadata: { hosts: ["framePopout"] } },
    { id: "bare", category: "plugins", name: "Bare" }
];
eq(Store.pluginKind(plugins[0]), "bar", "topbarGlyph host is a bar plugin");
eq(Store.pluginKind(plugins[1]), "bar", "any topbarGlyph host is a bar plugin");
eq(Store.pluginKind(plugins[2]), "desktop", "desktopWidget host is a desktop plugin");
eq(Store.pluginKind(plugins[3]), "desktop", "framePopout host is a desktop plugin");
eq(Store.pluginKind(plugins[4]), "desktop", "no hosts falls back to desktop");
eq(Store.pluginKind(undefined), "desktop", "missing item is a desktop plugin");
eq(
    Store.filter(plugins, { category: "plugins", pluginKind: "bar" }).map(item => item.id),
    ["vpn", "clockbar"],
    "BAR subtab keeps only topbarGlyph plugins"
);
eq(
    Store.filter(plugins, { category: "plugins", pluginKind: "desktop" }).map(item => item.id),
    ["widget", "popout", "bare"],
    "DESKTOP subtab keeps everything else"
);
eq(
    Store.collection(plugins, { view: "discover", categoryID: "plugins", pluginKind: "bar" }).map(Store.itemKey),
    ["plugins:vpn", "plugins:clockbar"],
    "plugins BAR collection filters through the pluginKind option"
);
eq(
    Store.collection(plugins, { view: "discover", categoryID: "plugins" }).map(item => item.id),
    ["vpn", "clockbar", "widget", "popout", "bare"],
    "plugins ALL collection keeps every plugin in source order"
);

const omarchyPlugins = [
    { id: "clock", category: "omarchy-plugins", name: "Clock", stars: 4, listedAt: "2026-10-01T00:00:00Z", verificationStatus: "verified", kind: "Bar widget", license: "MIT", metadata: { marketplaceCategory: "Widgets" } },
    { id: "notes", category: "omarchy-plugins", name: "Notes", stars: 20, listedAt: "2026-09-01T00:00:00Z", verificationStatus: "unverified", kind: "Panel", license: "GPL-3.0", metadata: { marketplaceCategory: "Productivity" } },
    { id: "weather", category: "omarchy-plugins", name: "Weather", stars: 8, listedAt: "2026-10-06T00:00:00Z", verificationStatus: "verified", kind: "Bar widget", license: "MIT", metadata: { marketplaceCategory: "Widgets" } }
];
eq(
    Store.collection(omarchyPlugins, { view: "discover", categoryID: "omarchy-plugins", omarchySort: "popular" }).map(item => item.id),
    ["notes", "weather", "clock"],
    "Omarchy Popular sorts by stars"
);
eq(
    Store.collection(omarchyPlugins, { view: "discover", categoryID: "omarchy-plugins", omarchySort: "new" }).map(item => item.id),
    ["weather", "clock", "notes"],
    "Omarchy New sorts by listing date"
);
eq(
    Store.collection(omarchyPlugins, { view: "discover", categoryID: "omarchy-plugins", omarchySort: "verified" }).map(item => item.id),
    ["weather", "clock", "notes"],
    "Omarchy Verified places verified entries first and uses popularity within the tier"
);
eq(Store.filter(omarchyPlugins, { query: "widgets bar mit" }).map(item => item.id), ["clock", "weather"], "Omarchy metadata participates in search");

console.log("RYOSTORE-STORE-HELPERS-PASS");
