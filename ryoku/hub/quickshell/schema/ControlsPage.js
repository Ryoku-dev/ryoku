.pragma library

var sections = [
    {
        id: "vitals", label: "Vitals", kana: "状態", visible: true,
        elements: [
            { id: "identity", label: "Identity & clock" },
            { id: "cpu", label: "CPU" },
            { id: "cpuTemperature", label: "Temperature" },
            { id: "liveGraph", label: "Live graph" },
            { id: "memory", label: "Memory" },
            { id: "gpu", label: "GPU" },
            { id: "network", label: "Network" },
            { id: "disk", label: "Disk" },
            { id: "battery", label: "Battery" }
        ]
    },
    {
        id: "connections", label: "Connections", kana: "接続", visible: true,
        elements: [
            { id: "wifi", label: "Wi-Fi" },
            { id: "bluetooth", label: "Bluetooth" },
            { id: "ethernet", label: "Ethernet" },
            { id: "vpn", label: "VPN" }
        ]
    },
    { id: "powerProfile", label: "Power profile", kana: "電力", visible: true, elements: [] },
    {
        id: "media", label: "Now playing", kana: "音楽", visible: false,
        elements: [
            { id: "mediaArtwork", label: "Artwork" },
            { id: "mediaTrack", label: "Track details" },
            { id: "mediaTransport", label: "Transport" }
        ]
    },
    {
        id: "levels", label: "Levels", kana: "音量", visible: true,
        elements: [
            { id: "volume", label: "Volume" },
            { id: "brightness", label: "Brightness" }
        ]
    },
    {
        id: "bottomControls", label: "Bottom controls", kana: "操作", visible: true,
        elements: [
            { id: "lock", label: "Lock" },
            { id: "sleep", label: "Sleep" },
            { id: "logout", label: "Log out" },
            { id: "restart", label: "Restart" },
            { id: "powerOff", label: "Power off" },
            { id: "nightLight", label: "Night light" },
            { id: "keepAwake", label: "Keep awake" },
            { id: "doNotDisturb", label: "Do not disturb" },
            { id: "micMute", label: "Mic mute" },
            { id: "gamingMode", label: "Gaming mode" },
            { id: "panelSettings", label: "Panel settings" },
            { id: "pluginCards", label: "Plugin cards" }
        ]
    }
];

var rows = [
    { tab: "Layout", group: "PANEL", key: "controls.sections", label: "Controls layout", desc: "Show, hide, and reorder the blocks in Super+Escape", ctl: "layoutdemo", src: "shell", keywords: "sidebar quick settings panel sections order drag" },
    { tab: "Layout", group: "VITALS", key: "controls.hidden", label: "Visible readings", desc: "Choose the readings and live graph that do work while Controls is open", ctl: "layoutdemo", src: "shell", keywords: "cpu ram memory gpu temperature graph sparkline network disk battery" }
];

function defaults() {
    var ordered = [];
    for (var i = 0; i < sections.length; ++i)
        ordered.push({ id: sections[i].id, visible: sections[i].visible });
    return { sections: ordered, hidden: [] };
}

function section(id) {
    for (var i = 0; i < sections.length; ++i)
        if (sections[i].id === id)
            return sections[i];
    return null;
}

function normalize(value) {
    var source = value && typeof value === "object" ? value : {};
    var incoming = Array.isArray(source.sections) ? source.sections : [];
    var seen = {};
    var ordered = [];
    for (var i = 0; i < incoming.length; ++i) {
        var item = incoming[i];
        var id = typeof item === "string" ? item : item && item.id;
        if (!section(id) || seen[id])
            continue;
        seen[id] = true;
        ordered.push({ id: id, visible: typeof item === "object" && item.visible !== undefined ? item.visible === true : true });
    }
    for (var j = 0; j < sections.length; ++j) {
        var def = sections[j];
        if (!seen[def.id])
            ordered.push({ id: def.id, visible: def.visible });
    }

    var allowed = {};
    for (var si = 0; si < sections.length; ++si)
        for (var ei = 0; ei < sections[si].elements.length; ++ei)
            allowed[sections[si].elements[ei].id] = true;
    var hidden = [];
    var hiddenSeen = {};
    var rawHidden = Array.isArray(source.hidden) ? source.hidden : [];
    for (var hi = 0; hi < rawHidden.length; ++hi) {
        var key = String(rawHidden[hi]);
        if (allowed[key] && !hiddenSeen[key]) {
            hiddenSeen[key] = true;
            hidden.push(key);
        }
    }
    return { sections: ordered, hidden: hidden };
}
