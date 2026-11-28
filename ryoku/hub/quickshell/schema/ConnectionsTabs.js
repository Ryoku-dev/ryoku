.pragma library

// The Connections page's tab strip, shared with the Hub's global search so a
// search hit and the panel it opens can never name different tabs. `desc` and
// `kw` are search vocabulary, not UI copy: they let "wireguard", "encrypted
// DNS" or "pair a device" reach the panel that owns them.
var tabs = [
    { key: "wifi", label: "Wi-Fi", desc: "Networks, saved connections, IP settings and the kill switch",
      kw: "wifi wi-fi wireless network ssid password saved hidden ip address mac random kill switch" },
    { key: "ethernet", label: "Ethernet", desc: "Wired connections and their IP settings",
      kw: "ethernet cable wired lan link ip address dhcp" },
    { key: "bluetooth", label: "Bluetooth", desc: "Adapter power, pairing and connected devices",
      kw: "bluetooth bt pair pairing device adapter discover connect" },
    { key: "hotspot", label: "Hotspot", desc: "Share this connection over Wi-Fi",
      kw: "hotspot tether share ssid network name password wpa" },
    { key: "vpn", label: "VPN", desc: "Add, edit and connect VPN profiles",
      kw: "vpn wireguard openvpn profile tunnel import connection" },
    { key: "dns", label: "DNS", desc: "DNS servers, encrypted DNS and per-connection overrides",
      kw: "dns server resolver doh dot encrypted over tls override" },
    { key: "phone", label: "Phone", desc: "Connect an Android phone for mirroring, clipboard and files",
      kw: "phone android scrcpy mirror clipboard usb wireless adb" }
];
