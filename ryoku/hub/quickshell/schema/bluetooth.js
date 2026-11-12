.pragma library

var title = "Bluetooth";
var lead = "Turn Bluetooth on or off, and pair, connect and disconnect your devices.";

var sections = [
    { id: "radio", title: "Bluetooth" },
    { id: "devices", title: "Devices" }
];

// the page draws every row itself; these feed search and name the store key
var rows = [
    { id: "bluetooth-on", section: "radio", label: "Bluetooth",
      detail: "Turn the Bluetooth radio on or off",
      keywords: ["radio", "adapter", "bluez", "rfkill", "blocked", "service", "start service", "airplane mode"],
      control: "toggle", bind: { store: "bluetooth", key: "power" } },
    { id: "devices", section: "devices", label: "Devices",
      detail: "Pair a new device, or connect and disconnect the ones you have",
      keywords: ["pair", "connect", "disconnect", "scan", "discover", "nearby", "headphones", "earbuds", "headset",
                 "speaker", "mouse", "keyboard", "controller", "gamepad", "battery"] }
];

var related = ["wifi", "phone"];
