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

const execFor = id => CATALOG.find(action => action.id === id)?.exec;
ok(
    JSON.stringify(execFor("open-clipboard")) === JSON.stringify(["ryoku-shell", "clipboard"]),
    "Clipboard History uses the dedicated clipboard surface"
);
ok(
    JSON.stringify(execFor("open-sysinfo")) === JSON.stringify(["ryoku-shell", "quicksettings"]),
    "System Info opens the left sidebar"
);
ok(
    JSON.stringify(execFor("open-toolkit")) === JSON.stringify(["ryoku-shell", "quicksettings"]),
    "Control Deck opens the left sidebar"
);
ok(
    CATALOG.filter(action => String(action.exec?.[0] || "").startsWith("ryoku-cmd-"))
        .every(action => !String(action.exec[0]).includes("/")),
    "Ryoku command helpers resolve through the packaged PATH"
);

// The catalog fires ryoku-cmd-* helpers by bare name, so they must actually
// reach /usr/bin. The ryoku-shell package ships ryoku/shell/scripts/ryoku-* in
// one glob loop; assert that loop is present and that every helper the catalog
// names is a file that glob will pick up. This is the guard the payload audit
// needed: a helper referenced by bare name but shipped only by the Hyprland
// variant is dead on a niri box.
const repoRoot = new URL("../../../../../../../../../", import.meta.url);
const shellPkgbuild = readFileSync(
    new URL("release/packages/ryoku-shell/PKGBUILD", repoRoot), "utf8");
ok(
    /for s in "\$_repo"\/ryoku\/shell\/scripts\/ryoku-\*/.test(shellPkgbuild),
    "ryoku-shell PKGBUILD installs ryoku/shell/scripts/ryoku-* by glob"
);
const cmdHelpers = [...new Set(
    CATALOG.map(a => String(a.exec?.[0] || "")).filter(n => n.startsWith("ryoku-cmd-"))
)];
const missingHelpers = cmdHelpers.filter(
    name => !existsSync(new URL("ryoku/shell/scripts/" + name, repoRoot)));
ok(
    missingHelpers.length === 0,
    "every ryoku-cmd-* the catalog fires lives in ryoku/shell/scripts so the shell glob ships it: "
        + (missingHelpers.join(", ") || "all present")
);

const providerSource = readFileSync(new URL("./Actions.qml", import.meta.url), "utf8");
ok(
    !providerSource.includes("Config.scriptsDir"),
    "action helpers use PATH instead of the obsolete Hyprland scripts directory"
);

if (failed > 0) { console.log("\n" + failed + " test(s) FAILED"); process.exit(1); }
console.log("\nAll tests PASSED");
