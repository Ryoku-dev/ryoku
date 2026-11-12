.pragma library

var title = "Quick settings";
var lead = "The panel Super+Escape opens: the blocks it shows and their order, its design, and where it sits on the screen.";

var sections = [
    { id: "blocks", title: "Blocks and their order" },
    { id: "design", title: "Design" },
    { id: "position", title: "Position and edge gap" },
    { id: "cards", title: "Extension cards" }
];

// the panel's blocks in their shipped order, each with the parts it can hide;
// ids are the shell's controls.sections and controls.hidden ids
var blocks = [
    { id: "vitals", label: "System status", visible: true,
      detail: "The time, and how hard the computer is working",
      keywords: ["vitals", "readings", "monitor", "stats", "usage"],
      parts: [
          { id: "identity", label: "Name and clock", keywords: ["hostname", "user", "uptime", "date", "time"] },
          { id: "cpu", label: "Processor", keywords: ["cpu", "load", "usage"] },
          { id: "cpuTemperature", label: "Temperature", keywords: ["cpu temperature", "heat", "thermal"] },
          { id: "liveGraph", label: "Activity graph", keywords: ["live graph", "sparkline", "chart", "history"] },
          { id: "memory", label: "Memory", keywords: ["ram", "swap"] },
          { id: "gpu", label: "Graphics card", keywords: ["gpu", "vram"] },
          { id: "network", label: "Network speed", keywords: ["download", "upload", "traffic", "bandwidth"] },
          { id: "disk", label: "Disk", keywords: ["storage", "drive", "space"] },
          { id: "battery", label: "Battery", keywords: ["charge", "power"] }
      ] },
    { id: "connections", label: "Connections", visible: true,
      detail: "Wi-Fi, Bluetooth, wired network and VPN",
      keywords: ["network", "internet"],
      parts: [
          { id: "wifi", label: "Wi-Fi", keywords: ["wireless", "wlan"] },
          { id: "bluetooth", label: "Bluetooth", keywords: ["headphones", "pair"] },
          { id: "ethernet", label: "Wired network", keywords: ["ethernet", "cable", "lan"] },
          { id: "vpn", label: "VPN", keywords: ["tunnel", "wireguard"] }
      ] },
    { id: "powerProfile", label: "Power mode", visible: true,
      detail: "Switch between saving power, balanced and performance",
      keywords: ["power profile", "performance", "balanced", "power saver"], parts: [] },
    { id: "media", label: "Now playing", visible: false,
      detail: "What is playing, with its controls",
      keywords: ["media", "music", "player", "mpris"],
      parts: [
          { id: "mediaArtwork", label: "Album art", keywords: ["artwork", "cover"] },
          { id: "mediaTrack", label: "Track details", keywords: ["title", "artist", "song"] },
          { id: "mediaTransport", label: "Play, pause and skip", keywords: ["transport", "controls", "next", "previous"] }
      ] },
    { id: "levels", label: "Volume and brightness", visible: true,
      detail: "Sliders for sound and screen brightness",
      keywords: ["levels", "sliders", "audio", "sound"],
      parts: [
          { id: "volume", label: "Volume", keywords: ["sound", "audio", "mixer"] },
          { id: "brightness", label: "Brightness", keywords: ["screen", "backlight"] }
      ] },
    { id: "bottomControls", label: "Buttons along the bottom", visible: true,
      detail: "Lock, sleep and power buttons, and quick switches",
      keywords: ["bottom controls", "power buttons", "toggles", "switches"],
      parts: [
          { id: "lock", label: "Lock", keywords: ["lock screen"] },
          { id: "sleep", label: "Sleep", keywords: ["suspend"] },
          { id: "logout", label: "Log out", keywords: ["sign out", "logout"] },
          { id: "restart", label: "Restart", keywords: ["reboot"] },
          { id: "powerOff", label: "Power off", keywords: ["shut down", "shutdown"] },
          { id: "nightLight", label: "Night light", keywords: ["warm", "blue light"] },
          { id: "keepAwake", label: "Keep awake", keywords: ["caffeine", "inhibit sleep", "idle"] },
          { id: "doNotDisturb", label: "Do not disturb", keywords: ["dnd", "silence notifications"] },
          { id: "micMute", label: "Mute the microphone", keywords: ["mic", "microphone"] },
          { id: "gamingMode", label: "Game mode", keywords: ["gaming mode", "games"] },
          { id: "panelSettings", label: "Panel settings button", keywords: ["gear", "panel options"] }
      ] }
];

function q(key) { return { store: "quicksettings", key: key }; }
function s(key, fallback) { return { store: "settings", key: key, fallback: fallback }; }

