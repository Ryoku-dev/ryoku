pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Ryoku.Ui
import Ryoku.Ui.Singletons
import ".."
import "../Singletons"

// Layer Rules (DESIGN.md section 11, ADVANCED). Per-namespace layer-shell tweaks
// (blur, dim, no-animation, show above lock) applied to Hyprland layer surfaces
// like the bar, launcher and notification daemon. Not a settings sheet: the one
// "key" (layerRules) is an array of { namespace, action, value } entries, so it
// is built bespoke as a list editor -- a section head, a scrolling column of
// namespace / action-chips / value cards, and an add affordance -- wired to the
// hub's hypr store. Layer rules are not previewed live; they apply on Save. The
// shell owns the rail, the side panel and the Save/Revert/Reset action bar (that
// action bar carries the dirty dot, the dirty text and the Save/Revert this page
// used to draw itself, and the hub head owns the Edit-config action); nothing
// here writes to disk. Every value reads from Tokens.
Item {
    id: pg

    property var hub

    // the live rules array from the draft: { namespace, action, value } entries.
    readonly property var rules: pg.hub ? (pg.hub.hyprVal("wm.hyprland.layerRules") || []) : []
    // gated so the empty state does not flash before `hypr get` returns.
    readonly property bool ready: pg.hub ? pg.hub.wmLoaded === true : false

    // Hyprland drives its own layer rules through the bespoke editor below, bound
    // to wm.hyprland.layerRules; every other provider drives this page through
    // its own page:"layerrules" schema rows and the shared record editor. The
    // only compositor name on this page is that one store key, and it is guarded
    // by modelsKey so the bespoke editor stands down the moment another provider
    // owns the session.
    readonly property bool bespoke: pg.hub ? Settings.modelsKey("wm.hyprland.layerRules") : false
    // the running provider's layer-rule rows, kept to the ones it actually backs.
    readonly property var provRows: {
        ProviderSchema.revision;
        var out = [], rs = ProviderSchema.rowsFor("layerrules");
        for (var i = 0; i < rs.length; i++) {
            var r = rs[i];
            if (!r || !r.key) continue;
            if (String(r.key).indexOf("[") >= 0) continue;
            if (!Settings.supports(r.caps, r.hostCaps)) continue;
            if (!Settings.modelsKey(r.key)) continue;
            out.push(r);
        }
        return out;
    }
    readonly property bool hasProvRows: pg.provRows.length > 0
    // flat dotted-key maps the shared sheet reads, rebuilt on every draft edit.
    readonly property var provDraft: {
        var d = {};
        if (pg.hub) for (var i = 0; i < pg.provRows.length; i++) { var k = pg.provRows[i].key; if (k) d[k] = pg.hub.hyprVal(k); }
        return d;
    }
    readonly property var provCommitted: {
        var d = {};
        if (pg.hub) for (var i = 0; i < pg.provRows.length; i++) { var k = pg.provRows[i].key; if (k) d[k] = pg.hub.hyprCommittedVal(k); }
        return d;
    }
    function focusKey(k) { if (schemaLoader.item) schemaLoader.item.focusKey(k); }

    // the seven layer-shell tweaks. Only ignorealpha carries a value (a 0..1
    // threshold); dimaround and the rest emit a plain bool on the compositor
    // side, so they keep value empty. This is the old page's action table.
    readonly property var actionOptions: [
        { "key": "blur", "label": I18n.tr("Blur") },
        { "key": "blurpopups", "label": I18n.tr("Blur popups") },
        { "key": "ignorealpha", "label": I18n.tr("Ignore alpha") },
        { "key": "noanim", "label": I18n.tr("No animations") },
        { "key": "dimaround", "label": I18n.tr("Dim around") },
        { "key": "xray", "label": I18n.tr("Blur X-ray") },
        { "key": "abovelock", "label": I18n.tr("Show above lock") }
    ]
    readonly property var valueActions: ["ignorealpha"]
    // Chips speak in labels; the draft stores keys, so map across the boundary.
    readonly property var actionLabels: pg.actionOptions.map(function (o) { return o.label; })
    function labelFor(key) {
        for (var i = 0; i < pg.actionOptions.length; i++)
            if (pg.actionOptions[i].key === key)
                return pg.actionOptions[i].label;
        return key;
    }
    function keyFor(label) {
        for (var i = 0; i < pg.actionOptions.length; i++)
            if (pg.actionOptions[i].label === label)
                return pg.actionOptions[i].key;
        return label;
    }

    // hyprEdit swaps the whole array, so the Repeater rebinds and rebuilds the
    // card owning a focused field. cards therefore commit on editing-finished
    // only, and every helper hands hyprEdit a fresh slice rather than mutating
    // the live list. (Same discipline as SessionPage.)
    function patch(i, key, val) {
        if (!pg.hub)
            return;
        var a = (pg.hub.hyprVal("wm.hyprland.layerRules") || []).slice();
        a[i] = Object.assign({}, a[i]);
        a[i][key] = val;
        pg.hub.hyprEdit("wm.hyprland.layerRules", a);
    }
    // switching action seeds ignorealpha's default and clears the value for
    // every valueless action so nothing stale lingers in the draft.
    function setAction(i, key) {
        if (!pg.hub)
            return;
        var a = (pg.hub.hyprVal("wm.hyprland.layerRules") || []).slice();
        a[i] = Object.assign({}, a[i]);
        a[i].action = key;
        a[i].value = key === "ignorealpha" ? "0.5" : "";
        pg.hub.hyprEdit("wm.hyprland.layerRules", a);
    }
    function addRule() {
        if (!pg.hub)
            return;
        var a = (pg.hub.hyprVal("wm.hyprland.layerRules") || []).slice();
        a.push({ "namespace": "", "action": "blur", "value": "" });
        pg.hub.hyprEdit("wm.hyprland.layerRules", a);
    }
    function removeRule(i) {
        if (!pg.hub)
            return;
        var a = (pg.hub.hyprVal("wm.hyprland.layerRules") || []).slice();
        a.splice(i, 1);
        pg.hub.hyprEdit("wm.hyprland.layerRules", a);
    }
    function clearAll() {
        if (pg.hub)
            pg.hub.hyprEdit("wm.hyprland.layerRules", []);
    }

    // head: eyebrow, Fraunces title, blurb (matches every settings page). The
    // blurb carries the two caveats the old page split between its intro and its
    // action bar: applied on Save (not live), and a namespace that matches
    // nothing is a no-op.
    Column {
        id: head
        visible: pg.bespoke
        // the head sits on the body's grid, so the title starts over the first
        // card column instead of floating in a page-wide window
        anchors { top: parent.top; left: parent.left; right: parent.right }
        // the register row sits off the title: a rule over a 32px
        // title needs more than the gap between two lines of body text
        spacing: Tokens.s3

        Row {
            // the register row holds a fixed box, so the rule and the seal keep
            // their distance from the title on every page
            height: Tokens.s5
            spacing: Tokens.s2
            Rectangle {
                width: 16; height: 1; color: Tokens.ink
                anchors.verticalCenter: parent.verticalCenter
            }
            Text {
                text: UiPreferences.brandMark("力"); color: Tokens.ink; font.family: Tokens.jp
                font.pixelSize: 11; anchors.verticalCenter: parent.verticalCenter
            }
            Text {
                text: I18n.tr("COMPOSITOR"); color: Tokens.inkMuted; font.family: Tokens.ui
                font.pixelSize: 9; font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                anchors.verticalCenter: parent.verticalCenter
            }
        }
        Text {
            text: I18n.tr("Layer Rules"); color: Tokens.ink
            font.family: Tokens.display; font.pixelSize: Tokens.fTitle
        }
        Text {
            width: Math.min(parent.width, 720)
            text: I18n.tr("Layer surfaces by namespace: blur, dim, motion, order.")
            color: Tokens.inkMuted; font.family: Tokens.ui
            font.pixelSize: Tokens.fBody; wrapMode: Text.WordWrap
        }
    }

    // section head: dot + RULES + leader + count + clear all + add.
    Item {
        id: sect
        visible: pg.bespoke
        anchors { left: parent.left; right: parent.right; top: head.bottom; topMargin: Tokens.s5 }
        height: 32

        Row {
            id: sectLabel
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s2
            Rectangle {
                width: 4; height: 4; color: Tokens.ink
                anchors.verticalCenter: parent.verticalCenter
            }
            Text {
                text: I18n.tr("RULES"); color: Tokens.ink; font.family: Tokens.ui
                font.pixelSize: Tokens.fMicro; font.weight: Font.Medium
                font.letterSpacing: Tokens.trackMark
                anchors.verticalCenter: parent.verticalCenter
            }
        }

        Row {
            id: sectActions
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s3

            Text {
                anchors.verticalCenter: parent.verticalCenter
                // an entry count is file-truth chrome, so mono (DESIGN.md section 2).
                text: pg.rules.length === 1 ? I18n.tr("%1 ENTRY").arg(pg.rules.length) : I18n.tr("%1 ENTRIES").arg(pg.rules.length)
                color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny
            }
            Btn {
                anchors.verticalCenter: parent.verticalCenter
                // wipes every rule; Revert (shell action bar) is the only undo.
                text: I18n.tr("CLEAR ALL")
                armed: pg.rules.length > 0
                onAct: pg.clearAll()
            }
            IconBtn {
                anchors.verticalCenter: parent.verticalCenter
                glyph: "+"
                onAct: pg.addRule()
            }
        }

        Rectangle {
            anchors.left: sectLabel.right; anchors.right: sectActions.left
            anchors.leftMargin: Tokens.s3; anchors.rightMargin: Tokens.s3
            anchors.verticalCenter: parent.verticalCenter
            height: 1; color: Tokens.lineSoft
        }
    }

    // the scrolling card list.
    Flickable {
        id: flick
        visible: pg.bespoke
        anchors {
            left: parent.left; right: parent.right
            top: sect.bottom; bottom: parent.bottom
            topMargin: Tokens.s4
        }
        contentWidth: width
        contentHeight: Math.max(col.height, height)
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
        WheelScroll { }

        CardColumns {

        id: col
            // a body of cards fills the measure and splits into balanced columns
            width: flick.width - Tokens.s3
            spacing: Tokens.s2
            fillTo: flick.height

            Repeater {
                model: pg.rules

                delegate: Item {
                    id: rowItem
                    required property int index
                    required property var modelData

                    readonly property bool needsValue: pg.valueActions.indexOf(rowItem.modelData.action) >= 0

                    width: col.colWidth
                    // the card grows a row when the action needs a value and
                    // shrinks when it does not: the reflow the old page did by
                    // recomputing the namespace field width, done by height here.
                    height: card.height

                    Rectangle {
                        id: card
                        width: parent.width
                        height: inner.implicitHeight + Tokens.s3 * 2
                        radius: Tokens.radius
                        color: "transparent"
                        border.width: Tokens.border
                        border.color: Tokens.line

                        Column {
                            id: inner
                            anchors {
                                left: parent.left; right: parent.right; top: parent.top
                                margins: Tokens.s3
                            }
                            spacing: Tokens.s3

                            // namespace + remove.
                            Item {
                                width: parent.width
                                height: Tokens.rowH - 18   // 30, the field height

                                // namespace: a layer-shell surface id, so mono
                                // (file-truth boundary).
                                Field {
                                    id: nsField
                                    anchors.left: parent.left
                                    anchors.right: removeBtn.left
                                    anchors.rightMargin: Tokens.s2
                                    anchors.verticalCenter: parent.verticalCenter
                                    tabular: true
                                    placeholder: I18n.tr("namespace  (e.g. launcher, overview, bar)")
                                    text: rowItem.modelData.namespace || ""
                                    onCommitted: (v) => {
                                        if (v !== (rowItem.modelData.namespace || ""))
                                            pg.patch(rowItem.index, "namespace", v);
                                    }
                                }
                                IconBtn {
                                    id: removeBtn
                                    anchors.right: parent.right
                                    anchors.verticalCenter: parent.verticalCenter
                                    // a paired minus, not a trash icon: remove is
                                    // not danger and there is no red on the sheet.
                                    glyph: "\u2212"
                                    onAct: pg.removeRule(rowItem.index)
                                }
                            }

                            // action: seven exclusive tweaks, so chips (DESIGN.md
                            // section 6). The selected chip inverts to bone; this
                            // replaces the old floating action Dropdown.
                            Chips {
                                width: parent.width
                                options: pg.actionLabels
                                current: pg.labelFor(rowItem.modelData.action)
                                onChose: (lab) => pg.setAction(rowItem.index, pg.keyFor(lab))
                            }

                            // value: only ignorealpha carries one (a 0..1 alpha
                            // threshold). Hidden for every valueless action, and
                            // Column drops the hidden child so the card shrinks.
                            Row {
                                visible: rowItem.needsValue
                                spacing: Tokens.s2

                                Text {
                                    anchors.verticalCenter: parent.verticalCenter
                                    text: I18n.tr("ALPHA")
                                    color: Tokens.inkMuted; font.family: Tokens.ui
                                    font.pixelSize: Tokens.fTiny; font.weight: Font.Medium
                                    font.letterSpacing: Tokens.trackLabel
                                }
                                Field {
                                    width: 120
                                    tabular: true
                                    placeholder: "0.0 - 1.0"
                                    text: rowItem.modelData.value || ""
                                    onCommitted: (v) => {
                                        if (v !== (rowItem.modelData.value || ""))
                                            pg.patch(rowItem.index, "value", v);
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }
    }

    // empty state, gated on load so it does not flash before data arrives.
    Empty {
        anchors.centerIn: flick
        visible: pg.bespoke && pg.ready && pg.rules.length === 0
        caption: I18n.tr("No custom layer rules yet. Add one to get started.")
    }

    // Any other provider's layer-rule rows, drawn by the shared renderer the same
    // way the Window Manager page draws its provider rows.
    Loader {
        id: schemaLoader
        anchors.fill: parent
        active: !pg.bespoke && pg.hasProvRows
        sourceComponent: schemaComp
    }
    Component {
        id: schemaComp
        SchemaPage {
            anchors.fill: parent
            schema: pg.provRows
            draft: pg.provDraft
            defaults: pg.provCommitted
            advanced: pg.hub ? pg.hub.advanced : false
            title: I18n.tr("Layer Rules")
            eyebrow: I18n.tr("COMPOSITOR")
            blurb: I18n.tr("Layer surfaces by namespace: blur, shadow, opacity and geometry.")
            query: pg.hub ? pg.hub.query : ""
            onEdited: (k, v) => { if (pg.hub) pg.hub.hyprEdit(k, v); }
            onPickRequested: (r) => { if (pg.hub) pg.hub.openPick(r); }
        }
    }

    // Capability present, but the provider ships no rows to edit: an honest
    // empty state rather than a blank page.
    Empty {
        anchors.centerIn: parent
        visible: !pg.bespoke && !pg.hasProvRows && ProviderSchema.ready && Settings.supports("layerRules")
        caption: I18n.tr("This compositor supports layer rules but has no editor to show yet.")
    }
}
