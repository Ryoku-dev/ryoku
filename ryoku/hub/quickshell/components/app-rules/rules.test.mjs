import fs from "node:fs";
import vm from "node:vm";
import path from "node:path";
import { fileURLToPath } from "node:url";

// load the schema and rules.js the way the QML engine links them
const here = path.dirname(fileURLToPath(import.meta.url));
function load(file, ctx) {
    const src = fs.readFileSync(path.join(here, file), "utf8").replace(/^\.(pragma|import) .*$/gm, "");
    vm.runInContext(src, ctx);
    return ctx;
}
const Schema = load("../../schema/app-rules.js", vm.createContext({}));
const R = load("rules.js", vm.createContext({ Schema }));

let failed = 0;
function eq(actual, expected, msg) {
    const a = JSON.stringify(actual);
    const e = JSON.stringify(expected);
    if (a === e) {
        console.log("PASS " + msg);
    } else {
        failed++;
        console.log("FAIL " + msg + "\n  expected " + e + "\n  got      " + a);
    }
}

// each provider's caps: windowRuleActions and appOverrideFields
const hypr = R.vocabulary(["float", "tile", "pin", "fullscreen", "maximize", "center", "pseudo", "keepaspectratio",
                           "size", "move", "workspace", "norounding", "noborder", "opacity", "opaque", "noblur",
                           "noshadow", "nodim", "noanim", "xray", "nofocus", "stayfocused", "suppressevent",
                           "idleinhibit", "immediate"],
                          ["opacity", "rounding", "borderSize", "blur", "shadow", "dim", "anim", "opaque"]);
const niri = R.vocabulary(["float", "tile", "fullscreen", "maximize", "norounding", "opacity", "workspace", "noborder",
                           "noshadow", "blur", "noblur", "xray", "columnwidth", "minsize", "maxsize", "scrollfactor",
                           "tiledstate", "babaisfloat", "blockout"],
                          ["opacity", "rounding", "borderSize", "blur"]);
const wayfire = R.vocabulary(["pin", "maximize", "opacity", "workspace"], ["opacity"]);

const wr = [
    { class: "firefox", title: "", action: "float", value: "" },
    { class: "mpv", title: "", action: "opacity", value: "0.8" },
    { class: "firefox", title: "", action: "size", value: "1200x800" },
    { class: "", title: "Picture-in-Picture", action: "pin", value: "" },
    { class: "steam", title: "", action: "idleinhibit", value: "focus" },
    { class: "firefox", title: "", action: "dimaround", value: "" }
];
const ao = [
    { class: "kitty", title: "", opacity: 0.95, rounding: 6, borderSize: -1, blur: "off", shadow: "inherit", dim: "inherit", anim: "inherit", opaque: "inherit" },
    { class: "firefox", title: "", opacity: -1, rounding: -1, borderSize: -1, blur: "inherit", shadow: "off", dim: "inherit", anim: "inherit", opaque: "inherit" }
];

const list = R.grouped(wr, ao);
eq(list.map(g => [g.cls, g.title]), [["firefox", ""], ["mpv", ""], ["", "Picture-in-Picture"], ["steam", ""], ["kitty", ""]],
   "records group by app in store order");
eq(list[0].rules, [0, 2, 5], "an app gathers all its window-rule records");
eq(list[0].override, 1, "and its look override");

const fx = R.itemOf(list[0], wr, ao, hypr);
eq(fx.values, { mode: "float", size: "1200x800", shadow: true }, "a rule reads its actions and its look");
eq(fx.extras.map(r => r.action), ["dimaround"], "an action not offered here is kept aside");

const mpv = R.itemOf(list[1], wr, ao, hypr);
eq(mpv.values, { opacity: 0.8 }, "an opacity action reads as the opacity amount");

eq(R.itemOf(list[3], wr, ao, hypr).values, { idleinhibit: "focus" }, "an enumerated value reads back");
eq(R.itemOf(list[2], wr, ao, niri).extras.map(r => r.action), ["pin"], "pin is kept aside where it cannot apply");

