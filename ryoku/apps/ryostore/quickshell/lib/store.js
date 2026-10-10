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

// unavailable: a product the catalogue wrote for another window manager. It is
// refused for install and update by the backend (a client cannot bypass that),
// stays listed, and keeps any installed copy removable; every surface here must
// show it greyed with the reason instead of an action that cannot work.
function isUnavailable(item) {
    return Boolean(item && item.unavailable);
}

// The chip and the disabled action both name the window manager the product
// wants, taken from the product itself rather than from a list of names here.
function unavailableLabel(item) {
    if (!isUnavailable(item))
        return "";
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
    var haystack = searchText(item);
    return terms.every(function(term) { return haystack.indexOf(term) !== -1; });
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
// installed, then any real art, then anything without a source error).
function eligiblePool(source) {
    var withArt = source.filter(function(x) { return !x.sourceError && x.art; });
    var fresh = withArt.filter(function(x) { return !isInstalled(x); });
    if (fresh.length) return fresh;
    if (withArt.length) return withArt;
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

function collection(items, options) {
    var opts = options || {};
    var filtered = filter(items, {
        category: opts.categoryID || "",
        installedOnly: opts.view === "library" || opts.installedOnly === true,
        provider: opts.provider || "",
        pluginKind: opts.pluginKind || "",
        query: opts.query || ""
    });
    if (opts.categoryID === "omarchy-plugins")
        filtered = sortOmarchyPlugins(filtered, opts.omarchySort || "popular");
    if (opts.view !== "library" && !opts.categoryID && !opts.query) {
        var seed = Number(opts.seed) || 0;
        var lead = featured(filtered, seed);
        if (!lead)
            return filtered;
        var rest = filtered.filter(function(item) { return itemKey(item) !== itemKey(lead); });
        return [lead].concat(seed > 0 ? shuffleSeeded(rest, seed) : rest);
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
    if (isDownloadPaused(item))
        return "UNDER CONSTRUCTION";
    if (item && item.busy)
        return "INSTALLING";
    if (item && item.updateAvailable)
        return "UPDATE";
    if (item && Number(item.installedCount || 0) > 0 && Number(item.totalCount || 0) > Number(item.installedCount || 0))
        return "INSTALL " + String(Number(item.totalCount) - Number(item.installedCount)) + " ITEMS";
    return isInstalled(item) ? "INSTALLED" : "INSTALL";
}

function secondaryAction(item) {
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
    module.exports = { statusLabels, isInstalled, isDownloadPaused, downloadPauseReason, isUnavailable, unavailableLabel, unavailableReason, pluginKind, searchText, matchesQuery, filter, groupSearch, featured, installed, itemKey, sortOmarchyPlugins, collection, selectionKey, categoryPlates, primaryAction, secondaryAction, sortCategories, shuffleSeeded };
