import { createRequire } from "node:module";
const require = createRequire(import.meta.url);
const { networks, activeBand, activeSecurity } = require("./wifinets.js");

let failed = 0;
function eq(actual, expected, msg) {
    const a = JSON.stringify(actual);
    const e = JSON.stringify(expected);
    if (a === e) console.log("PASS " + msg);
    else { failed++; console.log("FAIL " + msg + "\n  expected " + e + "\n  got      " + a); }
}

const ap = (ssid, bssid, band, strength, extra) =>
    Object.assign({ ssid, bssid, band, strength, security: "Wpa2", saved: false, active: false }, extra);

const scan = [
    ap("Home", "a1", "2.4", 60, { saved: true }),
    ap("Home", "a2", "5", 80),
    ap("Home", "a3", "5", 70),
    ap("Cafe", "b1", "2.4", 75),
    ap("Cafe", "b2", "2.4", 20, { security: "None" }),
    ap("Mesh", "c1", "6", 50),
    ap("Mesh", "c2", "5", 45, { security: "Wpa3" }),
];

const rows = networks(scan, "", "", "");
eq(rows.map(r => r.ssid + ":" + r.security + ":" + r.band),
    ["Home:Wpa2:2.4", "Home:Wpa2:5", "Cafe:Wpa2:2.4", "Mesh:Wpa2:6", "Mesh:Wpa3:5", "Cafe:None:2.4"],
    "one row per name, security kind and band, saved first, then by signal");
eq(rows[0].bandLabel, "2.4 GHz", "each row carries its band");
eq(rows.find(r => r.ssid === "Home" && r.band === "5").bssid, "a2", "the strongest access point of the band speaks for the row");
eq(rows.filter(r => r.ssid === "Home" && r.band === "5").length, 1, "mesh access points of one band collapse");
eq(rows.find(r => r.ssid === "Mesh" && r.band === "5").security, "Wpa3", "each band reports its own security");
eq(rows.map(r => r.saved).join(), "true,false,false,false,false,false", "saved rows sort first");

eq(networks(scan, "Home", "Wpa2", "5").map(r => r.ssid + ":" + r.band).slice(0, 2), ["Home:2.4", "Cafe:2.4"],
    "only the connected band leaves the list, the other band stays pickable");
eq(networks(scan, "Home", "Wpa2", "").filter(r => r.ssid === "Home").length, 0,
    "an unknown connected band hides every band of that network");
eq(networks(scan, "Cafe", "None", "2.4").map(r => r.ssid + ":" + r.security).filter(x => x.startsWith("Cafe")),
    ["Cafe:Wpa2"], "an open network of the same name is a different row");

const jitter = [ap("A", "1", "5", 61), ap("B", "2", "5", 64)];
const jittered = [ap("A", "1", "5", 66), ap("B", "2", "5", 62)];
eq(networks(jitter, "", "", "").map(r => r.ssid), networks(jittered, "", "", "").map(r => r.ssid),
    "a small signal change does not reorder rows");

eq(networks([ap("", "x1", "5", 50), ap("", "x2", "5", 49)], "", "", "").length, 2,
    "hidden networks are not merged");

const live = [ap("Home", "a1", "2.4", 60), ap("Home", "a2", "5", 80, { active: true })];
eq(activeBand(live), "5", "active band comes from the connected access point");
eq(activeSecurity(live), "Wpa2", "active security comes from the connected access point");
eq(activeBand([ap("Home", "a1", "2.4", 60)]), "", "no active access point, no band");

if (failed > 0) { console.log(failed + " failed"); process.exit(1); }
console.log("ok");
