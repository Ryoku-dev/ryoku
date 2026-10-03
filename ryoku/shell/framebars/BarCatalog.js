const entries = [
    ["audio-input", "Audio Input", ["horizontal", "vertical"], "widget", null],
    ["audio-output", "Audio Output", ["horizontal", "vertical"], "widget", null],
    ["battery", "Battery", ["horizontal", "vertical"], "widget", null],
    ["bluetooth", "Bluetooth", ["horizontal", "vertical"], "widget", null],
    ["clock", "Clock", ["horizontal", "vertical"], "widget", null],
    ["dock", "Dock", ["vertical"], "widget", null],
    ["workspaces", "Workspaces", ["horizontal", "vertical"], "widget", null],
    ["lock", "Lock", ["horizontal", "vertical"], "quick-action", "lock"],
    ["logout", "Log Out", ["horizontal", "vertical"], "quick-action", "logout"],
    ["music", "Music", ["horizontal", "vertical"], "widget", null],
    ["network", "Network", ["horizontal", "vertical"], "widget", null],
    ["notifications", "Notifications", ["horizontal", "vertical"], "widget", null],
    ["quick-settings", "Quick Settings", ["horizontal", "vertical"], "widget", null],
    ["sysmon", "System Monitor", ["horizontal", "vertical"], "widget", null],
    ["shutdown", "Shut Down", ["horizontal", "vertical"], "quick-action", "shutdown"],
    ["tray", "Tray", ["horizontal", "vertical"], "widget", null],
    ["vpn", "VPN", ["horizontal", "vertical"], "widget", null]
];

const byId = {};
for (const [id, label, axes, kind, menuId] of entries) byId[id] = { id, label, axes, kind, menuId };

function ids() { return entries.map(entry => entry[0]); }
function entry(id) { return byId[id] || null; }

if (typeof module !== "undefined" && module.exports) module.exports = { ids, entry };
