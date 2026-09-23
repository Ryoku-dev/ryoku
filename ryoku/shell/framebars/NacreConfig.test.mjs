import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const Nacre = require("./NacreConfig.js");

let failed = 0;

function ok(value, message) {
    if (!value) {
        console.error(`FAIL: ${message}`);
        failed++;
    }
}

function eq(actual, expected, message) {
    ok(JSON.stringify(actual) === JSON.stringify(expected),
        `${message}; got ${JSON.stringify(actual)}, want ${JSON.stringify(expected)}`);
}

const defaults = Nacre.defaultConfig();
eq(Nacre.normalize(null), defaults, "missing config restores defaults");
eq(defaults.workspaceStyle, "dots", "workspace style defaults to dots");
eq(
    Nacre.normalize({ workspaceStyle: "numbers" }).workspaceStyle,
    "numbers",
    "number workspaces survive normalization"
);
eq(
    Nacre.normalize({ workspaceStyle: "kanji" }).workspaceStyle,
    "kanji",
    "kanji workspaces survive normalization"
);
eq(
    Nacre.normalize({ workspaceStyle: "letters" }).workspaceStyle,
    "dots",
    "unknown workspace styles restore dots"
);
eq(defaults.brandClick, "launcher", "brand click defaults to the launcher");
eq(
    Nacre.normalize({ brandClick: "quicksettings" }).brandClick,
    "quicksettings",
    "a quicksettings brand click survives normalization"
);
eq(
    Nacre.normalize({ brandClick: "menu" }).brandClick,
    "launcher",
    "an unknown brand click restores the launcher"
);
eq(
    Nacre.setValue(defaults, "brandClick", "quicksettings").brandClick,
    "quicksettings",
    "the brand click can be staged"
);
eq(defaults.frame, true, "frame defaults on");
eq(defaults.frameSize, 9, "frame size matches main");
eq(defaults.frameRoundness, 9, "frame roundness matches main");
eq(defaults.edgeMelt, 8, "edge melt matches main");
eq(defaults.islandScale, 1, "island size defaults to full scale");
eq(defaults.osdScale, 1, "OSD size defaults to full scale");
eq(Nacre.entry("workspaces"), { id: "workspaces", label: "Workspaces", file: "Workspaces.qml" }, "catalog resolves widget metadata");

const normalized = Nacre.normalize({
    islands: {
        left: ["brand", "bogus", "media", "brand"],
        center: ["media", "clock"],
        right: []
    },
    height: 100,
    opacity: 0,
    padding: 2,
    spacing: 40,
    islandGap: -2,
    frameSize: 80,
    frameRoundness: 80,
    edgeMelt: -4,
    islandScale: 0.1,
    osdScale: 4,
    frame: false,
    occupiedWorkspaces: false
});
eq(normalized.islands.left, ["brand", "media"], "normalization removes unknown and duplicate ids");
eq(normalized.islands.center, ["clock"], "normalization removes cross-island duplicates");
eq(normalized.islands.right, [], "normalization preserves an empty island");
eq(
    [
        normalized.height, normalized.opacity, normalized.padding,
        normalized.spacing, normalized.islandGap,
        normalized.frameSize, normalized.frameRoundness, normalized.edgeMelt,
        normalized.islandScale, normalized.osdScale
    ],
    [56, 0.45, 6, 18, 6, 24, 32, 1, 0.65, 1.25],
    "normalization clamps appearance values"
);
eq(normalized.occupiedWorkspaces, false, "normalization preserves a boolean workspace mode");
eq(normalized.frame, false, "normalization preserves the frame toggle");

const partial = Nacre.normalize({
    islands: { left: ["weather"] },
    opacity: 0.7
});
eq(partial.islands.left, ["weather"], "normalization preserves a supplied island");
eq(partial.islands.center, defaults.islands.center, "normalization restores a missing island");
eq(partial.opacity, 0.7, "normalization preserves a valid appearance value");
eq(partial.height, defaults.height, "normalization restores a missing appearance value");

const moved = Nacre.move(defaults, "brand", "left", "right", 1);
eq(moved.islands.left, ["media", "activeWindow"], "cross-island move removes the source");
eq(
    moved.islands.right,
    ["connectivity", "brand", "audio", "battery", "notifications", "tray"],
    "cross-island move inserts at the target"
);
eq(defaults.islands.left, ["brand", "media", "activeWindow"], "move leaves its input unchanged");
eq(moved.opacity, 0.82, "move preserves appearance settings");

const reordered = Nacre.move(defaults, "brand", "left", "left", 3);
eq(reordered.islands.left, ["media", "activeWindow", "brand"], "same-island move adjusts its target after removal");

const removed = Nacre.remove(defaults, "media");
eq(removed.islands.left, ["brand", "activeWindow"], "remove returns a widget to the palette");
eq(Nacre.unused(removed), ["media", "weather", "utils"], "unused widgets follow catalog order");

const restored = Nacre.move(removed, "media", "", "center", 1);
eq(restored.islands.center, ["clock", "media", "workspaces", "resources"], "palette move inserts an unused widget");

eq(Nacre.move(defaults, "media", "", "right", 0), defaults, "palette move rejects a placed widget");
eq(Nacre.move(defaults, "media", "right", "left", 0), defaults, "move rejects the wrong source island");
eq(Nacre.move(defaults, "bogus", "", "left", 0), defaults, "move rejects an unknown widget");
eq(Nacre.move(defaults, "brand", "left", "bogus", 0), defaults, "move rejects an unknown island");

const resized = Nacre.setValue(defaults, "height", 47.8);
eq(resized.height, 48, "integer appearance values are rounded");
eq(Nacre.setValue(defaults, "frameSize", 14).frameSize, 14, "frame size can be changed");
eq(Nacre.setValue(defaults, "frameRoundness", 18).frameRoundness, 18, "frame roundness can be changed");
eq(Nacre.setValue(defaults, "edgeMelt", 20).edgeMelt, 20, "edge melt can be changed");
eq(Nacre.setValue(defaults, "islandScale", 0.8).islandScale, 0.8, "island size can be changed");
eq(Nacre.setValue(defaults, "osdScale", 0.75).osdScale, 0.75, "OSD size can be changed");
eq(Nacre.setValue(defaults, "occupiedWorkspaces", false).occupiedWorkspaces, false, "workspace mode can be changed");
eq(Nacre.setValue(defaults, "workspaceStyle", "kanji").workspaceStyle, "kanji", "workspace style can be staged");
eq(Nacre.setValue(defaults, "frame", false).frame, false, "frame can be changed");
eq(Nacre.setValue(defaults, "unknown", 4), defaults, "unknown appearance keys are ignored");

if (failed > 0) {
    console.error(`\n${failed} test(s) FAILED`);
    process.exit(1);
}

console.log("\nAll tests PASSED");
