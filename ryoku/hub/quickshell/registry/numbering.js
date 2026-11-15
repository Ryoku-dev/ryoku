.pragma library
.import "registry.js" as Registry

// the sheet's numbers, as ryoku.dev numbers its sections: Home is 00, each category its place
// in the tree (01 Appearance .. 09 Updates), each page its place in its category (01.2)

function pad(n) {
    return n < 10 ? "0" + n : String(n);
}

function categoryNumber(id) {
    var i = Registry.categoryIndex(id);
    return i < 0 ? "" : pad(i + 1);
}

// "01.2" for a page, "00" for Home, "" for an unknown page
function pageNumber(pageId, gate) {
    if (!pageId)
        return "00";
    var p = Registry.page(pageId);
    if (!p)
        return "";
    var c = Registry.category(p.category);
    if (c && c.pages.length === 1)
        return categoryNumber(p.category);
    var ids = Registry.pagesOf(p.category, gate).map(x => x.id);
    var at = ids.indexOf(pageId);
    return categoryNumber(p.category) + (at < 0 ? "" : "." + (at + 1));
}

// { at, of }: the page's place among every page that opens in Settings
function ordinal(pageId, gate) {
    var ids = [];
    Registry.categories.forEach(c => {
        Registry.pagesOf(c.id, gate).forEach(p => {
            if (!p.handoff)
                ids.push(p.id);
        });
    });
    return { at: ids.indexOf(pageId) + 1, of: ids.length };
}
