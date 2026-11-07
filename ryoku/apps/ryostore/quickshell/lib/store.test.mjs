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
eq(Store.collection([
    { id: "z", category: "lockscreens", name: "Zen" },
    { id: "a", category: "lockscreens", name: "Amber" },
    { id: "m", category: "lockscreens", name: "Moss" }
], { view: "discover", categoryID: "lockscreens" }).map(item => item.name), ["Amber", "Moss", "Zen"], "category collections sort A to Z");
eq(Store.collection([
    { id: "z", category: "lockscreens", name: "Zen" },
    { id: "a", category: "lockscreens", name: "Amber" }
], { view: "discover", categoryID: "lockscreens", query: "zen" }).map(item => item.name), ["Zen"], "category search stays scoped to the open category");

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
eq(Store.primaryAction({ category: "profiles", installed: false }), "INSTALL", "profile definition install action");
eq(Store.primaryAction({ category: "profiles", installed: true }), "CREATE ACCOUNT", "installed profile opens account creation");
eq(Store.primaryAction({ category: "profiles", installed: true, updateAvailable: true }), "CREATE ACCOUNT", "installed profile keeps its account action");
eq(Store.secondaryAction({ category: "profiles", installed: true, hasSettings: true }), "REMOVE", "installed profile removes its definition secondarily");
eq(Store.secondaryAction({ category: "profiles", installed: false, active: true, hasSettings: true }), "", "an account without its definition has no remove action");
eq(Store.competingActionLabel("lockscreens:clock", "lockscreens:broken", "FETCHING"), "DOWNLOADING ANOTHER ITEM", "another product explains the single-flight download");
eq(Store.competingActionLabel("lockscreens:clock", "lockscreens:clock", "FETCHING"), "", "the active product keeps its own progress stage");
eq(Store.competingActionLabel("lockscreens:clock", "lockscreens:broken", "REMOVING"), "REMOVING ANOTHER ITEM", "another product explains a removal");
eq(Store.competingActionLabel("", "lockscreens:broken", ""), "", "idle actions keep their ordinary label");
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

