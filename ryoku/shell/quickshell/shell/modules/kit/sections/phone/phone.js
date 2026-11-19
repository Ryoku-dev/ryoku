function device(state) {
    const devices = Array.isArray(state.devices) ? state.devices.filter(d => d && d.paired === true) : [];
    return devices.find(d => String(d.id) === String(state.activeDeviceId)) || devices[0] || null;
}

function numbers(contact) {
    return Array.isArray(contact.numbers) ? contact.numbers.map(n => String(n.number || "")).filter(Boolean) : [];
}

function matches(query, texts) {
    const haystack = texts.join(" ").toLocaleLowerCase();
    return String(query || "").trim().toLocaleLowerCase().split(/\s+/).every(term => haystack.indexOf(term) >= 0);
}

function arrivals(previous, pending, notifications, current) {
    const known = {};
    const next = current ? {} : Object.assign({}, pending);
    notifications.forEach(n => {
        const id = String(n.id || "");
        if (!id) return;
        known[id] = true;
        if (previous !== null && !previous[id] && !current) next[id] = true;
    });
    return { known: known, pending: next };
}