// shadow is an action on this window manager, a look field where none is listed
const shadowed = R.recordsOf({ cls: "foot", title: "", values: { shadow: true } }, hypr);
eq(shadowed.rules.map(r => r.action), ["noshadow"], "a listed look action is written as a rule");
eq(shadowed.override, null, "with no look override");
const shadowedLook = R.recordsOf({ cls: "foot", title: "", values: { shadow: true } }, R.vocabulary(["float"], ["shadow"]));
eq([shadowedLook.rules.length, shadowedLook.override.shadow], [0, "off"], "otherwise as the look field");

// a look the window manager does not write is not offered, and a stored one stays for the one that does
const ids = wm => R.offered(wm).map(p => p.id);
eq(["dim", "anim", "opaque"].filter(id => ids(niri).indexOf(id) >= 0), [], "niri offers no dim, animation or opaque switch");
eq(["shadow", "rounding", "borderSize", "blur"].filter(id => ids(niri).indexOf(id) < 0), [], "but its shadow action and its looks");
eq(["rounding", "borderSize", "blur", "shadow", "dim"].filter(id => ids(wayfire).indexOf(id) >= 0), [], "wayfire offers opacity alone");
eq(R.optionsOf(R.props.find(p => p.id === "blur"), hypr).map(o => o.value), ["off"], "blur can only be turned off on hyprland");
eq(["dim", "anim", "opaque", "rounding"].filter(id => ids(R.vocabulary([], [])).indexOf(id) < 0), [], "unknown looks are all offered");
const dimmed = [{ class: "foot", title: "", opacity: -1, rounding: -1, borderSize: -1, blur: "inherit", shadow: "inherit", dim: "off", anim: "inherit", opaque: "inherit" }];
const kept = R.withRule([], dimmed, 0, Object.assign(R.itemOf(R.grouped([], dimmed)[0], [], dimmed, niri), { values: { rounding: 4 } }), niri);
eq([kept.appOverrides[0].dim, kept.appOverrides[0].rounding], ["off", 4], "an edit on niri keeps the stored dim");

// editing firefox moves its old shadow override into the noshadow action and keeps the rest in place
const edited = Object.assign({}, fx, { values: Object.assign({}, fx.values, { rounding: 0 }) });
const out = R.withRule(wr, ao, 0, edited, hypr);
eq(out.windowRules.map(r => (r.class || r.title) + ":" + r.action),
   ["firefox:float", "firefox:size", "firefox:noshadow", "firefox:dimaround", "mpv:opacity", "Picture-in-Picture:pin", "steam:idleinhibit"],
   "an edited rule's records take the place of its first record");
eq(out.appOverrides.map(a => [a.class, a.rounding, a.shadow]), [["kitty", 6, "inherit"], ["firefox", 0, "inherit"]],
   "its look override stays where it was");
eq(R.grouped(out.windowRules, out.appOverrides).length, 5, "the edit adds no rule");

// mpv's opacity action becomes a look override
const mpvOut = R.withRule(wr, ao, 1, Object.assign({}, mpv, { values: { opacity: 0.7 } }), hypr);
eq(mpvOut.windowRules.some(r => r.class === "mpv"), false, "the old opacity action is gone");
eq(mpvOut.appOverrides[2].opacity, 0.7, "the opacity lives in the look override");

// adding and removing
const added = R.withRule(wr, ao, -1, { cls: "obs", title: "", values: { blockout: true, immediate: true } }, hypr);
eq(added.windowRules.slice(-1).map(r => r.action), ["immediate"], "a new rule appends its records");
eq(R.withoutRule(wr, ao, 0).windowRules.some(r => r.class === "firefox"), false, "removing drops every record");
eq(R.withoutRule(wr, ao, 0).appOverrides.map(a => a.class), ["kitty"], "and its look override");

// round trip: what is written reads back the same
const item = { cls: "gimp", title: "", values: { mode: "float", center: true, opacity: 0.9, rounding: 12, blur: "off", dim: true, anim: true, opaque: true, suppressevent: "activate" } };
const rt = R.withRule([], [], -1, item, hypr);
const back = R.itemOf(R.grouped(rt.windowRules, rt.appOverrides)[0], rt.windowRules, rt.appOverrides, hypr);
eq(back.values, item.values, "every window-rule property survives a round trip");

