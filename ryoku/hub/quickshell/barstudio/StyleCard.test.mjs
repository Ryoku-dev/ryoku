import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const StyleCard = require("./StyleCard.js");

let failed = 0;
function ok(value, message) {
    if (!value) {
        console.error(`FAIL: ${message}`);
        failed++;
    }
}
function eq(actual, expected, message) {
    ok(JSON.stringify(actual) === JSON.stringify(expected), `${message}; got ${JSON.stringify(actual)}, want ${JSON.stringify(expected)}`);
}
function rejects(file, ctx, why, message) {
    const parsed = StyleCard.parse(typeof file === "string" ? file : JSON.stringify(file), ctx);
    eq(parsed.cards, [], `${message} draws no card`);
    ok(parsed.error.includes(why), `${message} says "${why}"; got "${parsed.error}"`);
}

const store = { id: "ajiadb9", shipped: false };
const shipped = { id: "nomarchy", shipped: true };
const one = blocks => ({ schema: 1, cards: [{ title: "AJIADB9", glyph: "力", blocks }] });

// a Store style: text and its own IPC target, with and without arguments
{
    const parsed = StyleCard.parse(JSON.stringify(one([
        { text: "  Notch islands on a top band.  " },
        { buttons: [{ label: "OPEN AJIADB9 SETTINGS", ipc: "ajiadb9", call: "open" }] }
    ])), store);
    eq(parsed.error, "", "a well-formed Store card parses");
    eq(parsed.cards, [{
        title: "AJIADB9", glyph: "力", blocks: [
            { kind: "text", text: "Notch islands on a top band." },
            { kind: "buttons", buttons: [{ label: "OPEN AJIADB9 SETTINGS", hub: "", command: ["qs", "-c", "shell", "ipc", "call", "ajiadb9", "open"] }] }
        ]
    }], "the Store card's blocks and argv");
}

// the forms every hand-built card uses, from a shipped style
{
    const parsed = StyleCard.parse(JSON.stringify({ schema: 1, cards: [{ title: "NOMARCHY", glyph: "野", blocks: [
        { text: "Nomarchy keeps Omarchy's configurable bar." },
        { buttons: [
            { label: "CHOOSE THEME", ipc: "nomarchy", call: "summon", args: ["omarchy.image-picker", "{\"source\":\"themes\"}"] },
            { label: "OPEN PLUGINS", ipc: "nomarchy", call: "summon", args: ["omarchy.menu", "{\"menu\":\"setup.plugins\"}"] }
        ] },
        { buttons: [
            { label: "OPEN PYTHON SETTINGS", ipc: "main", call: "handleCommand", args: ["toggle", "guide", ""] },
            { label: "OPEN QS BAR SETTINGS", ryokuShell: ["bar", "settings"] },
            { label: "OPEN DISPLAYS", hub: "displays" }
        ] }
    ] }] }), shipped);
    eq(parsed.error, "", "a shipped card with every action form parses");
    const buttons = parsed.cards[0].blocks[1].buttons.concat(parsed.cards[0].blocks[2].buttons);
    eq(buttons.map(b => b.command), [
        ["qs", "-c", "shell", "ipc", "call", "nomarchy", "summon", "omarchy.image-picker", "{\"source\":\"themes\"}"],
        ["qs", "-c", "shell", "ipc", "call", "nomarchy", "summon", "omarchy.menu", "{\"menu\":\"setup.plugins\"}"],
        ["qs", "-c", "shell", "ipc", "call", "main", "handleCommand", "toggle", "guide", ""],
        ["ryoku-shell", "bar", "settings"],
        []
    ], "each form builds the argv the hand-built card runs");
    eq(buttons[4].hub, "displays", "a hub button names its section");
}

// up to three cards, each drawn in order
{
    const c = n => ({ title: "CARD " + n, blocks: [{ text: "t" }] });
    eq(StyleCard.parse(JSON.stringify({ schema: 1, cards: [c(1), c(2), c(3)] }), shipped).cards.map(x => x.title),
        ["CARD 1", "CARD 2", "CARD 3"], "three cards keep their order");
    eq(StyleCard.parse(JSON.stringify({ schema: 1, cards: [c(1)] }), shipped).cards[0].glyph, "", "the glyph is optional");
    rejects({ schema: 1, cards: [c(1), c(2), c(3), c(4)] }, shipped, "cards is not a list", "a fourth card");
}

// no file is not an error
eq(StyleCard.parse("", store), { cards: [], error: "" }, "an empty text is no file");
eq(StyleCard.parse(undefined, store), { cards: [], error: "" }, "an unread file is no file");

