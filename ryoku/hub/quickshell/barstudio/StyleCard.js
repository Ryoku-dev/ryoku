// A bar style's own Bar Studio cards, declared in a studio-card.json beside its
// Scene.qml. parse() turns the file's text into cards the page can draw, or says
// why it cannot. Nothing here runs anything: a button comes out as either a Hub
// section to open or a fixed argv, and only the forms below can build one.
//
// The file is data from whoever made the style. A Store style may only reach
// its own IPC target and the Hub's pages; a style that ships with Ryoku may
// also call any shell IPC target and the ryoku-shell CLI, and place one of the
// Hub's own editors (Sumi's frame, Obi's widgets, ...) by name. Anything
// outside these forms rejects the whole file, so a card is never drawn
// half-understood.

const SCHEMA = 1;
const MAX_CARDS = 3;
const MAX_BLOCKS = 8;
const MAX_BUTTONS = 4;
const MAX_ARGS = 8;
const MAX_ARG = 512;
const MAX_TITLE = 40;
const MAX_GLYPH = 2;
const MAX_TEXT = 600;
const MAX_LABEL = 48;
const MAX_VERBS = 4;

const IDENT = /^[A-Za-z][A-Za-z0-9_-]{0,63}$/;
// an IPC target may start with a digit, as a Store id may; never with "-".
const TARGET = /^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$/;
const WORD = /^[a-z][a-z0-9-]{0,31}$/;

function fail(message) {
    throw new Error(message);
}

// A title, glyph or label lands in a shared control that would read "<b>" as
// markup; a paragraph is drawn as plain text and may hold any character.
function plainText(value, what, max, optional, markupFree) {
    if (value === undefined && optional)
        return "";
    if (typeof value !== "string")
        fail(what + " is not a string");
    if (markupFree && value.indexOf("<") >= 0)
        fail(what + " has a \"<\"");
    const s = value.trim();
    if (s === "" && !optional)
        fail(what + " is empty");
    if (Array.from(s).length > max)
        fail(what + " is longer than " + max + " characters");
    return s;
}

function onlyKeys(obj, allowed, what) {
    for (const key of Object.keys(obj))
        if (allowed.indexOf(key) < 0)
            fail(what + " has an unknown field \"" + key + "\"");
}

function isObject(v) {
    return v !== null && typeof v === "object" && !Array.isArray(v);
}

function button(raw, where, ctx) {
    if (!isObject(raw))
        fail(where + " is not an object");
    const label = plainText(raw.label, where + ".label", MAX_LABEL, false, true);
    const forms = ["ipc", "hub", "ryokuShell"].filter(k => raw[k] !== undefined);
    if (forms.length !== 1)
        fail(where + " needs exactly one of ipc, hub or ryokuShell");

    if (forms[0] === "hub") {
        onlyKeys(raw, ["label", "hub"], where);
        if (typeof raw.hub !== "string" || !WORD.test(raw.hub))
            fail(where + ".hub is not a Hub section name");
        return { label: label, hub: raw.hub, command: [] };
    }

    if (forms[0] === "ryokuShell") {
        onlyKeys(raw, ["label", "ryokuShell"], where);
        if (!ctx.shipped)
            fail(where + ".ryokuShell is only for styles that ship with Ryoku");
        const verbs = raw.ryokuShell;
        if (!Array.isArray(verbs) || verbs.length === 0 || verbs.length > MAX_VERBS
                || !verbs.every(v => typeof v === "string" && WORD.test(v)))
            fail(where + ".ryokuShell is not a list of 1 to " + MAX_VERBS + " plain words");
        return { label: label, hub: "", command: ["ryoku-shell"].concat(verbs) };
    }

    onlyKeys(raw, ["label", "ipc", "call", "args"], where);
    if (typeof raw.ipc !== "string" || !TARGET.test(raw.ipc))
        fail(where + ".ipc is not a plain identifier");
    if (!ctx.shipped && raw.ipc !== ctx.id)
        fail(where + ".ipc must be the style's own id \"" + ctx.id + "\"");
    if (typeof raw.call !== "string" || !IDENT.test(raw.call))
        fail(where + ".call is not a plain identifier");
    const args = raw.args === undefined ? [] : raw.args;
    if (!Array.isArray(args) || args.length > MAX_ARGS
            || !args.every(a => typeof a === "string" && a.length <= MAX_ARG))
        fail(where + ".args is not a list of up to " + MAX_ARGS + " strings");
    return {
        label: label,
        hub: "",
        command: ["qs", "-c", "shell", "ipc", "call", raw.ipc, raw.call].concat(args)
    };
}