eq(R.problem(R.props.find(p => p.id === "size"), "1200x800"), "", "a size is checked");
eq(R.problem(R.props.find(p => p.id === "size"), "1200"), "size", "half a size is not");
eq(R.problem(R.props.find(p => p.id === "columnwidth"), "1.5"), "fraction", "column width stays within the display");
eq(R.phrases(fx, hypr, s => s, "%1, unused"), ["Floating", "1200x800", "No shadow", "dimaround, unused"],
   "a rule reads as short phrases");
eq(R.joined(["80% opacity", "Keeps the screen awake", "PiP stays", "Floating"]),
   "80% opacity, keeps the screen awake, PiP stays, floating", "the line reads as one sentence");
eq(R.ruleOfStoreKey(list, "desktop.appOverrides[0]"), 4, "an unhonored override names its rule");
eq(R.ruleOfStoreKey(list, "desktop.windowRules[2]"), 0, "an unhonored window rule names its rule");

// shell layers, both provider shapes
const hyprRows = [
    { key: "wm.hyprland.layerRules", ctl: "list", page: "layerrules" },
    { key: "wm.hyprland.layerRules[i].action", ctl: "chips", opts: ["blur", "blurpopups", "ignorealpha", "noanim", "dimaround", "xray", "abovelock"] }
];
const hs = R.layerSpec(hyprRows);
eq([hs.key, hs.shape, hs.props.length], ["wm.hyprland.layerRules", "actions", 7], "an action-shaped layer list");
const hl = [{ namespace: "launcher", action: "blur", value: "" }, { namespace: "bar", action: "noanim", value: "" },
            { namespace: "launcher", action: "ignorealpha", value: "0.3" }];
const hi = R.layerItems(hs, hl);
eq(hi.map(i => [i.namespace, i.values]), [["launcher", { blur: true, ignorealpha: 0.3 }], ["bar", { noanim: true }]],
   "layer records group by namespace");
const hlOut = R.withLayer(hs, hl, 0, { namespace: "launcher", values: { blur: true, dimaround: true } });
eq(hlOut.map(r => r.namespace + ":" + r.action), ["launcher:blur", "launcher:dimaround", "bar:noanim"], "a layer edit keeps its place");
eq(R.withLayer(hs, hl, 1, null).map(r => r.namespace), ["launcher", "launcher"], "a layer rule is removed");

const niriRows = [{ key: "wm.niri.layerRules", ctl: "list", fields: [
    { name: "namespace", ctl: "text" }, { name: "opacity", ctl: "slid", lo: 0, hi: 1, def: -1 },
    { name: "cornerRadius", ctl: "step", lo: -1, hi: 64 }, { name: "blur", ctl: "seg", opts: ["inherit", "on", "off"] },
    { name: "shadow", ctl: "seg", opts: ["inherit", "on", "off"] }, { name: "blockOut", ctl: "sw" }, { name: "babaIsFloat", ctl: "sw" }] }];
const ns = R.layerSpec(niriRows);
eq([ns.key, ns.shape, ns.props.map(p => p.kind)], ["wm.niri.layerRules", "fields", ["amount", "amount", "tri", "tri", "flag", "flag"]],
   "a field-shaped layer list");
const nrec = R.layerRecords(ns, { namespace: "ryoku-bar", values: { cornerRadius: 12, blur: "on", blockOut: true } });
eq(nrec, [{ namespace: "ryoku-bar", opacity: -1, cornerRadius: 12, blur: "on", shadow: "inherit", blockOut: true, babaIsFloat: false }],
   "unset fields write their leave-alone values");
eq(R.layerItems(ns, nrec)[0].values, { cornerRadius: 12, blur: "on", blockOut: true }, "and read back");

// the store's tagged list: key order does not matter, each half comes back alone
const t1 = R.tagged([{ value: "", class: "mpv", action: "float", title: "" }], [{ opacity: 0.8, class: "mpv", title: "" }]);
const t2 = R.tagged([{ action: "float", class: "mpv", title: "", value: "" }], [{ class: "mpv", opacity: 0.8, title: "" }]);
eq(JSON.stringify(t1) === JSON.stringify(t2), true, "a record read back in another key order is the same record");
eq(R.untagged(t1, "appOverrides"), [{ class: "mpv", opacity: 0.8, title: "" }], "each half comes back alone");
eq(R.untagged(t1, "windowRules").length, 1, "and the other half too");

if (failed > 0) {
    console.log(failed + " failed");
    process.exit(1);
}
