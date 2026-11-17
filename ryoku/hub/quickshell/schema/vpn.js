.pragma library

var title = "VPN";
var lead = "WireGuard tunnels and the VPN profiles NetworkManager holds: switch them on or off, import a tunnel, edit it.";

var sections = [
    { id: "wireguard", title: "WireGuard" },
    { id: "profiles", title: "VPN profiles" }
];

// the page draws every row itself; these feed search
var rows = [
    { id: "wireguard-import", section: "wireguard", label: "Add a tunnel",
      detail: "Import the .conf file your VPN provider gives you",
      keywords: ["wireguard", "import", "conf", "config file", "wg", "add vpn", "new tunnel"] },
    { id: "wireguard-tunnels", section: "wireguard", label: "Tunnels",
      detail: "Switch a tunnel on or off, open its settings, edit its peer, address and DNS, remove it",
      keywords: ["wireguard", "wg", "connect", "disconnect", "endpoint", "allowed ips", "keepalive", "peer", "traffic",
                 "private key", "export", "open conf", "load changes", "remove tunnel", "delete", "auto-connect", "autoconnect",
                 "interface", "address", "dns"] },
    { id: "vpn-profiles", section: "profiles", label: "VPN profiles",
      detail: "OpenVPN and other profiles added through NetworkManager",
      keywords: ["openvpn", "openconnect", "ipsec", "l2tp", "pptp", "vpn plugin", "connect", "disconnect", "auto-connect",
                 "autoconnect", "server", "networkmanager"] }
];

var related = ["wifi", "dns"];