function blockRows() {
    var out = [];
    for (var i = 0; i < blocks.length; i++) {
        var b = blocks[i];
        out.push({ id: "block-" + b.id, section: "blocks", block: b.id, label: b.label, detail: b.detail,
                   keywords: ["show", "hide", "order", "move"].concat(b.keywords), template: true,
                   control: "toggle", bind: q("block:" + b.id), "default": b.visible,
                   changeLabel: b.label });
        for (var j = 0; j < b.parts.length; j++) {
            var p = b.parts[j];
            out.push({ id: "part-" + p.id, section: "blocks", block: b.id, part: p.id, label: p.label,
                       detail: b.label, keywords: ["show", "hide"].concat(p.keywords), template: true,
                       control: "toggle", bind: q("part:" + p.id), "default": true });
        }
    }
    return out;
}

var rows = blockRows().concat([
    { id: "restore-panel", section: "blocks", label: "Restore the panel", template: true,
      detail: "Every block back in its first order, with all its parts",
      keywords: ["reset", "default", "original"] },

    { id: "panel-design", section: "design", label: "Quick settings design", template: true,
      detail: "Choose how the panel looks: the one that comes with Ryoku, or one from RyoStore",
      keywords: ["design", "look", "skin", "theme", "sidebar design", "style"] },
    { id: "browse-designs", section: "design", label: "Browse RyoStore",
      detail: "Find more designs for Quick settings",
      keywords: ["store", "download", "more designs", "sidebars"], control: "handoff",
      command: ["ryostore", "open", "sidebars"], actionText: "Open" },
    { id: "open-panel", section: "design", label: "Open Quick settings",
      detail: "See the panel as it is now",
      keywords: ["try", "preview", "show the panel"], control: "action",
      command: ["ryoku-shell", "quicksettings"], actionText: "Open" },

    { id: "position", section: "position", label: "Position",
      detail: "Where on the screen the panel opens",
      keywords: ["placement", "corner", "where", "left", "right", "center", "middle"], control: "segmented",
      options: [
          { label: "Top left", value: "top-left" },
          { label: "Top centre", value: "top-center" },
          { label: "Top right", value: "top-right" },
          { label: "Centre", value: "center" }
      ], bind: s("controls.position", "top-left"), "default": "top-left" },
    { id: "edge-gap", section: "position", label: "Edge gap",
      detail: "Space between the panel and the edges of the screen",
      keywords: ["margin", "distance", "gap", "spacing", "offset"], control: "slider", min: 0, max: 96, step: 1,
      unit: "px", bind: s("controls.edgeGap", 12), "default": 12 },

    { id: "show-extensions", section: "cards", label: "Show extension cards",
      detail: "Adds an Extensions page to the panel with the cards your add-ons bring",
      keywords: ["extensions", "plugins", "add-ons", "cards", "plugin cards"], control: "toggle",
      bind: q("part:pluginCards"), "default": true },
    { id: "extension-cards", section: "cards", label: "Order of extension cards", template: true,
      detail: "Move the cards your add-ons bring up or down",
      keywords: ["extensions", "plugins", "add-ons", "cards", "order", "arrange"] },
    { id: "choose-add-ons", section: "cards", label: "Choose which add-ons show a card", template: true,
      detail: "Turn add-ons on or off and choose where each one shows",
      keywords: ["add-ons", "plugins", "install", "placement"] }
]);

var related = ["add-ons", "bar", "notifications", "shortcuts"];

function row(id) {
    for (var i = 0; i < rows.length; i++)
        if (rows[i].id === id)
            return rows[i];
    return null;
}

function block(id) {
    for (var i = 0; i < blocks.length; i++)
        if (blocks[i].id === id)
            return blocks[i];
    return null;
}

function defaultOrder() {
    return blocks.map(function (b) { return b.id; }).join(",");
}

// controls.sections and controls.hidden as the shipped panel has them
function defaultLayout() {
    return JSON.stringify({ sections: blocks.map(function (b) { return { id: b.id, visible: b.visible }; }), hidden: [] });
}

function partIds() {
    var out = ["pluginCards"];
    for (var i = 0; i < blocks.length; i++)
        for (var j = 0; j < blocks[i].parts.length; j++)
            out.push(blocks[i].parts[j].id);
    return out;
}

// what the shell would draw from a stored value: known blocks once each, the
// missing ones appended with their shipped visibility, known hidden parts once
function normalize(sections, hidden) {
    var seen = {}, out = [];
    var incoming = Array.isArray(sections) ? sections : [];
    for (var i = 0; i < incoming.length; i++) {
        var entry = incoming[i];
        var id = typeof entry === "string" ? entry : entry && entry.id;
        if (!block(id) || seen[id])
            continue;
        seen[id] = true;
        out.push({ id: id, visible: typeof entry === "object" && entry.visible !== undefined ? entry.visible === true : true });
    }
    for (var j = 0; j < blocks.length; j++)
        if (!seen[blocks[j].id])
            out.push({ id: blocks[j].id, visible: blocks[j].visible });
    var allowed = partIds(), kept = [], had = {};
    var raw = Array.isArray(hidden) ? hidden : [];
    for (var k = 0; k < raw.length; k++) {
        var p = String(raw[k]);
        if (allowed.indexOf(p) >= 0 && !had[p]) {
            had[p] = true;
            kept.push(p);
        }
    }
    return { sections: out, hidden: kept };
}
