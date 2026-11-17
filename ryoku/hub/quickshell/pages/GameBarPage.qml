pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons
import ".."
import "../schema/GameBarPage.js" as GameBarSchema

// Game Bar: the overlay's master switch, look, placement and settle, plus the
// Discord Voice widget. The rows are the shared settings sheet over the
// shell.json draft, so Save, Revert and Reset ride the Hub's action bar.
Item {
    id: pg

    property var hub

    readonly property string pTitle: I18n.tr("Game Bar")
    readonly property string pEyebrow: I18n.tr("DESKTOP")
    readonly property string pBlurb: I18n.tr("The overlay SUPER + SHIFT + G opens over a game: dimming, widget placement, snapping, motion and the Discord Voice overlay.")
    readonly property var schema: GameBarSchema.rows

    function focusKey(k) { sp.focusKey(k) }

    SchemaPage {
        id: sp
        anchors.fill: parent
        schema: pg.schema
        draft: pg.hub ? pg.hub.draft : null
        defaults: pg.hub ? pg.hub.committed : ({})
        advanced: pg.hub ? pg.hub.advanced : false
        title: pg.pTitle
        eyebrow: pg.pEyebrow
        blurb: pg.pBlurb
        query: pg.hub ? pg.hub.query : ""
        onEdited: (k, v) => { if (pg.hub) pg.hub.edit(k, v); }
        onPickRequested: (r) => { if (pg.hub) pg.hub.openPick(r); }
    }
}
