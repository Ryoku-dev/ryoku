.pragma library

var title = "Ethernet";
var lead = "Wired adapters: connect or disconnect a cable, pick the saved profile it uses, and set its IP address.";

var sections = [
    { id: "adapters", title: "Wired adapters" }
];

// the page draws every row itself; these feed search
var rows = [
    { id: "wired-adapters", section: "adapters", label: "Wired adapters",
      detail: "Each adapter's link, speed and address, with Connect or Disconnect",
      keywords: ["ethernet", "cable", "lan", "wired", "network adapter", "usb ethernet", "dock", "link speed", "mac address",
                 "connect", "disconnect", "plug in", "no cable"] },
    { id: "ethernet-profile", section: "adapters", label: "Profile",
      detail: "The saved connection an adapter uses when there are several",
      keywords: ["saved connection", "wired connection", "switch profile"] },
    { id: "ethernet-auto", section: "adapters", label: "Auto-connect",
      detail: "Bring the adapter up whenever a cable is plugged in",
      keywords: ["autoconnect", "auto connect", "join automatically", "plug in"] },
    { id: "ethernet-ip", section: "adapters", label: "IP settings",
      detail: "Automatic (DHCP) or a fixed address, gateway and DNS servers",
      keywords: ["ip address", "static ip", "manual ip", "dhcp", "gateway", "subnet", "netmask", "dns servers", "ipv4",
                 "revert", "keep settings"] }
];

var related = ["wifi", "vpn", "dns"];
