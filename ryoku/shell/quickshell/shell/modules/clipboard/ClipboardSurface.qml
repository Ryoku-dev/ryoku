pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Quickshell
import Quickshell.Wayland
import shell.services
import Ryoku.Ui.Singletons
import "../../components"
import "../../utils/fuzzy.js" as Fuzzy

/**
 * once per screen, replacing the old sidebar clipboard page. The card is a
 * shallow landscape split into Clipboard history on the left and Starred on the
 * right. Each entry is a card: an image renders as a cropped preview, text as a
 * long horizontal card, and anything else as mime and size. A card copies on
 * click; its top-right star moves it between the panes and persists across daemon
 * restarts. Super+V toggles the surface, Escape closes it while it has keyboard
 * focus, and a click anywhere off the card dismisses it (the surface is modal
 * while open: no dimming, but it does take the pointer). Nothing maps while it is
 * closed, so the desktop is left alone then.
 *
 * A search field sits beside the Clipboard title and holds the keyboard while
 * the surface is open, so Super+V then typing filters the history with no click.
 * The match is a fuzzy subsequence over the entry's preview (or its mime, so
 * "png" finds a screenshot) ranked by the shared launcher ranker; Escape clears
 * the filter first and closes the surface on the next press, and the query clears
 * on close so a reopen always shows the full history. Starred is never filtered.
 *
 * The card rests on the screen edge (shell.json clipboard.bottomPercent lifts
 * it) and every open and close is one continuous vertical move: it rises out of
 * that edge and settles, and on close drops back through it. `reveal` is the
 * eased 0..1 progress; the surface outlives the raw open flag until the descent
 * has finished, so a close is never cut off half-way.
 *
 * `active` drives the reveal and keyboard grab; the controller binds it to this
 * monitor's ShellState.clipboardOpen. `screen` is the monitor this surface draws
 * on, injected by the per-screen Variants in shell.qml.
 */