function block(raw, where, ctx) {
    if (!isObject(raw))
        fail(where + " is not an object");
    if (raw.text !== undefined) {
        onlyKeys(raw, ["text"], where);
        return { kind: "text", text: plainText(raw.text, where + ".text", MAX_TEXT, false) };
    }
    if (raw.buttons !== undefined) {
        onlyKeys(raw, ["buttons"], where);
        const list = raw.buttons;
        if (!Array.isArray(list) || list.length === 0 || list.length > MAX_BUTTONS)
            fail(where + ".buttons is not a list of 1 to " + MAX_BUTTONS + " buttons");
        return { kind: "buttons", buttons: list.map((b, i) => button(b, where + ".buttons[" + i + "]", ctx)) };
    }
    if (raw.editor !== undefined) {
        onlyKeys(raw, ["editor"], where);
        if (!ctx.shipped)
            fail(where + ".editor is only for styles that ship with Ryoku");
        if (typeof raw.editor !== "string" || ctx.editors.indexOf(raw.editor) < 0)
            fail(where + ".editor is not one of the Hub's editors");
        return { kind: "editor", editor: raw.editor };
    }
    fail(where + " is neither a text, a buttons nor an editor block");
}

function card(raw, where, ctx) {
    if (!isObject(raw))
        fail(where + " is not an object");
    onlyKeys(raw, ["title", "glyph", "blocks"], where);
    const blocks = raw.blocks;
    if (!Array.isArray(blocks) || blocks.length === 0 || blocks.length > MAX_BLOCKS)
        fail(where + ".blocks is not a list of 1 to " + MAX_BLOCKS + " blocks");
    return {
        title: plainText(raw.title, where + ".title", MAX_TITLE, false, true),
        glyph: plainText(raw.glyph, where + ".glyph", MAX_GLYPH, true, true),
        blocks: blocks.map((b, i) => block(b, where + ".blocks[" + i + "]", ctx))
    };
}

// parse(text, { id, shipped, editors }) -> { cards, error }. `editors` names
// the editors the page can place. `cards` is empty when the file is malformed,
// and `error` then says where; an empty text is no file.
function parse(text, ctx) {
    const context = {
        id: String(ctx && ctx.id || ""),
        shipped: !!(ctx && ctx.shipped),
        editors: Array.isArray(ctx && ctx.editors) ? ctx.editors : []
    };
    if (typeof text !== "string" || text.trim() === "")
        return { cards: [], error: "" };
    try {
        let raw;
        try {
            raw = JSON.parse(text);
        } catch (e) {
            fail("not JSON: " + e.message);
        }
        if (!isObject(raw))
            fail("the file is not a JSON object");
        onlyKeys(raw, ["schema", "cards"], "the file");
        if (raw.schema !== SCHEMA)
            fail("schema is not " + SCHEMA);
        if (!Array.isArray(raw.cards) || raw.cards.length === 0 || raw.cards.length > MAX_CARDS)
            fail("cards is not a list of 1 to " + MAX_CARDS + " cards");
        return { cards: raw.cards.map((c, i) => card(c, "cards[" + i + "]", context)), error: "" };
    } catch (e) {
        return { cards: [], error: e.message };
    }
}

// The Store's view of an installed style, from barstyles.json, checked the way
// the shell's BarProducts checks it before it loads the style's Scene.qml.
function storeView(indexText, id) {
    if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(String(id || "")))
        return "";
    let rows = [];
    try {
        rows = JSON.parse(indexText || "[]");
    } catch (e) {
        return "";
    }
    if (!Array.isArray(rows))
        return "";
    const pattern = new RegExp("^barstyle-views/" + id + "/[a-f0-9]{64}$");
    for (const row of rows) {
        const view = String(row && row.view || "");
        if (row && row.id === id && pattern.test(view))
            return view;
    }
    return "";
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { parse, storeView };
