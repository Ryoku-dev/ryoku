import { existsSync, readFileSync } from "node:fs";
import { createRequire } from "node:module";
const require = createRequire(import.meta.url);
const { CATALOG, CATEGORIES, validate } = require("./catalog.js");

let failed = 0;
function ok(cond, msg) {
    if (cond) console.log("PASS " + msg);
    else { failed++; console.log("FAIL " + msg); }
}

ok(CATALOG.length > 0, "catalog is non-empty");
ok(validate(CATALOG).length === 0, "catalog validates clean: " + JSON.stringify(validate(CATALOG)));

// every category tab (except All) has at least one action.
for (let i = 1; i < CATEGORIES.length; i++) {
    const cat = CATEGORIES[i];
    ok(CATALOG.some(a => a.category === cat), "category has actions: " + cat);
}

// validate catches a duplicate id.
ok(validate(CATALOG.concat([CATALOG[0]])).some(p => p.indexOf("duplicate") !== -1), "validate flags duplicate id");
// validate catches a missing exec.
ok(validate([{ id: "x", name: "X", category: "System", exec: [] }]).some(p => p.indexOf("empty exec") !== -1), "validate flags empty exec");
// validate catches an unknown category.
ok(validate([{ id: "y", name: "Y", category: "Nope", exec: ["a"] }]).some(p => p.indexOf("unknown category") !== -1), "validate flags unknown category");
// validate wants exactly one action per entry.
ok(validate([{ id: "z", name: "Z", category: "System" }]).some(p => p.indexOf("exactly one") !== -1), "validate flags an entry with no action");
ok(validate([{ id: "w", name: "W", category: "System", verb: "lock", exec: ["a"] }]).some(p => p.indexOf("exactly one") !== -1), "validate flags an entry with two actions");
ok(validate([{ id: "v", name: "V", category: "System", surface: "" }]).some(p => p.indexOf("empty surface") !== -1), "validate flags an empty surface");
// daemon verbs ride the bus rather than a spawned client.
ok(validate([{ id: "u", name: "U", category: "System", exec: ["ryoku-shell", "lock"] }]).some(p => p.indexOf("instead of a verb") !== -1), "validate flags a spawned ryoku-shell");

const entry = id => CATALOG.find(action => action.id === id) || {};
ok(entry("open-clipboard").surface === "clipboard", "Clipboard History uses the dedicated clipboard surface");
ok(entry("open-sysinfo").surface === "sidebar-left", "System Info opens the left sidebar");
ok(entry("open-toolkit").surface === "sidebar-left", "Control Deck opens the left sidebar");
ok(entry("media-visualizer").surface === "visualizer", "Audio Visualizer toggles the visualizer surface");
ok(entry("lock-screen").verb === "lock", "Lock Screen asks the daemon to lock");
ok(entry("screen-record").verb === "record", "Screen Record asks the daemon to record");
ok(entry("reload-shell").verb === "reload", "Reload Shell asks the daemon to reload");
ok(
    CATALOG.filter(action => String(action.exec?.[0] || "").startsWith("ryoku-cmd-"))
        .every(action => !String(action.exec[0]).includes("/")),
    "Ryoku command helpers resolve through the packaged PATH"
);

// Helpers run by bare name must live in the shared shell scripts tree rather
// than a compositor-specific directory.
const repoRoot = new URL("../../../../../../../../../", import.meta.url);
const cmdHelpers = [...new Set(
    CATALOG.map(a => String(a.exec?.[0] || "")).filter(n => n.startsWith("ryoku-cmd-"))
)];
const missingHelpers = cmdHelpers.filter(
    name => !existsSync(new URL("ryoku/shell/scripts/" + name, repoRoot)));
ok(
    missingHelpers.length === 0,
    "every ryoku-cmd-* the catalog fires is shared by every compositor: "
        + (missingHelpers.join(", ") || "all present")
);

const providerSource = readFileSync(new URL("./Actions.qml", import.meta.url), "utf8");
ok(
    !providerSource.includes("Config.scriptsDir"),
    "action helpers use PATH instead of the obsolete Hyprland scripts directory"
);

if (failed > 0) { console.log("\n" + failed + " test(s) FAILED"); process.exit(1); }
console.log("\nAll tests PASSED");
