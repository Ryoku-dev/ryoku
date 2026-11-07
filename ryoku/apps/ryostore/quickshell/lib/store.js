function statusLabels(item) {
    var labels = [];
    if (isUnavailable(item))
        labels.push(unavailableLabel(item));
    if (isDownloadPaused(item))
        labels.push("UNDER CONSTRUCTION");
    if (item && item.updateAvailable)
        labels.push("UPDATE");
    if (item && item.active)
        labels.push("ACTIVE");
    else if (item && item.enabled)
        labels.push("ENABLED");
    else if (item && Number(item.installedCount || 0) > 0 && Number(item.totalCount || 0) > 0)
        labels.push(String(item.installedCount) + " / " + String(item.totalCount) + " INSTALLED");
    else if (item && item.installed)
        labels.push("INSTALLED");
    else if (!isDownloadPaused(item) && !isUnavailable(item))
        labels.push("AVAILABLE");
    return labels;
}

function isInstalled(item) {
    return Boolean(item && (item.installed || item.active || item.enabled || Number(item.installedCount || 0) > 0));
}

// downloadPaused: an item the catalogue has frozen. It stays listed and an
// installed copy can still be removed, but every install/update surface must
// treat it as unavailable and surface the reason instead of a download.
function isDownloadPaused(item) {
    return Boolean(item && item.downloadPaused);
}

function downloadPauseReason(item) {
    return isDownloadPaused(item) ? String((item && item.downloadPauseReason) || "") : "";
}

// unavailable: a product this machine cannot run, because the catalogue wrote it
// for another window manager or another edition (package manager). It is
// refused for install and update by the backend (a client cannot bypass that),
// stays listed, and keeps any installed copy removable; every surface here must
// show it greyed with the reason instead of an action that cannot work.
function isUnavailable(item) {
    return Boolean(item && item.unavailable);
}

// The chip and the disabled action name the cause: the window manager the
// product wants, taken from the product itself rather than from a list of names
// here, or this system when the edition is what rules it out.
function unavailableLabel(item) {
    if (!isUnavailable(item))
        return "";
    if (item.unavailableEdition)
        return "NOT ON THIS SYSTEM";
    var want = String((item && item.requiredWindowManager) || "").toUpperCase();
    return want.length > 0 ? want + " ONLY" : "UNAVAILABLE";
}

function unavailableReason(item) {
    return isUnavailable(item) ? String((item && item.unavailableReason) || "") : "";
}

// pluginKind classifies a plugin by its host surface. Sidebar-only plugins stay
// in the all-plugins view instead of falling through to the desktop tab.
function pluginKind(item) {
    var hosts = (item && item.metadata && Array.isArray(item.metadata.hosts)) ? item.metadata.hosts : [];
    if (hosts.indexOf("topbarGlyph") !== -1)
        return "bar";
    if (hosts.indexOf("desktopWidget") !== -1 || hosts.indexOf("framePopout") !== -1)
        return "desktop";
    return hosts.indexOf("sidebarCard") !== -1 ? "sidebar" : "desktop";
}

function searchText(item) {
    if (!item)
        return "";
    if (item.searchIndex)
        return String(item.searchIndex);
    return [
        item.id,
        item.category,
        item.categoryName,
        item.name,
        item.summary,
        item.description,
        item.author,
        item.version,
        item.kind,
        item.license,
        item.verificationStatus,
        item.metadata && item.metadata.marketplaceCategory,
        (item.tags || []).join(" "),
        statusLabels(item).join(" ")
    ].filter(Boolean).join(" ").toLowerCase();
}

function matchesQuery(item, query) {
    var terms = String(query || "").trim().toLowerCase().split(/\s+/).filter(Boolean);
    if (terms.length === 0)
        return true;
    // A live search result was matched by its source (typos, keywords and
    // translations included), so the literal text test would wrongly drop it.
    if (item && item.remoteMatch === true)
        return true;
    var haystack = searchText(item);
    return terms.every(function(term) { return haystack.indexOf(term) !== -1; });
}

// Subshells and bar styles are worn through shell.json barStyle: an installed
// one is applied, and one that ships with Ryoku (metadata.core) is never removed.
function isShellItem(item) {
    var category = String(item && item.category || "");
    return category === "subshells" || category === "barstyles";
}
function isShipped(item) {
    return Boolean(item && item.metadata && item.metadata.core === true);
}
function categoryName(item, category) {
    return String(category && category.name || item && item.category || "");
}

// The catalogue measures every subshell and bar style against Orpheus and
// publishes a tier with the measured delta (docs/store.md). Anything else in
// that field reads as unmeasured.
function resourceTier(item) {
    var tier = String(item && item.metadata && item.metadata.resourceTier || "");
    return tier === "light" || tier === "standard" || tier === "heavy" ? tier : "";
}