// An item ruled out by this edition (its package manager) must not claim a window
// manager is the problem, even when it names the one that is running.
const otherEdition = { installed: false, unavailable: true, unavailableEdition: true, requiredWindowManager: "wm-a", unavailableReason: "Needs the AUR." };
eq(Store.unavailableLabel(otherEdition), "NOT ON THIS SYSTEM", "the chip names the edition gate, not the window manager");
eq(Store.primaryAction(otherEdition), "NOT ON THIS SYSTEM", "the disabled action names the edition gate too");
eq(Store.unavailableReason(otherEdition), "Needs the AUR.", "the edition reason is surfaced");

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
    ["plugins:clockbar", "plugins:vpn"],
    "plugins BAR collection filters and sorts A to Z"
);
eq(
    Store.collection(plugins, { view: "discover", categoryID: "plugins" }).map(item => item.id),
    ["bare", "clockbar", "popout", "vpn", "widget"],
    "plugins ALL collection sorts A to Z"
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

const apps = [
    { id: "graphics", category: "flatpak", name: "Graphics", art: "a.png", tags: ["graphics", "photography"], listedAt: "2026-01-02T00:00:00Z", metadata: { trending: 3, installsLastMonth: 900, updatedAt: "2026-01-05T00:00:00Z" } },
    { id: "browser", category: "flatpak", name: "Browser", art: "b.png", tags: ["network"], listedAt: "2026-01-05T00:00:00Z", metadata: { trending: 9, installsLastMonth: 100, updatedAt: "2026-01-03T00:00:00Z" } },
    { id: "player", category: "flatpak", name: "Player", art: "c.png", tags: ["audiovideo", "player"], listedAt: "2026-01-03T00:00:00Z", metadata: { trending: 6, installsLastMonth: 500, updatedAt: "2026-01-04T00:00:00Z" } },
    { id: "bare", category: "flatpak", name: "Bare" }
];
eq(Store.collection(apps, { categoryID: "flatpak", appSort: "trending" }).map(i => i.id), ["browser", "player", "graphics", "bare"], "Apps Trending sorts by the Flathub trending score");
eq(Store.collection(apps, { categoryID: "flatpak", appSort: "popular" }).map(i => i.id), ["graphics", "player", "browser", "bare"], "Apps Popular sorts by monthly installs");
eq(Store.collection(apps, { categoryID: "flatpak", appSort: "new" }).map(i => i.id), ["browser", "player", "graphics", "bare"], "Apps New sorts by listing date");
eq(Store.collection(apps, { categoryID: "flatpak", appSort: "updated" }).map(i => i.id), ["graphics", "player", "browser", "bare"], "Apps Updated sorts by update date");
eq(Store.collection(apps, { categoryID: "flatpak", appSort: "a-z" }).map(i => i.id), ["bare", "browser", "graphics", "player"], "Apps A to Z sorts by display name");
eq(Store.collection(apps, { categoryID: "flatpak", query: "a" }).map(i => i.id), ["graphics", "browser", "player", "bare"], "a search keeps Flathub relevance order");
eq(Store.appCategories(apps).map(c => c.key), ["audiovideo", "graphics", "network"], "market tabs list the categories the catalogue carries");
eq(Store.appCategories(apps)[1].note, "1", "a tab carries its app count");
eq(Store.collection(apps, { categoryID: "flatpak", appSort: "trending", tag: "network" }).map(i => i.id), ["browser"], "a tab narrows the market to its tag");
eq(Store.collection(apps, { categoryID: "flatpak", appSort: "popular", tag: "audiovideo", query: "play" }).map(i => i.id), ["player"], "a tab and a search combine");
eq(Store.featuredApps(apps, 2, 0).map(i => i.id), ["browser", "player"], "featured apps take the top trending entries");
eq(Store.featuredApps(apps, 2, 1).map(i => i.id), ["player", "graphics"], "the daily seed rotates the featured set");
eq(Store.featuredApps(apps.concat([{ id: "owned", category: "flatpak", name: "Owned", art: "o.png", installed: true, metadata: { trending: 99 } }]), 2, 0).map(i => i.id), ["browser", "player"], "installed apps are not featured");
const appsSnapshot = JSON.stringify(apps);
Store.sortApps(apps, "popular");
Store.featuredApps(apps, 3, 4);
eq(JSON.stringify(apps), appsSnapshot, "app sorting and featured picks do not mutate the source");

eq(Store.statusTag({ id: "x", installed: true }), "INSTALLED", "a compact tile chips installed state");
eq(Store.statusTag({ id: "x" }), "", "an available app carries no chip");
eq(Store.initials({ name: "Google Chrome" }), "GC", "initials take the outer words of a name");
eq(Store.initials({ name: "VLC", initials: "vlc" }), "VLC", "an explicit monogram wins");
eq(Store.initials({ name: "" }), "", "a nameless item has no monogram");

console.log("RYOSTORE-STORE-HELPERS-PASS");

// Subshells and bar styles are two real catalogue categories; nothing is
// reclassified by id on this side.
const shellProducts = [
    { id: "orpheus", category: "subshells", installed: true, active: true, metadata: { core: true } },
    { id: "python", category: "subshells", installed: true },
    { id: "gokko", category: "subshells", installed: false },
    { id: "caelestia", category: "subshells", installed: true, downloadPaused: true },
    { id: "obi", category: "barstyles", installed: true, updateAvailable: true },
    { id: "chroma", category: "barstyles", installed: false },
    { id: "launcher", category: "launchers", installed: true }
];
eq(Store.collection(shellProducts, { categoryID: "subshells" }).map(i => i.id),
   ["orpheus", "python", "gokko", "caelestia"], "subshell tab keeps the backend's recommendation order");
eq(Store.collection(shellProducts, { categoryID: "barstyles" }).map(i => i.id),
   ["obi", "chroma"], "bar style tab lists only bar styles, in the backend's order");
eq(Store.collection(shellProducts, { categoryID: "subshells", query: "on" }).map(i => i.id),
   ["python", "caelestia"], "a category search keeps that order too");
eq(Store.categoryName(shellProducts[1], { name: "Subshells" }), "Subshells", "cards carry their category name");
eq(Store.primaryAction(shellProducts[0]), "ACTIVE", "the worn subshell reads active");
eq(Store.primaryDone(shellProducts[0]), true, "an active subshell has nothing left to start");
eq(Store.primaryAction(shellProducts[1]), "APPLY", "an installed subshell applies");
eq(Store.primaryAction(shellProducts[2]), "INSTALL", "a catalogue subshell installs");
eq(Store.primaryAction(shellProducts[3]), "APPLY", "a paused download still lets an installed copy apply");
eq(Store.primaryAction(shellProducts[4]), "UPDATE", "an outdated bar style updates first");
eq(Store.primaryAction(shellProducts[6]), "INSTALLED", "other categories keep their installed state");
eq(Store.isShipped(shellProducts[0]), true, "a shipped subshell is marked core");
eq(Store.isShipped(shellProducts[1]), false, "a store subshell is not shipped");
eq(Store.secondaryAction({ id: "orpheus", category: "subshells", installed: true, hasSettings: true, metadata: { core: true } }),
   "OPEN IN SETTINGS", "a shipped subshell offers its settings, never removal");

eq(
    Store.filter([
        { id: "local", category: "flatpak", name: "Local", summary: "no match here" },
        { id: "live", category: "flatpak", name: "Live", summary: "no match here", remoteMatch: true }
    ], { query: "discord" }).map(i => i.id),
    ["live"],
    "a live search result is kept even when its text does not contain the query"
);
eq(
    Store.filter([{ id: "live", category: "flatpak", name: "Live", remoteMatch: true }], { category: "plugins", query: "discord" }).length,
    0,
    "a live search result still respects the category filter"
);

// Resource tiers come from the catalogue's measurements; the backend decides
// what a modest machine is not recommended for, and the app only presents it.
const measured = { id: "obi", category: "barstyles", name: "Obi", art: "obi.png",
    metadata: { resourceTier: "heavy", resourceDelta: { pssMB: 96.4, gpuMB: 40, cpuPct: 0.66, wakeupsPerSec: -1.25, launchesPerMin: 0 } } };
eq(Store.resourceTier(measured), "heavy", "the published tier is read from metadata");
eq(Store.resourceTier({ metadata: { resourceTier: "massive" } }), "", "an unknown tier reads as unmeasured");
eq(Store.resourceTier({ id: "x" }), "", "no metadata is unmeasured");
eq(Store.resourceDelta(measured), { pssMB: 96.4, gpuMB: 40, cpuPct: 0.66, wakeupsPerSec: -1.25, launchesPerMin: 0 }, "the measured delta is kept whole");
eq(Store.resourceDelta({ metadata: { resourceDelta: { pssMB: 1 } } }), null, "a partial delta is not shown");
eq(Store.resourceDelta({ metadata: { resourceDelta: { pssMB: "1", gpuMB: 0, cpuPct: 0, wakeupsPerSec: 0, launchesPerMin: 0 } } }), null, "a non-numeric delta is not shown");
eq(Store.signedNumber(96.4, 0), "+96", "memory deltas print whole megabytes with a sign");
eq(Store.signedNumber(0.66, 1), "+0.7", "CPU deltas keep one decimal");
eq(Store.signedNumber(-1.25, 1), "-1.3", "a lighter item prints a minus");
eq(Store.signedNumber(0.04, 1), "0", "a delta that rounds away prints 0");

const flagged = Object.assign({}, measured, { notRecommended: true });
eq(Store.isNotRecommended(flagged), true, "the backend's advice is read as is");
eq(Store.isNotRecommended(measured), false, "a heavy item on a capable machine carries no advice");
eq(Store.statusTag(flagged), "NOT RECOMMENDED", "an available item chips the advice");
eq(Store.statusTag(Object.assign({}, flagged, { installed: true })), "INSTALLED", "an owned item keeps its state chip");
eq(Store.primaryAction(flagged), "INSTALL", "advice never blocks the install");

const discover = [
    flagged,
    { id: "sumi", category: "barstyles", name: "Sumi", art: "sumi.png", metadata: { resourceTier: "light" } },
    { id: "paper", category: "rices", name: "Paper", art: "paper.png" },
    Object.assign({}, flagged, { id: "orbit", name: "Orbit", art: "orbit.png" })
];
for (const seed of [0, 1, 7, 20260101]) {
    const order = Store.collection(discover, { view: "discover", seed: seed }).map(i => i.id);
    eq(order.length, discover.length, "Discover keeps every item (seed " + seed + ")");
    assert.ok(!["obi", "orbit"].includes(order[0]), "Discover never leads with an item this machine is not recommended for (seed " + seed + ")");
    eq(order.slice(-2).sort(), ["obi", "orbit"], "not-recommended items sit at the end of Discover (seed " + seed + ")");
}
eq(Store.featured([flagged]).id, "obi", "with nothing else to show, a not-recommended item can still lead");
console.log("RYOSTORE-STORE-TIERS-PASS");
