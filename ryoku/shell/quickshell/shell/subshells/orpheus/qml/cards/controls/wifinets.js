function securityClass(security) {
    var value = String(security || "").toLowerCase();
    if (value === "" || value === "none" || value === "open")
        return "open";
    return value === "enterprise" ? "enterprise" : "secured";
}

function bandLabel(band) {
    return band ? band + " GHz" : "";
}

// access points of one name, security kind and band collapse into a row led by the strongest
function networks(aps, activeSsid, activeSecurity, activeBand) {
    var groups = {};
    var order = [];
    var list = aps || [];
    for (var i = 0; i < list.length; i++) {
        var ap = list[i];
        if (!ap)
            continue;
        var ssid = String(ap.ssid || "");
        var band = ap.band ? String(ap.band) : "";
        if (ssid !== "" && ssid === activeSsid && securityClass(ap.security) === securityClass(activeSecurity)
                && (!activeBand || band === activeBand))
            continue;
        // a hidden network has no name to share, so each access point stays alone
        var key = ssid === ""
            ? "\u0001" + String(ap.bssid || i)
            : ssid + "\u0000" + securityClass(ap.security) + "\u0000" + band;
        var group = groups[key];
        if (!group) {
            group = { key: key, ssid: ssid, security: ap.security || "", bssid: ap.bssid || "",
                band: band, bandLabel: bandLabel(band), strength: Number(ap.strength || 0),
                saved: false, autoconnect: false };
            groups[key] = group;
            order.push(key);
        } else if (Number(ap.strength || 0) > group.strength) {
            group.strength = Number(ap.strength || 0);
            group.security = ap.security || group.security;
            group.bssid = ap.bssid || group.bssid;
        }
        group.saved = group.saved || !!ap.saved;
        group.autoconnect = group.autoconnect || !!ap.autoconnect;
    }
    var out = order.map(function(k) {
        var g = groups[k];
        // steps of ten so a flickering reading does not shuffle rows
        g.tier = Math.floor(g.strength / 10);
        return g;
    });
    out.sort(function(a, b) {
        if (a.saved !== b.saved)
            return a.saved ? -1 : 1;
        if (a.tier !== b.tier)
            return b.tier - a.tier;
        var byName = a.ssid.localeCompare(b.ssid);
        return byName !== 0 ? byName : Number(a.band) - Number(b.band);
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
