pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons

// App Overrides (DESIGN.md section 11, ADVANCED). Per-app look overrides layered
// on top of the global Appearance, matched by window class (and an optional
// title). Like Environment and Window Rules this is not a settings sheet: the
// one "key" (appOverrides) is an array of records, so it is built bespoke as a
// record-list editor. Each record overrides opacity, corners, border, blur,
// shadow, dim, animations, or forces the window opaque; anything left on Inherit
// keeps following the global. On Save every record becomes a Hyprland window
// rule; window rules are not live-previewed, so this whole page is Save-only.
// The shell owns the rail, the side panel and the Save/Revert/Reset action bar;
// nothing here writes to disk. Every value reads from Tokens.
Item {
    id: pg

    property var hub

    // the live records from the draft: an unbounded list of app-override
    // objects, each carrying the 10-field schema seeded by addApp().
    readonly property var overrides: pg.hub ? (pg.hub.hyprVal("desktop.appOverrides") || []) : []
    // gated so the empty state does not flash before `hypr get` returns.
    readonly property bool ready: pg.hub ? pg.hub.wmLoaded === true : false

    // hyprEdit swaps the whole array by identity, so the Repeater rebinds and
    // rebuilds the card owning a focused field. Fields therefore commit on
    // editing-finished only, and every helper hands hyprEdit a fresh slice
    // (slice + Object.assign) rather than mutating the live list: HyprStore
    // diffs the whole snapshot, so an in-place mutation would never mark dirty.
    function patch(i, key, val) {
        if (!pg.hub)
            return;
        var a = (pg.hub.hyprVal("desktop.appOverrides") || []).slice();
        a[i] = Object.assign({}, a[i]);
        a[i][key] = val;
        pg.hub.hyprEdit("desktop.appOverrides", a);
    }
    // the literal seed IS the per-record default contract: -1 sentinels for the
    // three numeric fields (0 is a legal custom value, distinct from -1), and
    // "inherit" for the five string enums.
    function addApp(cls) {
        if (!pg.hub)
            return;
        var a = (pg.hub.hyprVal("desktop.appOverrides") || []).slice();
        a.push({
            "class": cls || "", "title": "",
            "opacity": -1, "rounding": -1, "borderSize": -1,
            "blur": "inherit", "shadow": "inherit", "dim": "inherit",
            "anim": "inherit", "opaque": "inherit"
        });
        pg.hub.hyprEdit("desktop.appOverrides", a);
    }
    function removeApp(i) {
        if (!pg.hub)
            return;
        var a = (pg.hub.hyprVal("desktop.appOverrides") || []).slice();
        a.splice(i, 1);
        pg.hub.hyprEdit("desktop.appOverrides", a);
    }
    function clearAll() {
        if (pg.hub)
            pg.hub.hyprEdit("desktop.appOverrides", []);
    }

    // open windows, for the class picker: the app ids the compositor reports,
    // deduped, so you can pick an app instead of typing its class.
    readonly property var openClasses: {
        var ws = (pg.hub ? pg.hub.wmWindows : []) || [];
        var seen = {}, out = [];
        for (var i = 0; i < ws.length; i++) {
            var c = ws[i].appId || "";
            if (c.length && !seen[c]) { seen[c] = true; out.push(c); }
        }
        out.sort();
        return out;
    }

    // ── head: eyebrow, Fraunces title, blurb (matches every settings page) ──
    Column {
        id: head
        anchors.top: parent.top
        // the head sits on the body's grid, so the title starts over the first
        // card column instead of floating in the middle of a page-wide window
        x: Tokens.s6
        width: Math.max(320, pg.width - Tokens.s6 * 2 - Tokens.s3)
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
                text: I18n.tr("APPS & KEYS"); color: Tokens.inkMuted; font.family: Tokens.ui
                font.pixelSize: 9; font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                anchors.verticalCenter: parent.verticalCenter
            }
        }
        Text {
            text: I18n.tr("App Overrides"); color: Tokens.ink
            font.family: Tokens.display; font.pixelSize: Tokens.fTitle
        }
        Text {
            width: Math.min(parent.width, 720)
            text: I18n.tr("One app's own look. Anything left on Inherit follows the global.")
            color: Tokens.inkMuted; font.family: Tokens.ui
            font.pixelSize: Tokens.fBody; wrapMode: Text.WordWrap
        }
    }

    // ── section head: dot + APPS + leader + count + from-window + clear + add ──
    Item {
        id: sect
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
                text: I18n.tr("APPS"); color: Tokens.ink; font.family: Tokens.ui
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
                text: pg.overrides.length === 1 ? I18n.tr("%1 APP").arg(pg.overrides.length) : I18n.tr("%1 APPS").arg(pg.overrides.length)
                color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny
            }
            // a fire-once action wearing a button: opens the catalogue of open
            // window classes; picking one appends a card. Vanishes when nothing
            // is open, exactly like the old Dropdown.
            Btn {
                anchors.verticalCenter: parent.verticalCenter
                visible: pg.openClasses.length > 0
                text: I18n.tr("FROM WINDOW")
                onAct: appPick.show()
            }
            Btn {
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("CLEAR ALL")
                armed: pg.overrides.length > 0
                onAct: pg.clearAll()
            }
            IconBtn {
                anchors.verticalCenter: parent.verticalCenter
                glyph: "+"
                onAct: pg.addApp("")
            }
        }

        Rectangle {
            anchors.left: sectLabel.right; anchors.right: sectActions.left
            anchors.leftMargin: Tokens.s3; anchors.rightMargin: Tokens.s3
            anchors.verticalCenter: parent.verticalCenter
            height: 1; color: Tokens.lineSoft
        }
    }

    // ── the scrolling card list ──
    Flickable {
        id: flick
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
            spacing: Tokens.s3

            Repeater {
                model: pg.overrides

                delegate: Rectangle {
                    id: card
                    required property int index
                    required property var modelData

                    // records are read defensively: an older hypr.json may lack
                    // keys, and `class` is a reserved word so it is bracket-indexed.
                    readonly property string clsVal: card.modelData["class"] || ""
                    readonly property string titleVal: card.modelData.title || ""
                    readonly property real opacityVal: card.modelData.opacity === undefined ? -1 : card.modelData.opacity
                    readonly property real roundingVal: card.modelData.rounding === undefined ? -1 : card.modelData.rounding
                    readonly property real borderVal: card.modelData.borderSize === undefined ? -1 : card.modelData.borderSize

                    width: col.colWidth
                    height: body.implicitHeight + Tokens.s4 * 2
                    radius: Tokens.radius
                    color: "transparent"
                    border.width: Tokens.border
                    border.color: Tokens.line

                    Column {
                        id: body
                        anchors { left: parent.left; right: parent.right; top: parent.top; margins: Tokens.s4 }
                        spacing: Tokens.s3

                        // match row: class + optional title + remove.
                        Item {
                            width: parent.width
                            height: 30

                            readonly property real removeW: 26
                            readonly property real gap: Tokens.s2
                            readonly property real fieldsW: width - removeW - gap * 2
                            readonly property real classW: Math.round(fieldsW / 2)

                            Field {
                                id: classField
                                anchors.left: parent.left
                                anchors.verticalCenter: parent.verticalCenter
                                width: parent.classW
                                // a window class is a config match string, so mono.
                                tabular: true
                                placeholder: I18n.tr("Match class (e.g. kitty)")
                                text: card.clsVal
                                onCommitted: (v) => {
                                    if (v !== card.clsVal)
                                        pg.patch(card.index, "class", v);
                                }
                            }
                            Field {
                                anchors.left: classField.right
                                anchors.leftMargin: parent.gap
                                anchors.right: removeBtn.left
                                anchors.rightMargin: parent.gap
                                anchors.verticalCenter: parent.verticalCenter
                                tabular: true
                                placeholder: I18n.tr("Match title (optional)")
                                text: card.titleVal
                                onCommitted: (v) => {
                                    if (v !== card.titleVal)
                                        pg.patch(card.index, "title", v);
                                }
                            }
                            IconBtn {
                                id: removeBtn
                                anchors.right: parent.right
                                anchors.verticalCenter: parent.verticalCenter
                                // a paired minus, not a trash icon: remove is not
                                // danger and there is no red on the sheet to carry one.
                                glyph: "\u2212"
                                onAct: pg.removeApp(card.index)
                            }
                        }

                        Rectangle { width: parent.width; height: 1; color: Tokens.lineSoft }

                        // LOOK: the numeric overrides, Inherit or a custom value.
                        Column {
                            width: parent.width
                            spacing: Tokens.s1
                            Text {
                                text: I18n.tr("LOOK"); color: Tokens.ink; font.family: Tokens.ui
                                font.pixelSize: Tokens.fMicro; font.weight: Font.Medium
                                font.letterSpacing: Tokens.trackMark
                            }
                            Text {
                                width: parent.width
                                wrapMode: Text.WordWrap
                                text: I18n.tr("Inherit follows the global Appearance. Switch to Custom to set this app's own value.")
                                color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: 12
                            }
                        }

                        OvNum {
                            width: parent.width
                            label: I18n.tr("Opacity")
                            value: card.opacityVal
                            from: 0.2; to: 1.0; percent: true; customDefault: 0.9
                            onChanged: (v) => pg.patch(card.index, "opacity", v)
                        }
                        OvNum {
                            width: parent.width
                            label: I18n.tr("Corners")
                            value: card.roundingVal
                            from: 0; to: 24; customDefault: 8
                            onChanged: (v) => pg.patch(card.index, "rounding", Math.round(v))
                        }
                        OvNum {
                            width: parent.width
                            label: I18n.tr("Border")
                            value: card.borderVal
                            from: 0; to: 8; customDefault: 2
                            onChanged: (v) => pg.patch(card.index, "borderSize", Math.round(v))
                        }

                        // EFFECTS: the decoration toggles. Four force an effect off;
                        // Force opaque is the odd one out and forces opacity on.
                        Column {
                            width: parent.width
                            spacing: Tokens.s1
                            Text {
                                text: I18n.tr("EFFECTS"); color: Tokens.ink; font.family: Tokens.ui
                                font.pixelSize: Tokens.fMicro; font.weight: Font.Medium
                                font.letterSpacing: Tokens.trackMark
                            }
                            Text {
                                width: parent.width
                                wrapMode: Text.WordWrap
                                text: I18n.tr("Inherit follows the global. Off forces the effect off for this app. Force opaque removes transparency entirely (overrides Opacity).")
                                color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: 12
                            }
                        }

                        OvChoice {
                            width: parent.width
                            label: I18n.tr("Blur")
                            value: card.modelData.blur || "inherit"
                            altKey: "off"; altLabel: I18n.tr("Off")
                            onChose: (k) => pg.patch(card.index, "blur", k)
                        }
                        OvChoice {
                            width: parent.width
                            label: I18n.tr("Shadow")
                            value: card.modelData.shadow || "inherit"
                            altKey: "off"; altLabel: I18n.tr("Off")
                            onChose: (k) => pg.patch(card.index, "shadow", k)
                        }
                        OvChoice {
                            width: parent.width
                            label: I18n.tr("Dim inactive")
                            value: card.modelData.dim || "inherit"
                            altKey: "off"; altLabel: I18n.tr("Off")
                            onChose: (k) => pg.patch(card.index, "dim", k)
                        }
                        OvChoice {
                            width: parent.width
                            label: I18n.tr("Animations")
                            value: card.modelData.anim || "inherit"
                            altKey: "off"; altLabel: I18n.tr("Off")
                            onChose: (k) => pg.patch(card.index, "anim", k)
                        }
                        OvChoice {
                            width: parent.width
                            label: I18n.tr("Force opaque")
                            value: card.modelData.opaque || "inherit"
                            altKey: "on"; altLabel: I18n.tr("On")
                            onChose: (k) => pg.patch(card.index, "opaque", k)
                        }
                    }
                }
            }
        }
    }

    // ── empty state, gated on load so it does not flash before data arrives ──
    Empty {
        anchors.centerIn: flick
        visible: pg.ready && pg.overrides.length === 0
        caption: I18n.tr("No app overrides yet. Add one, or pick an open window, to give it its own opacity, corners, blur, and more.")
    }

    // ── the open-window catalogue overlay (paperRaised + lineStrong, per spec) ──
    Item {
        id: appPick
        anchors.fill: parent
        visible: false
        z: 50

        function show() { appPick.visible = true; pk.open(); }
        function hide() { appPick.visible = false; }

        Rectangle {
            anchors.fill: parent
            color: Tokens.paper
            opacity: 0.55
            TapHandler { onTapped: appPick.hide() }
        }
        Picker {
            id: pk
            anchors.centerIn: parent
            title: I18n.tr("OPEN WINDOWS")
            options: pg.openClasses
            current: ""
            onChose: (k) => { pg.addApp(k); appPick.hide(); }
            onDismissed: appPick.hide()
        }
    }

    // ── label + Inherit/Custom + a control that appears only when Custom ──
    // The value carries -1 for inherit; switching to Inherit writes -1 over the
    // previous custom value, so returning to Custom always lands on customDefault
    // (the old page's amnesia, reproduced faithfully). Opacity is a ratio, so it
    // draws a Slid over a 0-100 integer domain and stores the fraction; corners
    // and border are bounded integers, so they draw a Step.
    component OvNum: Item {
        id: ov
        property string label: ""
        property real value: -1
        property real from: 0
        property real to: 1
        property real customDefault: 0.9
        property bool percent: false
        signal changed(real v)

        readonly property bool custom: ov.value >= 0
        width: parent ? parent.width : 0
        height: 30

        Text {
            id: ovLbl
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            width: 92
            text: I18n.tr(ov.label)
            color: Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
        }

        Seg {
            id: ovMode
            anchors.left: ovLbl.right
            anchors.leftMargin: Tokens.s2
            anchors.verticalCenter: parent.verticalCenter
            options: [I18n.tr("Inherit"), I18n.tr("Custom")]
            current: ov.custom ? I18n.tr("Custom") : I18n.tr("Inherit")
            onChose: (k) => ov.changed(k === I18n.tr("Custom") ? (ov.value >= 0 ? ov.value : ov.customDefault) : -1)
        }

        Text {
            id: ovVal
            visible: ov.custom
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            width: 46
            horizontalAlignment: Text.AlignRight
            // a presented quantity, so Grotesk (DESIGN.md section 2), not mono.
            text: ov.percent ? Math.round(ov.value * 100) + "%" : Math.round(ov.value).toString()
            color: Tokens.ink
            font.family: Tokens.ui
            font.pixelSize: Tokens.fBody
            font.weight: Font.Light
        }

        Slid {
            visible: ov.custom && ov.percent
            anchors.left: ovMode.right
            anchors.leftMargin: Tokens.s4
            anchors.right: ovVal.left
            anchors.rightMargin: Tokens.s3
            anchors.verticalCenter: parent.verticalCenter
            from: Math.round(ov.from * 100)
            to: Math.round(ov.to * 100)
            value: Math.round(ov.value * 100)
            onModified: (v) => ov.changed(v / 100)
        }
        Step {
            visible: ov.custom && !ov.percent
            anchors.right: ovVal.left
            anchors.rightMargin: Tokens.s3
            anchors.verticalCenter: parent.verticalCenter
            from: ov.from
            to: ov.to
            value: Math.round(ov.value)
            onModified: (v) => ov.changed(v)
        }
    }

    // ── label + a two-member Inherit/Off (or Inherit/On) segmented ──
    // Not a boolean: a 2-member enum of strings ("inherit" and one alt), so the
    // third state (inherit) survives. Blur/shadow/dim/anim use "off"; the odd
    // one out, Force opaque, uses "on".
    component OvChoice: Item {
        id: oc
        property string label: ""
        property string value: "inherit"
        property string altKey: "off"
        property string altLabel: I18n.tr("Off")
        signal chose(string key)

        width: parent ? parent.width : 0
        height: 30

        Text {
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            width: 92
            text: I18n.tr(oc.label)
            color: Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
        }

        Seg {
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            options: [I18n.tr("Inherit"), oc.altLabel]
            current: oc.value === oc.altKey ? oc.altLabel : I18n.tr("Inherit")
            onChose: (k) => oc.chose(k === oc.altLabel ? oc.altKey : "inherit")
        }
    }
}