// malformed files are refused whole, with the reason
rejects("{ not json", store, "not JSON", "broken JSON");
rejects("[]", store, "not a JSON object", "a top-level array");
rejects({ cards: [] }, store, "schema is not 1", "a missing schema");
rejects({ schema: 2, cards: [] }, store, "schema is not 1", "a newer schema");
rejects({ schema: 1, cards: [] }, store, "cards is not a list", "no cards");
rejects({ schema: 1, cards: [{ title: "X", blocks: [{ text: "t" }] }], extra: 1 }, store, "unknown field \"extra\"", "an unknown top-level field");
rejects(one([{ text: "t", buttons: [] }]), store, "unknown field \"buttons\"", "a block with two kinds");
rejects(one([{ image: "x.png" }]), store, "neither a text nor a buttons block", "an unknown block kind");
rejects(one([]), store, "blocks is not a list", "a card with no blocks");
rejects({ schema: 1, cards: [{ title: "  ", blocks: [{ text: "t" }] }] }, store, "title is empty", "a blank title");
rejects({ schema: 1, cards: [{ title: "X".repeat(41), blocks: [{ text: "t" }] }] }, store, "longer than 40", "a long title");
rejects({ schema: 1, cards: [{ title: "X", glyph: "力力力", blocks: [{ text: "t" }] }] }, store, "glyph is longer", "a long glyph");
rejects(one([{ text: 5 }]), store, "text is not a string", "a number as text");
rejects({ schema: 1, cards: [{ title: "<b>X</b>", blocks: [{ text: "t" }] }] }, store, "title has a \"<\"", "markup in a title");
rejects({ schema: 1, cards: [{ title: "X", glyph: "<i>", blocks: [{ text: "t" }] }] }, store, "glyph has a \"<\"", "markup in a glyph");
rejects(one([{ buttons: [{ label: "<img src=x>", ipc: "ajiadb9", call: "open" }] }]), store, "label has a \"<\"", "markup in a label");
eq(StyleCard.parse(JSON.stringify(one([{ text: "a < b & <b>c</b>" }])), store).cards[0].blocks[0].text, "a < b & <b>c</b>", "a paragraph keeps its characters, drawn as plain text");
rejects(one([{ buttons: [] }]), store, "buttons is not a list", "an empty button row");
rejects(one([{ buttons: [1, 2, 3, 4, 5].map(n => ({ label: "B" + n, ipc: "ajiadb9", call: "open" })) }]), store, "buttons is not a list", "five buttons");

// the action forms stay narrow
const btn = b => one([{ buttons: [Object.assign({ label: "GO" }, b)] }]);
rejects(btn({}), store, "exactly one of", "a button with no action");
rejects(btn({ ipc: "ajiadb9", call: "open", hub: "displays" }), store, "exactly one of", "a button with two actions");
rejects(btn({ ipc: "ajiadb9; rm -rf ~", call: "open" }), store, "ipc is not a plain identifier", "a target with shell syntax");
rejects(btn({ ipc: "-ajiadb9", call: "open" }), store, "ipc is not a plain identifier", "a target that reads as a flag");
eq(StyleCard.parse(JSON.stringify(btn({ ipc: "2bar", call: "open" })), { id: "2bar", shipped: false }).cards[0].blocks[0].buttons[0].command,
    ["qs", "-c", "shell", "ipc", "call", "2bar", "open"], "a Store id that starts with a digit reaches its own target");
rejects(btn({ ipc: "ajiadb9", call: "$(id)" }), store, "call is not a plain identifier", "a call with shell syntax");
rejects(btn({ ipc: "ajiadb9", call: "open", args: "x" }), store, "args is not a list", "args as a string");
rejects(btn({ ipc: "ajiadb9", call: "open", args: [1] }), store, "args is not a list", "a number argument");
rejects(btn({ ipc: "ajiadb9", call: "open", args: ["x".repeat(513)] }), store, "args is not a list", "a long argument");
rejects(btn({ ipc: "ajiadb9", call: "open", run: "sh" }), store, "unknown field \"run\"", "an extra command field");
rejects(btn({ hub: "../etc" }), store, "not a Hub section name", "a hub route that is not a section name");
rejects(btn({ hub: "displays", args: ["x"] }), store, "unknown field \"args\"", "a hub button with arguments");
rejects(btn({ ipc: "desktop", call: "editSection" }), store, "must be the style's own id", "a Store card calling another target");
rejects(btn({ ryokuShell: ["bar", "settings"] }), store, "only for styles that ship with Ryoku", "a Store card calling ryoku-shell");
rejects(btn({ ryokuShell: ["bar", "--json"] }), shipped, "plain words", "a ryoku-shell flag");
rejects(btn({ ryokuShell: [] }), shipped, "plain words", "an empty ryoku-shell call");

// the Store view is the one BarProducts would load
{
    const view = "barstyle-views/ajiadb9/" + "a".repeat(64);
    const index = JSON.stringify([{ id: "other", view: "barstyle-views/other/" + "b".repeat(64) }, { id: "ajiadb9", view }]);
    eq(StyleCard.storeView(index, "ajiadb9"), view, "finds the style's view");
    eq(StyleCard.storeView(index, "missing"), "", "no row, no view");
    eq(StyleCard.storeView(JSON.stringify([{ id: "ajiadb9", view: "barstyle-views/ajiadb9/../../x" }]), "ajiadb9"), "", "refuses a view outside the pattern");
    eq(StyleCard.storeView(JSON.stringify([{ id: "x", view: "barstyle-views/other/" + "a".repeat(64) }]), "x"), "", "refuses another style's view");
    eq(StyleCard.storeView(index, "../x"), "", "refuses an id that is not a store id");
    eq(StyleCard.storeView("{", "ajiadb9"), "", "a broken index has no views");
}

if (failed > 0) {
    console.error(`${failed} failure(s)`);
    process.exit(1);
}
console.log("StyleCard: ok");
