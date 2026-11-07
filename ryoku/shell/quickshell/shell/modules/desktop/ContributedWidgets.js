.pragma library

// Helpers over Shells.contributions("desktopWidgets"): every installed item's
// widget sets, each with its roster of {id, prefix, label, icon, gloss, kind, sizes}.

function find(sets, prefix) {
    const source = Array.isArray(sets) ? sets : [];
    for (let i = 0; i < source.length; i++) {
        const set = source[i];
        const roster = set && Array.isArray(set.roster) ? set.roster : [];
        for (let j = 0; j < roster.length; j++) {
            if (roster[j] && roster[j].prefix === prefix)
                return { set: set, record: roster[j] };
        }
    }
    return null;
}

function records(sets) {
    const output = [];
    const source = Array.isArray(sets) ? sets : [];
    for (let i = 0; i < source.length; i++) {
        const set = source[i];
        const roster = set && Array.isArray(set.roster) ? set.roster : [];
        for (let j = 0; j < roster.length; j++)
            if (roster[j] && roster[j].prefix)
                output.push({ set: set, record: roster[j] });
    }
    return output;
}

// True when a widget the user selected belongs to a set that is not unpacked yet.
function needsSync(sets, selected) {
    const entries = records(sets);
    for (let i = 0; i < entries.length; i++) {
        if (entries[i].set.prepared !== true && selected(entries[i].record.prefix))
            return true;
    }
    return false;
}

function options(raw) {
    if (raw && typeof raw === "object")
        return raw;
    if (raw === undefined || raw === null || String(raw).length === 0)
        return ({});
    try {
        const parsed = JSON.parse(String(raw));
        return parsed && typeof parsed === "object" ? parsed : ({});
    } catch (e) {
        return ({});
    }
}

function optionSource(match) {
    if (!match || !match.set || !match.record)
        return "";
    const base = String(match.set.optionsUrl || "");
    const prefix = String(match.record.prefix || "");
    if (base.length === 0 || prefix.length === 0)
        return "";
    const filename = prefix.charAt(0).toUpperCase() + prefix.slice(1) + "Options.qml";
    return base.replace(/\/$/, "") + "/" + filename;
}
