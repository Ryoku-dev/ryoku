.pragma library

var title = "Wi-Fi & internet";
var lead = "Join Wi-Fi networks, forget the ones you no longer use, and share this computer's connection.";

var sections = [
    { id: "wifi", title: "Wi-Fi" },
    { id: "saved", title: "Saved networks" },
    { id: "hotspot", title: "Hotspot" }
];

function w(key) {
    return { store: "wifi", key: key };
}

// the page draws every row itself; these feed search and name the store keys
var rows = [
    { id: "wifi-on", section: "wifi", label: "Wi-Fi",
      detail: "Turn the Wi-Fi radio on or off",
      keywords: ["wireless", "wlan", "radio", "airplane mode", "flight mode"],
      control: "toggle", bind: w("enabled") },
    { id: "networks", section: "wifi", label: "Networks nearby",
      detail: "Connect to a network, with its password when it asks for one",
      keywords: ["connect", "join", "password", "ssid", "scan", "rescan", "search", "signal", "disconnect",
                 "band", "5 GHz", "2.4 GHz", "6 GHz", "wpa", "secured", "open network"] },
    { id: "wifi-backend", section: "wifi", label: "Wi-Fi backend", more: true,
      detail: "Try the other one when a WPA3 network or a phone's hotspot will not connect",
      keywords: ["iwd", "wpa_supplicant", "wpa supplicant", "supplicant", "wpa3", "networkmanager", "android hotspot", "driver"],
      control: "segmented", options: [
          { label: "iwd", value: "iwd" }, { label: "wpa_supplicant", value: "wpa_supplicant" }
      ], bind: w("backend"), "default": "iwd" },
    { id: "block-all", section: "wifi", label: "Block all connections", more: true,
      detail: "Cuts Wi-Fi, Ethernet, VPN and the local network, remote sessions too",
      keywords: ["kill switch", "isolate", "offline", "firewall", "nftables", "ssh", "airplane mode", "ethernet", "ipv6"] },

    { id: "saved", section: "saved", label: "Saved networks",
      detail: "Networks this computer joins, and Forget for the ones you no longer use",
      keywords: ["forget", "remove", "delete", "known networks", "remembered", "profiles", "auto-reconnect", "auto reconnect",
                 "reconnect", "autoconnect", "join automatically"] },

    { id: "hotspot-on", section: "hotspot", label: "Hotspot",
      detail: "Share this computer's connection over Wi-Fi",
      keywords: ["tethering", "share internet", "access point", "ap", "mobile hotspot"],
      control: "toggle", bind: w("hotspot.on") },
    { id: "hotspot-name", section: "hotspot", label: "Hotspot name",
      detail: "The network name other devices see",
      keywords: ["ssid", "network name"], control: "text", bind: w("hotspot.name") },
    { id: "hotspot-password", section: "hotspot", label: "Hotspot password",
      detail: "At least 8 characters",
      keywords: ["key", "wpa2", "psk", "passphrase"], control: "text", bind: w("hotspot.password") }
];

var related = ["bluetooth", "phone"];
