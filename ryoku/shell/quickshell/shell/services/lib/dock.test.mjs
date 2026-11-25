import { createRequire } from "node:module";
const require = createRequire(import.meta.url);
const { pin, unpin, resolve, dockRevealed, classOf } = require("./dock.js");

let failed = 0;
function eq(actual, expected, msg) {
    const a = JSON.stringify(actual);
    const e = JSON.stringify(expected);
    if (a === e) console.log("PASS " + msg);
    else { failed++; console.log("FAIL " + msg + "\n  expected " + e + "\n  got      " + a); }
}

const clients = [
    { className: "firefox", address: "0x1" },
    { className: "kitty", address: "0x2" },
    { className: "firefox", address: "0x3" },
    { className: "steam", address: "0x4" }
];

eq(pin(["firefox"], "firefox"), ["firefox"], "duplicate pins are rejected");
eq(resolve(["kitty", "firefox"], clients), ["kitty", "firefox", "steam"], "pinned classes lead and live unpinned clients follow stably");
eq(resolve(["firefox"], clients), ["firefox", "kitty", "steam"], "live unpinned clients are inserted after pins");
eq(unpin(["firefox", "kitty"], "firefox"), ["kitty"], "unpin removes only the requested class");
eq(resolve(["firefox"], clients).some(value => value.includes("0x")), false, "window addresses are never persisted in dock entries");

const idle = { stageEditing: false, autohide: true, fullscreen: false, pointerInside: false,
    workspaceEmpty: false, anyFocused: true, dragging: false, menuHere: false, popoutHeld: false };
eq(dockRevealed(idle), false, "an idle autohiding dock stays hidden");
eq(dockRevealed({ ...idle, popoutHeld: true }), true,
    "an autohiding dock stays revealed while its preview is held");
eq(dockRevealed({ ...idle, pointerInside: true }), true, "the pointer on the band reveals the dock");
eq(dockRevealed({ ...idle, autohide: false }), true, "a pinned dock is always revealed");
eq(dockRevealed({ ...idle, fullscreen: true, popoutHeld: true }), true,
    "the hold keeps the dock revealed under a fullscreen window too");
eq(dockRevealed({ ...idle, fullscreen: true }), false, "a fullscreen output still hides the idle dock");
eq(dockRevealed({ ...idle, stageEditing: true }), true, "the Stage Editor holds the dock open");

const titleMap = { "Ryoku Settings": "ryoku-hub", "Ryostore": "ryostore", "ryovm": "ryovm", "Ryotunes": "ryotunes" };
eq(classOf("ryoku-hub", "Configuración Ryoku", titleMap), "ryoku-hub",
    "a Quickshell app with a stable app id resolves by that id, whatever its title says");
eq(classOf("ryostore", "Tienda Ryoku", titleMap), "ryostore",
    "a translated title never changes the identity of an app that carries an app id");
eq(classOf("org.quickshell", "Ryotunes", titleMap), "ryotunes",
    "a Quickshell app without an app id falls back to the title map");
eq(classOf("org.quickshell", "Configuración Ryoku", titleMap), "org.quickshell",
    "an unknown Quickshell title keeps the generic id");
eq(classOf("firefox", "Firefox", titleMap), "firefox", "a normal app keeps its own id");
eq(classOf("", "", titleMap), "", "a window with no identity resolves to nothing");

if (failed > 0) { console.log("\n" + failed + " test(s) FAILED"); process.exit(1); }
console.log("\nAll tests PASSED");
