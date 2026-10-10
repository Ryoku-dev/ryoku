.pragma library

// QS Bar Settings routes, in rail order. One entry per page: its Latin name, its
// kanji seal (a real word, per the desktop's language, never decoration), the
// one-line summary the head prints (kept to a few words: it has one line), the
// words search matches on, and the rail section it sits in.
//
// The panel is about one thing, the bar, so the rail's first group is five
// routes: where the bar sits and how its panels look (Bar), the mark and
// workspaces that make it this user's (Identity), how its widgets are arranged
// (Layout), what each widget is and does (Widgets), and the dock that rides
// beside it (Dock). A second, parted group holds Community: every installed bar
// plugin that is not Ryoku's own, so a widget someone else wrote is never mixed
// in with the shipped set. Desktop widgets, mid-work switches and the session
// already have a home elsewhere.
var ROUTES = [
    { id: "bars",    label: "Bar",     gloss: "\u5e2f", file: "BarsRoute", section: "bar",
      desc: "Position, shape, surface, gaps, motion and picker style.",
      keywords: "position top bottom form full fit dock notch islands surface border corners frost shadow depth tooltip gap gaps accent colour scale size motion animation auto hide picker pickers tanzaku hearthstone carousel" },
    { id: "identity", label: "Identity", gloss: "\u5370", file: "IdentityRoute", section: "bar",
      desc: "The launcher mark and the workspaces.",
      keywords: "identity logo mark wordmark word glyph icon kanji launcher brand ryoku arch hyprland workspaces spaces count marker style numbers kanji rings preview" },
    { id: "layout",  label: "Layout",  gloss: "\u914d\u7f6e", file: "LayoutRoute", section: "bar",
      desc: "Where each widget sits on the bar.",
      keywords: "arrange order move reorder left center centre right lane add widget plugin unlock drag reset layout hide show" },
    { id: "widgets", label: "Widgets", gloss: "\u90e8\u54c1", file: "WidgetsRoute", section: "bar",
      desc: "Show, size, colour and tune each widget.",
      keywords: "widget show hide on off density icon compact colour fill ai claude codex opencode volume boost clock weather sensor temperature plugin settings" },
    { id: "dock",    label: "Dock",    gloss: "\u53f0", file: "DockRoute", section: "bar",
      desc: "The app dock opposite the bar.",
      keywords: "dock app pinned pin edge autohide auto hide magnify frost depth shadow label media chip peek" },
    { id: "community", label: "Community", gloss: "\u6709\u5fd7", file: "CommunityRoute", section: "community",
      desc: "Bar widgets installed from outside Ryoku.",
      keywords: "community plugin plugins installed third party store ryostore git add remove widget" }
];

function byId(id) {
    for (var i = 0; i < ROUTES.length; i++)
        if (ROUTES[i].id === id) return ROUTES[i];
    return null;
}

function labelFor(id) {
    var r = byId(id);
    return r ? r.label : id;
}

function fileFor(id) {
    var r = byId(id);
    return r ? r.file : "";
}

function indexOf(id) {
    for (var i = 0; i < ROUTES.length; i++)
        if (ROUTES[i].id === id) return i;
    return -1;
}

// the routes of one rail section, in order.
function inSection(section) {
    var out = [];
    for (var i = 0; i < ROUTES.length; i++)
        if (ROUTES[i].section === section) out.push(ROUTES[i]);
    return out;
}

// Retired route ids, mapped to their nearest current home, so an old caller (a
// keybind, a saved link, `bar settings <route>`) never lands on nothing:
//   logo, spaces      -> Identity (the mark and the workspaces, one route again)
//   widgets, appearance -> the Widgets route (renamed from the old Appearance)
//   pickers           -> Bar, where picker appearance now lives
//   desktop           -> the Hub owns desktop widgets; nearest here is Widgets
//   system, session   -> quick settings (Super+Escape); nearest here is Bar
//   plugins           -> the Community route
function resolve(id) {
    if (byId(id)) return id;
    switch (id) {
    case "logo":
    case "spaces":
        return "identity";
    case "appearance":
    case "desktop":
        return "widgets";
    case "pickers":
        return "bars";
    case "plugins":
        return "community";
    case "system":
    case "session":
        return "bars";
    default:
        return "bars";
    }
}
