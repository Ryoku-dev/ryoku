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

const rows = networks(scan, "", "");
eq(rows.map(r => r.ssid + ":" + r.security), ["Home:Wpa2", "Cafe:Wpa2", "Mesh:Wpa2", "Cafe:None"],
    "one row per name and security kind, saved first, then by signal");
eq(rows[0].bandLabel, "2.4 · 5 GHz", "a dual-band network lists both bands once");
eq(rows[0].bssid, "a2", "the strongest access point speaks for the row");
eq(rows.find(r => r.ssid === "Mesh").bandLabel, "5 · 6 GHz", "bands sort numerically");
eq(rows.find(r => r.ssid === "Mesh").security, "Wpa2", "Wpa2 and Wpa3 radios of one network collapse; strongest reports security");

eq(networks(scan, "Home", "Wpa2").map(r => r.ssid), ["Cafe", "Mesh", "Cafe"],
    "the connected network leaves the list on every band");
eq(networks(scan, "Cafe", "None").map(r => r.ssid + ":" + r.security), ["Home:Wpa2", "Cafe:Wpa2", "Mesh:Wpa2"],
    "an open network of the same name is a different row");

const jitter = [ap("A", "1", "5", 61), ap("B", "2", "5", 64)];
const jittered = [ap("A", "1", "5", 66), ap("B", "2", "5", 62)];
eq(networks(jitter, "", "").map(r => r.ssid), networks(jittered, "", "").map(r => r.ssid),
    "a small signal change does not reorder rows");

eq(networks([ap("", "x1", "5", 50), ap("", "x2", "5", 49)], "", "").length, 2,
    "hidden networks are not merged");

const live = [ap("Home", "a1", "2.4", 60), ap("Home", "a2", "5", 80, { active: true })];
eq(activeBand(live), "5", "active band comes from the connected access point");
eq(activeSecurity(live), "Wpa2", "active security comes from the connected access point");
eq(activeBand([ap("Home", "a1", "2.4", 60)]), "", "no active access point, no band");

if (failed > 0) { console.log(failed + " failed"); process.exit(1); }
console.log("ok");