// resourceDelta: the measured difference from Orpheus, or null when the item
// has no complete, finite measurement.
function resourceDelta(item) {
    var delta = item && item.metadata && item.metadata.resourceDelta;
    if (!delta || typeof delta !== "object")
        return null;
    var keys = ["pssMB", "gpuMB", "cpuPct", "wakeupsPerSec", "launchesPerMin"];
    var out = {};
    for (var i = 0; i < keys.length; i++) {
        var value = delta[keys[i]];
        if (typeof value !== "number" || !isFinite(value))
            return null;
        out[keys[i]] = value;
    }
    return out;
}

// signedNumber prints a delta with its sign and at most `digits` decimals, so
// +12, -3.5 and 0 read as differences rather than totals. Halves round away
// from zero either way, so a lighter item reads exactly as far below as a
// heavier one reads above.
function signedNumber(value, digits) {
    var number = Number(value);
    var places = Math.max(0, Number(digits) || 0);
    var scale = Math.pow(10, places);
    var magnitude = Math.round(Math.abs(number) * scale) / scale;
    if (!isFinite(magnitude) || magnitude === 0)
        return "0";
    return (number > 0 ? "+" : "-") + String(magnitude);
}

// notRecommended: the backend's advice that a heavy item asks more of this
// machine than it comfortably has. A note only; every action stays available.
function isNotRecommended(item) {
    return Boolean(item && item.notRecommended);
}

function filter(items, options) {
    var source = Array.isArray(items) ? items : [];
    var opts = options || {};
    return source.filter(function(item) {
        if (opts.category && item.category !== opts.category)
            return false;
        if (opts.installedOnly && !isInstalled(item))
            return false;
        if (opts.updatesOnly && !item.updateAvailable)
            return false;
        if (opts.provider && (item.metadata && item.metadata.provider ? item.metadata.provider : "Community") !== opts.provider)
            return false;
        if (opts.tag && (item.tags || []).indexOf(opts.tag) === -1)
            return false;
        if (opts.pluginKind && pluginKind(item) !== opts.pluginKind)
            return false;
        return matchesQuery(item, opts.query);
    });
}

function groupSearch(items, query) {
    return filter(items, { query: query });
}

// mulberry32: a tiny deterministic PRNG, so a given seed always yields the same
// sequence. Discover seeds it with the day number: the page rotates daily but
// holds still while you browse.
function mulberry32(a) {
    return function() {
        a |= 0; a = (a + 0x6D2B79F5) | 0;
        var t = Math.imul(a ^ (a >>> 15), 1 | a);
        t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
        return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    };
}

// shuffleSeeded: Fisher-Yates with a seeded PRNG. Returns a new array (never
// mutates the input), so the same seed always produces the same order.
function shuffleSeeded(items, seed) {
    var a = (Array.isArray(items) ? items : []).slice();
    var rand = mulberry32(Number(seed) || 1);
    for (var i = a.length - 1; i > 0; i--) {
        var j = Math.floor(rand() * (i + 1));
        var tmp = a[i]; a[i] = a[j]; a[j] = tmp;
    }
    return a;
}

// eligiblePool: the Discover hero candidates, best tier first (real art and not
// installed, then any real art, then anything without a source error). Within
// the first two, an item recommended for this machine wins over one that is not.
function eligiblePool(source) {
    var withArt = source.filter(function(x) { return !x.sourceError && x.art; });
    var fresh = withArt.filter(function(x) { return !isInstalled(x); });
    var pools = [fresh, withArt];
    for (var i = 0; i < pools.length; i++) {
        var fitting = pools[i].filter(function(x) { return !isNotRecommended(x); });
        if (fitting.length) return fitting;
        if (pools[i].length) return pools[i];
    }
    return source.filter(function(x) { return !x.sourceError; });
}

// featured: the Discover hero. With no seed it is deterministic (first eligible),
// preserving the legacy order; with a day seed it rotates through the pool.
function featured(items, seed) {
    var pool = eligiblePool(Array.isArray(items) ? items : []);
    if (!pool.length)
        return null;
    var n = Number(seed) || 0;
    return n > 0 ? pool[Math.floor(mulberry32(n)() * pool.length)] : pool[0];
}

function installed(items) {
    return (Array.isArray(items) ? items : []).filter(isInstalled);
}

function itemKey(item) {
    return item ? String(item.category || "") + ":" + String(item.id || "") : "";
}

