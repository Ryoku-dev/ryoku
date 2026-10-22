pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons
import ".."
import "../Singletons"

// One group of the Caelestia shell style's settings, drawn by the shared schema
// renderer from rows the generator wrote out of the shell's own settings pages.
// Every value rides the Hub's Caelestia store (hub.caeVal and hub.caeEdit); Save
// writes only what differs from the shell's defaults, and the running shell
// picks the file up on its own.
Item {
    id: pg
    property var hub
    property var rows: []
    property string pTitle: ""
    property string pBlurb: ""

    function focusKey(k) { sp.focusKey(k) }

    readonly property var draft: {
        var d = {};
        if (pg.hub)
            for (var i = 0; i < pg.rows.length; i++) d[pg.rows[i].key] = pg.hub.caeVal(pg.rows[i].key);
        return d;
    }
    // what a row's reset puts back: the shell's own default, carried on the row
    readonly property var factory: {
        var d = {};
        for (var i = 0; i < pg.rows.length; i++) d[pg.rows[i].key] = pg.rows[i].def;
        return d;
    }

    // a dependent option stays hidden until the option it needs is on
    readonly property var shownRows: {
        var d = pg.draft, out = [];
        for (var i = 0; i < pg.rows.length; i++) {
            var r = pg.rows[i], ok = true;
            for (var k in (r.when || {}))
                if (r.when[k].indexOf(d[k] === undefined ? pg.factory[k] : d[k]) < 0) ok = false;
            if (ok) out.push(r);
        }
        return out;
    }

    SchemaPage {
        id: sp
        anchors.fill: parent
        schema: pg.shownRows
        draft: pg.draft
        defaults: pg.factory
        advanced: pg.hub ? pg.hub.advanced : false
        title: pg.pTitle
        eyebrow: I18n.tr("SHELL")
        blurb: pg.pBlurb
        query: pg.hub ? pg.hub.query : ""
        onEdited: (k, v) => { if (pg.hub) pg.hub.caeEdit(k, v); }
    }
}