Scope {
    id: root

    // The monitor this surface draws on, injected per-screen by shell.qml.
    property var screen: null

    // Reveal input the controller binds to this monitor's ShellState.clipboardOpen.
    property bool active: false

    // Close request (Esc / click-out). The controller resets clipboardOpen; we
    // never assign `active` here, which would destroy its binding to the flag and
    // leave the overlay unable to reopen.
    signal requestClose()

    function hide() {
        root.requestClose();
    }

    readonly property string focusedMon: Wm.focusedOutput

    // Two panes off one warm resident topic: history is everything unstarred,
    // Starred is everything starred. Both recompute when the daemon republishes.
    readonly property var historyEntries: (Clipboard.entries || []).filter(e => e && e.starred !== true)
    readonly property var starredEntries: (Clipboard.entries || []).filter(e => e && e.starred === true)

    // Free-text filter over the history. The query must appear as an ordered
    // subsequence of the entry's text -- its preview for a text entry, its mime
    // otherwise -- and the launcher's tiers rank it (prefix, then substring, then
    // scatter); equal tiers keep the daemon's newest-first order. The search
    // belongs to the pane whose header owns it, so Starred is never filtered.
    property string query: ""
    readonly property bool searching: root.query.trim().length > 0
    readonly property var shownHistory: {
        const items = root.historyEntries;
        const q = root.query.trim().toLowerCase();
        if (q.length === 0)
            return items;
        const hits = [];
        for (let i = 0; i < items.length; i++) {
            const e = items[i];
            const text = e.kind === "text" ? String(e.preview || "") : String(e.mime || "");
            const score = Fuzzy.score({ name: text, keywords: [String(e.mime || ""), String(e.kind || "")] }, q);
            if (score < 99)
                hits.push({ entry: e, score: score, at: i });
        }
        hits.sort((a, b) => a.score !== b.score ? a.score - b.score : a.at - b.at);
        return hits.map(h => h.entry);
    }

    readonly property var layout: Config.clipboard || ({})
    readonly property real widthPercent: Math.max(40, Math.min(90, root.layout.widthPercent === undefined ? 65 : Number(root.layout.widthPercent)))
    readonly property real heightPercent: Math.max(24, Math.min(72, root.layout.heightPercent === undefined ? 42 : Number(root.layout.heightPercent)))
    readonly property real bottomPercent: Math.max(0, Math.min(20, root.layout.bottomPercent === undefined ? 0 : Number(root.layout.bottomPercent)))
    readonly property real panelRadius: Math.max(0, Math.min(32, root.layout.panelRadius === undefined ? 18 : Number(root.layout.panelRadius)))
    readonly property real paneRadius: Math.max(0, Math.min(32, root.layout.paneRadius === undefined ? 12 : Number(root.layout.paneRadius)))
    readonly property real cardRadius: Math.max(0, Math.min(32, root.layout.cardRadius === undefined ? 9 : Number(root.layout.cardRadius)))

    // A single pane: header (icon, title, count), a hairline, the card list, and
    // a compact actionable empty state. `s` scales it; ids inside stay local, so
    // scale and the close request are passed in rather than reaching outward.
    component Pane: Rectangle {
        id: pane

        property real s: 1
        property string title: ""
        property string glyph: ""
        property var items: []
        property string emptyGlyph: ""
        property string emptyText: ""
        // The pane that owns a search field: its header carries the input, bound
        // to the surface's query. Panes without it (Starred) are never filtered.
        property bool searchable: false
        // The surface is open on this pane's monitor, so the field takes the
        // keyboard: Super+V then typing filters with no click, and the field's own
        // Escape unwinds the filter before the surface.
        property bool hot: false
        property string query: ""
        signal queryEdited(string text)
        signal closeRequested()

        radius: Math.round(root.paneRadius * pane.s)
        color: Qt.rgba(Theme.surface.r, Theme.surface.g, Theme.surface.b, 0.35)
        border.width: Theme.borderWidth
        border.color: Theme.outlineVariant

        Item {
            id: hdr
            anchors.top: parent.top
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.margins: Math.round(14 * pane.s)
            height: Math.round(26 * pane.s)

            Row {
                id: titleRow
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                spacing: Math.round(8 * pane.s)

                MaterialIcon {
                    anchors.verticalCenter: parent.verticalCenter
                    text: pane.glyph
                    fill: 0
                    font.pixelSize: Math.round(Theme.iconSm * pane.s)
                    color: Theme.onSurface
                }
                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: pane.title
                    color: Theme.onSurface
                    font.family: Theme.fontPrimary
                    font.pixelSize: Math.round(Theme.fontMd * pane.s)
                    font.weight: Font.DemiBold
                }
            }

            // The pane's own search field (Clipboard only): the history filters as
            // it is typed. Escape unwinds it in stages -- the text first, then the
            // surface -- so a mistyped search never costs the panel.
            Rectangle {
                id: field
                visible: pane.searchable
                anchors.left: titleRow.right
                anchors.leftMargin: Math.round(14 * pane.s)
                anchors.verticalCenter: parent.verticalCenter
                height: Math.round(24 * pane.s)
                radius: height / 2
                // Compact: it grows with the pane but never crowds the count chip.
                // Plain x/width arithmetic -- anchor lines do not subtract.
                width: Math.min(Math.round(260 * pane.s),
                                Math.max(Math.round(96 * pane.s),
                                         countChip.x - (titleRow.x + titleRow.width) - Math.round(24 * pane.s)))
                color: Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, input.activeFocus ? 0.10 : 0.05)
                border.width: 1
                border.color: input.activeFocus
                    ? Qt.rgba(Theme.primary.r, Theme.primary.g, Theme.primary.b, 0.75)
                    : Theme.outlineVariant
                Behavior on color { ColorAnimation { duration: Motion.rowFade; easing.type: Motion.easeType; easing.bezierCurve: Motion.easeCurve } }
                Behavior on border.color { ColorAnimation { duration: Motion.rowFade; easing.type: Motion.easeType; easing.bezierCurve: Motion.easeCurve } }

                MaterialIcon {
                    id: mag
                    anchors.left: parent.left
                    anchors.leftMargin: Math.round(9 * pane.s)
                    anchors.verticalCenter: parent.verticalCenter
                    text: "search"
                    fill: 0
                    font.pixelSize: Math.round(Theme.iconSm * pane.s * 0.8)
                    color: input.activeFocus ? Theme.onSurfaceVariant : Qt.rgba(Theme.onSurfaceVariant.r, Theme.onSurfaceVariant.g, Theme.onSurfaceVariant.b, 0.75)
                }

                Text {
                    anchors.left: input.left
                    anchors.right: input.right
                    anchors.verticalCenter: parent.verticalCenter
                    visible: input.text.length === 0
                    text: I18n.tr("Search")
                    color: Qt.rgba(Theme.onSurfaceVariant.r, Theme.onSurfaceVariant.g, Theme.onSurfaceVariant.b, 0.7)
                    font.family: Theme.fontPrimary
                    font.pixelSize: input.font.pixelSize
                    elide: Text.ElideRight
                }

                TextInput {
                    id: input
                    focus: pane.searchable && pane.hot
                    anchors.left: mag.right
                    anchors.leftMargin: Math.round(6 * pane.s)
                    anchors.right: clear.left
                    anchors.rightMargin: Math.round(6 * pane.s)
                    anchors.verticalCenter: parent.verticalCenter
                    color: Theme.onSurface
                    font.family: Theme.fontPrimary
                    font.pixelSize: Math.round((Theme.fontSm - 3) * pane.s)
                    selectionColor: Qt.rgba(Theme.primary.r, Theme.primary.g, Theme.primary.b, 0.4)
                    selectedTextColor: Theme.onSurface
                    selectByMouse: true
                    clip: true
                    verticalAlignment: TextInput.AlignVCenter
                    // onTextChanged, not onTextEdited: the clear button and the
                    // surface's reset write the text programmatically and the pane
                    // must follow those too.
                    onTextChanged: pane.queryEdited(input.text)
                    Keys.onEscapePressed: (e) => {
                        if (input.text.length > 0)
                            input.text = "";
                        else
                            pane.closeRequested();
                        e.accepted = true;
                    }
                }

                // A programmatic query (the surface clears it on close) lands back
                // in the field. A user edit already matches, so this cannot loop.
                Connections {
                    target: pane
                    function onQueryChanged() { if (input.text !== pane.query) input.text = pane.query }
                }

                MaterialIcon {
                    id: clear
                    anchors.right: parent.right
                    anchors.rightMargin: Math.round(8 * pane.s)
                    anchors.verticalCenter: parent.verticalCenter
                    visible: input.text.length > 0
                    text: "close"
                    fill: 0
                    font.pixelSize: Math.round(Theme.iconSm * pane.s * 0.75)
                    color: clearArea.containsMouse ? Theme.onSurface : Theme.onSurfaceVariant
                    MouseArea {
                        id: clearArea
                        anchors.fill: parent
                        anchors.margins: -Math.round(4 * pane.s)
                        hoverEnabled: true
                        cursorShape: Qt.PointingHandCursor
                        onClicked: {
                            input.text = "";
                            input.forceActiveFocus();
                        }
                    }
                }
            }

            Rectangle {
                id: countChip
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                visible: pane.items.length > 0
                height: Math.round(20 * pane.s)
                width: Math.max(height, cnt.implicitWidth + Math.round(12 * pane.s))
                radius: height / 2
                color: Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.10)

                Text {
                    id: cnt
                    anchors.centerIn: parent
                    text: pane.items.length
                    color: Theme.onSurfaceVariant
                    font.family: Theme.fontPrimary
                    font.pixelSize: Math.round((Theme.fontSm - 2) * pane.s)
                    font.weight: Font.DemiBold
                }
            }
        }

        Rectangle {
            id: rule
            anchors.top: hdr.bottom
            anchors.topMargin: Math.round(10 * pane.s)
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.leftMargin: Math.round(14 * pane.s)
            anchors.rightMargin: Math.round(14 * pane.s)
            height: 1
            color: Theme.outlineVariant
            opacity: 0.6
            visible: pane.items.length > 0
        }

        ListView {
            id: cards
            anchors.top: rule.bottom
            anchors.topMargin: Math.round(10 * pane.s)
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            anchors.leftMargin: Math.round(14 * pane.s)
            add: Transition {
                NumberAnimation { properties: "x,y"; duration: 0 }
            }
            move: Transition {
                NumberAnimation { properties: "x,y"; duration: 0 }
            }
            remove: Transition {
                NumberAnimation { properties: "x,y"; duration: 0 }
            }
            anchors.rightMargin: Math.round(14 * pane.s)
            anchors.bottomMargin: Math.round(14 * pane.s)
            clip: true
            spacing: Math.round(10 * pane.s)
            model: pane.items
            reuseItems: true
            cacheBuffer: Math.max(0, height)
            boundsBehavior: Flickable.StopAtBounds
            visible: pane.items.length > 0

            delegate: ClipboardCard {
                required property var modelData
                width: ListView.view.width
                s: pane.s
                radius: Math.round(root.cardRadius * pane.s)
                entry: modelData
                onCopyRequested: {
                    Clipboard.copy(Number(modelData.id));
                    pane.closeRequested();
                }
                onStarRequested: Clipboard.star(Number(modelData.id), !(modelData.starred === true))
                onDeleteRequested: Clipboard.del(Number(modelData.id))
            }
        }

        Column {
            anchors.centerIn: parent
            width: parent.width - Math.round(40 * pane.s)
            spacing: Math.round(8 * pane.s)
            visible: pane.items.length === 0

            MaterialIcon {
                anchors.horizontalCenter: parent.horizontalCenter
                text: pane.emptyGlyph
                fill: 0
                font.pixelSize: Math.round(Theme.iconLg * pane.s)
                color: Qt.rgba(Theme.onSurfaceVariant.r, Theme.onSurfaceVariant.g, Theme.onSurfaceVariant.b, 0.5)
            }
            Text {
                width: parent.width
                horizontalAlignment: Text.AlignHCenter
                text: pane.emptyText
                wrapMode: Text.WordWrap
                color: Theme.onSurfaceVariant
                font.family: Theme.fontPrimary
                font.pixelSize: Math.round(Theme.fontSm * pane.s)
            }
        }
    }

    PanelWindow {
        id: win

        readonly property real s: Math.min(1.25, (root.screen ? root.screen.height / 1080 : 1)) * Math.max(0.8, Math.min(1.4, Config.fontScale)) * Tokens.uiScaleFor(root.screen ? root.screen.name : "")
        readonly property bool isFocused: !root.focusedMon || root.focusedMon === (root.screen ? root.screen.name : "")
        readonly property bool shown: root.active

        screen: root.screen
        color: "transparent"
        exclusionMode: ExclusionMode.Ignore
        WlrLayershell.namespace: "clipboard"
        WlrLayershell.layer: WlrLayer.Overlay
        WlrLayershell.keyboardFocus: (shown && isFocused) ? WlrKeyboardFocus.OnDemand : WlrKeyboardFocus.None
        // Full-screen while open, so a click anywhere off the card can dismiss it:
        // the panel is one card inside a modal layer rather than a tight surface
        // with no way to catch an outside press. Nothing maps while closed, so the
        // desktop stays interactive then.
        anchors { top: true; bottom: true; left: true; right: true }

        // How far the card travels: its own height, so a hidden card is entirely
        // past the panel's resting edge and the screen cuts it away.
        readonly property real slide: bodyWrap.bodyH

        // Eased reveal, 0 = fully past the edge, 1 = at rest. Driven by hand
        // rather than a Behavior so the frame the surface maps on is known to be
        // the hidden offset -- the card is never flashed at rest for a frame
        // before the move starts -- and so an interrupted close resumes from
        // wherever it had reached instead of restarting.
        property real reveal: 0
        onShownChanged: {
            rise.stop();
            fall.stop();
            if (shown) {
                rise.from = win.reveal;
                rise.start();
            } else {
                // A reopen always shows the whole history: a filter left behind
                // would silently hide entries the user came back for.
                root.query = "";
                fall.from = win.reveal;
                fall.start();
            }
        }

        // Mapped for the whole move: the exit animates the card off the edge, so
        // the layer outlives the open flag until that has finished.
        visible: shown || reveal > 0.001

        // Outside click dismisses. The card sits above this, and swallows presses
        // over its own chrome, so only a press off the card closes the surface.
        MouseArea {
            id: dismiss
            anchors.fill: parent
            acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
            onPressed: root.hide()
        }

        // The card, centred on the bottom edge of the monitor it draws on.
        Item {
            id: bodyWrap
            readonly property real bodyW: Math.min(1100 * win.s, (root.screen ? root.screen.width : 1600) * root.widthPercent / 100)
            readonly property real bodyH: Math.min((root.screen ? root.screen.height : 900) * root.heightPercent / 100, bodyWrap.bodyW * 0.52)

            width: bodyWrap.bodyW
            height: bodyWrap.bodyH
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.bottom: parent.bottom
            anchors.bottomMargin: Math.round((root.screen ? root.screen.height : 900) * root.bottomPercent / 100)

            // The panel's own area swallows presses: tapping its chrome (the gap
            // between cards, the header's dead space) must not dismiss the
            // surface, only a click off the card may.
            MouseArea {
                anchors.fill: parent
                acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
            }

            // One vertical move rides `win.reveal`: the card rises out of the
            // screen edge and settles on the emphasized curve, and drops back
            // through the edge on the shorter exit curve. Opacity and scale stay
            // fixed, so nothing about the contents fades or re-settles.
            transform: Translate { y: Math.round((1 - win.reveal) * win.slide) }

            // The two halves of that move. They live inside the card so a stopped
            // one leaves the reveal where it was; the value is read, never
            // re-bound.
            NumberAnimation {
                id: rise
                target: win
                property: "reveal"
                to: 1
                duration: Motion.sidebarEnter
                easing.type: Easing.BezierSpline
                easing.bezierCurve: Motion.sidebarEnterCurve
            }
            NumberAnimation {
                id: fall
                target: win
                property: "reveal"
                to: 0
                duration: Motion.sidebarExit
                easing.type: Easing.BezierSpline
                easing.bezierCurve: Motion.sidebarExitCurve
            }

            // The soft 20px pass is a gaussian the size of the card, and the
            // card slides, so it re-renders on every frame it is shown. Gate it
            // to at-rest: the crisp 7px pass carries the card through the
            // animation and the soft one appears the moment it settles. The cut
            // reads as the stop, not a flicker; fading it would re-render the
            // blur every frame, which is the cost this gates away.
            RectangularShadow {
                anchors.fill: cardBg
                radius: cardBg.radius
                blur: Math.round(20 * win.s)
                spread: 1
                offset: Qt.vector2d(0, Math.round(8 * win.s))
                color: Qt.rgba(0, 0, 0, 0.32)
                z: -2
                visible: win.reveal >= 0.999
            }

            // The tight 7px pass always rides the card: it is cheap, and it is
            // what keeps the sliding card grounded.
            RectangularShadow {
                anchors.fill: cardBg
                radius: cardBg.radius
                blur: Math.round(7 * win.s)
                spread: 0
                offset: Qt.vector2d(0, Math.round(2 * win.s))
                color: Qt.rgba(0, 0, 0, 0.42)
                z: -1
            }

            Rectangle {
                id: cardBg
                anchors.fill: parent
                radius: Math.round(root.panelRadius * win.s)
                color: Qt.rgba(Theme.surface.r, Theme.surface.g, Theme.surface.b, 0.9)
                border.width: Theme.borderWidth
                border.color: Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.12)

                Row {
                    anchors.fill: parent
                    anchors.margins: Math.round(18 * win.s)
                    spacing: Math.round(16 * win.s)

                    Pane {
                        id: historyPane
                        width: (parent.width - parent.spacing) * 0.6
                        height: parent.height
                        s: win.s
                        glyph: "content_paste"
                        title: I18n.tr("Clipboard")
                        searchable: true
                        hot: win.shown && win.isFocused
                        query: root.query
                        onQueryEdited: text => root.query = text
                        items: root.shownHistory
                        emptyGlyph: root.searching ? "search_off" : "content_paste_off"
                        emptyText: root.searching
                            ? I18n.tr("Nothing here matches that search.")
                            : I18n.tr("Copy something and it lands here.")
                        onCloseRequested: root.hide()
                    }

                    Pane {
                        width: (parent.width - parent.spacing) * 0.4
                        height: parent.height
                        s: win.s
                        glyph: "star"
                        title: I18n.tr("Starred")
                        items: root.starredEntries
                        emptyGlyph: "star_outline"
                        emptyText: I18n.tr("Tap the star on an item to keep it here.")
                        onCloseRequested: root.hide()
                    }
                }
            }
        }

    }
}
