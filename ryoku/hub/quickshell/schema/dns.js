.pragma library

var title = "DNS";
var lead = "Which servers this computer asks to look up names: the network's own, a public resolver, or your own.";

var sections = [
    { id: "resolver", title: "Resolver" }
];

// the page draws every row itself; these feed search
var rows = [
    { id: "dns-provider", section: "resolver", label: "DNS provider",
      detail: "DHCP uses the servers the network hands out",
      keywords: ["dns", "resolver", "cloudflare", "google", "1.1.1.1", "8.8.8.8", "dhcp", "custom", "nameserver",
                 "name server", "ryoku-dns", "resolved", "private dns"] },
    { id: "dns-custom", section: "resolver", label: "Custom servers",
      detail: "IPv4 or IPv6 addresses, separated by spaces or commas",
      keywords: ["custom dns", "your own dns", "quad9", "9.9.9.9", "ipv6", "servers", "nameserver"] },
    { id: "dns-in-use", section: "resolver", label: "In use",
      detail: "The servers names are looked up with right now",
      keywords: ["current dns", "active dns", "which dns"] }
];

var related = ["wifi", "vpn"];