function sortOmarchyPlugins(items, mode) {
    var sorted = (Array.isArray(items) ? items : []).slice();
    sorted.sort(function(a, b) {
        if (mode === "new" && String(a.listedAt || "") !== String(b.listedAt || ""))
            return String(b.listedAt || "").localeCompare(String(a.listedAt || ""));
        if (mode === "verified") {
            var av = String(a.verificationStatus || "").toLowerCase() === "verified";
            var bv = String(b.verificationStatus || "").toLowerCase() === "verified";
            if (av !== bv)
                return av ? -1 : 1;
        }
        var stars = Number(b.stars || 0) - Number(a.stars || 0);
        return stars !== 0 ? stars : String(a.name || a.id || "").localeCompare(String(b.name || b.id || ""));
    });
    return sorted;
}

function appMetric(item, mode) {
    var meta = (item && item.metadata) || {};
    if (mode === "popular")
        return Number(meta.installsLastMonth || 0);
    if (mode === "trending")
        return Number(meta.trending || 0);
    if (mode === "new")
        return String(item && item.listedAt || "");
    if (mode === "updated")
        return String(meta.updatedAt || (item && item.listedAt) || "");
    return 0;
}

function sortApps(items, mode) {
    var sorted = (Array.isArray(items) ? items : []).slice();
    var key = mode || "trending";
    if (key === "a-z")
        return sortAlphabetical(sorted);
    sorted.sort(function(a, b) {
        var av = appMetric(a, key);
        var bv = appMetric(b, key);
        var cmp = typeof av === "number" && typeof bv === "number"
                ? bv - av
                : String(bv).localeCompare(String(av));
        return cmp !== 0 ? cmp
                : String(a.name || a.id || "").localeCompare(String(b.name || b.id || ""));
    });
    return sorted;
}

function featuredApps(items, count, seed) {
    var source = Array.isArray(items) ? items : [];
    var size = Math.max(1, Number(count) || 1);
    var pool = source.filter(function(item) { return item && item.art && !isInstalled(item); });
    if (!pool.length)
        pool = source.filter(function(item) { return item && item.art; });
    if (!pool.length)
        pool = source;
    // A wider pool than the shelf keeps the daily rotation on the strongest apps.
    var top = sortApps(pool, "trending").slice(0, Math.max(size, 5));
    if (!top.length)
        return [];
    var start = (Number(seed) || 0) % top.length;
    if (start < 0)
        start += top.length;
    return top.slice(start).concat(top.slice(0, start)).slice(0, size);
}

// AVAILABLE is the absence of state, so it never becomes a chip; an available
// item this machine is not recommended for chips that advice instead.
function statusTag(item) {
    var labels = statusLabels(item);
    if (labels.length > 0 && labels[0] !== "AVAILABLE")
        return labels[0];
    return isNotRecommended(item) ? "NOT RECOMMENDED" : "";
}

function initials(item, title) {
    var explicit = String(item && item.initials || "").trim();
    if (explicit !== "")
        return explicit.slice(0, 3).toUpperCase();
    var name = String(title || (item && (item.name || item.id)) || "").trim();
    var words = name.split(/\s+/);
    return (words.length > 1 ? words[0][0] + words[words.length - 1][0]
                             : name.slice(0, 2)).toUpperCase();
}

// Flathub's main categories, in the API's own order. An app carries them as
// tags, so the market tabs need no backend field.
var appCategoryLabels = {
    "audiovideo": "AUDIO & VIDEO",
    "development": "DEVELOPMENT",
    "education": "EDUCATION",
    "game": "GAMES",
    "graphics": "GRAPHICS",
    "network": "NETWORK",
    "office": "OFFICE",
    "science": "SCIENCE",
    "system": "SYSTEM",
    "utility": "UTILITY"
};

function appCategories(items) {
    var counts = {};
    (Array.isArray(items) ? items : []).forEach(function(item) {
        if (!item || item.category !== "flatpak")
            return;
        var tags = item.tags || [];
        Object.keys(appCategoryLabels).forEach(function(key) {
            if (tags.indexOf(key) !== -1)
                counts[key] = (counts[key] || 0) + 1;
        });
    });
    return Object.keys(appCategoryLabels).filter(function(key) {
        return counts[key] > 0;
    }).map(function(key) {
        return { key: key, label: appCategoryLabels[key], note: String(counts[key]) };
    });
}

function sortAlphabetical(items) {
    return (Array.isArray(items) ? items : []).slice().sort(function(a, b) {
        var an = String(a && (a.name || a.id) || "").toLocaleLowerCase();
        var bn = String(b && (b.name || b.id) || "").toLocaleLowerCase();
        var byName = an.localeCompare(bn);
        return byName || itemKey(a).localeCompare(itemKey(b));
    });
}

