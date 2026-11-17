function securityClass(security) {
    var value = String(security || "").toLowerCase();
    if (value === "" || value === "none" || value === "open")
        return "open";
    return value === "enterprise" ? "enterprise" : "secured";
}

function bandLabel(bands) {
    return bands.length > 0 ? bands.join(" · ") + " GHz" : "";
}

// access points of one name and security kind collapse into a row led by the strongest
function networks(aps, activeSsid, activeSecurity) {
    var groups = {};
    var order = [];
    var list = aps || [];
    for (var i = 0; i < list.length; i++) {
        var ap = list[i];
        if (!ap)
            continue;
        var ssid = String(ap.ssid || "");
        if (ssid !== "" && ssid === activeSsid && securityClass(ap.security) === securityClass(activeSecurity))
            continue;
        // a hidden network has no name to share, so each access point stays alone
        var key = ssid === ""
            ? "\u0001" + String(ap.bssid || i)
            : ssid + "\u0000" + securityClass(ap.security);
        var group = groups[key];
        if (!group) {
            group = { key: key, ssid: ssid, security: ap.security || "", bssid: ap.bssid || "",
                strength: Number(ap.strength || 0), saved: false, autoconnect: false, bands: [] };
            groups[key] = group;
            order.push(key);
        } else if (Number(ap.strength || 0) > group.strength) {
            group.strength = Number(ap.strength || 0);
            group.security = ap.security || group.security;
            group.bssid = ap.bssid || group.bssid;
        }
        group.saved = group.saved || !!ap.saved;
        group.autoconnect = group.autoconnect || !!ap.autoconnect;
        if (ap.band && group.bands.indexOf(String(ap.band)) < 0)
            group.bands.push(String(ap.band));
    }
    var out = order.map(function(k) {
        var g = groups[k];
        g.bands.sort(function(a, b) { return Number(a) - Number(b); });
        g.bandLabel = bandLabel(g.bands);
        // steps of ten so a flickering reading does not shuffle rows
        g.tier = Math.floor(g.strength / 10);
        return g;
    });
    out.sort(function(a, b) {
        if (a.saved !== b.saved)
            return a.saved ? -1 : 1;
        if (a.tier !== b.tier)
            return b.tier - a.tier;
        return a.ssid.localeCompare(b.ssid);
    });
    return out;
}

function activeBand(aps) {
    var list = aps || [];
    for (var i = 0; i < list.length; i++)
        if (list[i] && list[i].active)
            return list[i].band ? String(list[i].band) : "";
    return "";
}

function activeSecurity(aps) {
    var list = aps || [];
    for (var i = 0; i < list.length; i++)
        if (list[i] && list[i].active)
            return list[i].security || "";
    return "";
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { networks, activeBand, activeSecurity, bandLabel, securityClass };