function collection(items, options) {
    var opts = options || {};
    var filtered = filter(items, {
        category: opts.categoryID || "",
        installedOnly: opts.view === "library" || opts.installedOnly === true,
        provider: opts.provider || "",
        pluginKind: opts.pluginKind || "",
        tag: opts.tag || "",
        query: opts.query || ""
    });
    if (opts.appSort)
        filtered = sortApps(filtered, opts.appSort);
    else if (opts.categoryID === "omarchy-plugins")
        filtered = sortOmarchyPlugins(filtered, opts.omarchySort || "popular");
    // Subshells and bar styles keep the backend's order: A to Z, or on a modest
    // machine lightest first (ryostore backend resource_tier.go).
    else if (opts.categoryID && opts.categoryID !== "flatpak" && !isShellItem({ category: opts.categoryID }))
        filtered = sortAlphabetical(filtered);
    if (opts.view !== "library" && !opts.categoryID && !opts.query) {
        var seed = Number(opts.seed) || 0;
        var lead = featured(filtered, seed);
        if (!lead)
            return filtered;
        var rest = filtered.filter(function(item) { return itemKey(item) !== itemKey(lead); });
        rest = seed > 0 ? shuffleSeeded(rest, seed) : rest;
        // What this machine is not recommended for sits behind everything else.
        return [lead].concat(rest.filter(function(item) { return !isNotRecommended(item); }),
                             rest.filter(isNotRecommended));
    }
    return filtered;
}

function selectionKey(items, requestedKey, fallbackIndex) {
    var source = Array.isArray(items) ? items : [];
    for (var i = 0; i < source.length; i++)
        if (itemKey(source[i]) === requestedKey)
            return requestedKey;
    if (source.length === 0)
        return "";
    var index = Math.max(0, Math.min(source.length - 1, Number(fallbackIndex || 0)));
    return itemKey(source[index]);
}

function categoryPlates(categories) {
    return (Array.isArray(categories) ? categories : []).map(function(category) {
        var copy = {};
        Object.keys(category).forEach(function(key) { copy[key] = category[key]; });
        return copy;
    });
}

function primaryAction(item) {
    if (isUnavailable(item))
        return unavailableLabel(item);
    // A pause stops downloads only: an installed subshell or bar style can still be worn.
    if (isShellItem(item) && isInstalled(item) && !(item.updateAvailable && !isDownloadPaused(item)) && !item.busy)
        return item.active ? "ACTIVE" : "APPLY";
    if (isDownloadPaused(item))
        return "UNDER CONSTRUCTION";
    if (item && item.busy)
        return "INSTALLING";
    if (item && String(item.category || "") === "profiles")
        return item.installed === true ? "CREATE ACCOUNT" : "INSTALL";
    if (item && item.updateAvailable)
        return "UPDATE";
    if (item && Number(item.installedCount || 0) > 0 && Number(item.totalCount || 0) > Number(item.installedCount || 0))
        return "INSTALL " + String(Number(item.totalCount) - Number(item.installedCount)) + " ITEMS";
    return isInstalled(item) ? "INSTALLED" : "INSTALL";
}

// A finished action: nothing left to start from the primary button.
function primaryDone(item) {
    var action = primaryAction(item);
    return action === "INSTALLED" || action === "ACTIVE";
}

function competingActionLabel(busyKey, actionKey, installStage) {
    if (String(busyKey || "") === "" || String(busyKey) === String(actionKey || ""))
        return "";
    var stage = String(installStage || "").toUpperCase();
    if (stage.indexOf("REMOV") !== -1)
        return "REMOVING ANOTHER ITEM";
    if (stage.indexOf("ENABL") !== -1 || stage.indexOf("DISABL") !== -1)
        return "CHANGING ANOTHER ITEM";
    return "DOWNLOADING ANOTHER ITEM";
}

function secondaryAction(item) {
    if (item && String(item.category || "") === "profiles")
        return item.installed === true ? "REMOVE" : "";
    return (item && item.hasSettings && isInstalled(item)) ? "OPEN IN SETTINGS" : "";
}

function sortCategories(categories) {
    var rank = { find: 0, wear: 1, extend: 2 };
    return (Array.isArray(categories) ? categories : []).map(function(category, index) {
        return { category: category, index: index };
    }).sort(function(a, b) {
        var ar = Object.prototype.hasOwnProperty.call(rank, a.category.group) ? rank[a.category.group] : 99;
        var br = Object.prototype.hasOwnProperty.call(rank, b.category.group) ? rank[b.category.group] : 99;
        return ar - br || a.index - b.index;
    }).map(function(row) { return row.category; });
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { statusLabels, statusTag, initials, isInstalled, isDownloadPaused, downloadPauseReason, isUnavailable, unavailableLabel, unavailableReason, pluginKind, searchText, matchesQuery, filter, groupSearch, featured, installed, itemKey, sortOmarchyPlugins, sortApps, sortAlphabetical, featuredApps, appCategories, collection, selectionKey, categoryPlates, primaryAction, primaryDone, competingActionLabel, secondaryAction, sortCategories, shuffleSeeded, isShellItem, isShipped, categoryName, resourceTier, resourceDelta, signedNumber, isNotRecommended };
