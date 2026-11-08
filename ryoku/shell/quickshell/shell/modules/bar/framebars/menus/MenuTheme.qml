pragma ComponentBehavior: Bound

import QtQuick
import "../.." as Pill
import shell.services
import "../../../../components"
import Ryoku.Ui as RyokuUi
import Ryoku.Ui.Singletons

// The colour-scheme picker (contract 08 sec 2.2/2.3/4.2): a horizontally
// scrolling row of theme cards. Each card is a 100-wide, 120-tall rounded
// preview (radius 6) filled with the theme's surface colour, carrying the theme
// name in its own on-surface colour, a two-by-three grid of 16x16 swatches, and
// a check badge on the selected one. A single click applies the theme live.
//
// WHERE THE CATALOG LIVES. The static-theme palettes belong to the Go theme
// layer, not to QML: Theme.qml already documents `namedScheme` as the seam the
// daemon drives with the active preset's full 30-role palette, and applying a
// theme is a single settings write (theme.theme) that the daemon's style and
// wallpaper reactors consume (contract 08 sec 3.1/4.2). This picker only needs a
// lightweight preview projection -- label, dark/light and seven swatch colours
// per theme -- to draw the cards. That projection is the sole extension point
// below (`schemeSource`). It is deliberately the ONE place the 57-row table is
// materialised on the QML side; when a Go `theme` topic serves the catalog,
// bind `schemes` to the subscribed frame and delete the literal, and nothing
// else in this file changes. The full recolour stays Go's job through
// Theme.namedScheme; the picker owns only the selection intent and its badge.
Item {
    id: root

    property real s: 1
    property bool open: false
    signal requestClose()

    // The preview catalog the cards render from (the extension point). A future
    // Go `theme` topic replaces this with a subscribed frame.
    readonly property var schemes: schemeSource.catalog

    property string activeId: ""

    implicitHeight: col.implicitHeight

    // Apply a scheme: write theme.theme through the settings seam (contract 14).
    // The daemon validates, persists and broadcasts, then its style reactor
    // recolours the shell via Theme.namedScheme and its wallpaper reactor
    // re-tints the desktop. Selecting never dismisses the menu.
    function select(id) {
        if (id.length === 0)
            return;
        root.activeId = id;
        ShellSettings.patch("theme.theme", id);
    }

    function syncActive() {
        if (typeof appliedTheme.value === "string")
            root.activeId = appliedTheme.value;
    }
    RyokuUi.ShellSetting {
        id: appliedTheme
        path: "theme.theme"
        fallback: null
        onValueChanged: root.syncActive()
    }
    Component.onCompleted: root.syncActive()

    Column {
        id: col
        width: parent.width
        spacing: 12

        // Section title (contract 08 sec 2.2).
        Text {
            text: I18n.tr("Color Scheme")
            color: Theme.onSurface
            font.family: Theme.fontPrimary
            font.pixelSize: Theme.fontXl
            font.weight: Font.Bold
        }

        // The card strip: fixed 100-wide cards, 8px apart, inset 32px from each
        // end, scrolling horizontally (contract 08 sec 2.2).
        ListView {
            id: strip
            width: parent.width
            height: 120
            clip: true
            orientation: ListView.Horizontal
            spacing: 8
            leftMargin: 32
            rightMargin: 32
            cacheBuffer: 1200
            boundsBehavior: Flickable.StopAtBounds
            interactive: contentWidth > width
            model: root.schemes
            delegate: Card {}

            // Vertical wheel drives horizontal scroll, 64px per notch, matching
            // the wallpaper grid (contract 08 sec 2.2).
            WheelHandler {
                acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
                onWheel: event => {
                    const step = (event.angleDelta.y / 120) * 64;
                    const max = Math.max(0, strip.contentWidth + strip.leftMargin + strip.rightMargin - strip.width);
                    strip.contentX = Math.max(0, Math.min(max, strip.contentX - step));
                }
            }
        }
    }

    // One theme card. `sw` is the seven-swatch projection in reference order
    // [surface, onSurface, primary, secondary, tertiary, error, outline];
    // dynamic themes (Default, Wallpaper) carry no palette and show a glyph.
    component Card: Rectangle {
        id: card
        required property var modelData
        readonly property bool dynamic: modelData.dynamic === true
        readonly property var sw: card.dynamic ? [] : modelData.sw
        // the five colour-combo pills: the theme's ink plus its four accents.
        readonly property var pills: !card.dynamic && card.sw && card.sw.length >= 6
            ? [card.sw[1], card.sw[2], card.sw[3], card.sw[4], card.sw[5]] : []

        width: 100
        height: 120
        radius: Theme.radiusWidget
        color: "transparent"
        border.width: Theme.borderWidth
        // Outline at rest, on-surface on hover (the shell's roles; contract 08
        // sec 2.3); selection shows the badge, not a border change.
        border.color: cardHov.hovered ? Theme.onSurface : Theme.outline

        // The 120-tall preview: a radius-6 rounded rect filled with the theme's
        // own surface colour (dynamic themes use the shell surface).
        Rectangle {
            id: preview
            anchors.fill: parent
            anchors.margins: Theme.borderWidth
            radius: 6
            color: card.dynamic ? Theme.surface : card.sw[0]
            clip: true

            // Theme name, top-centred, in the theme's own on-surface colour.
            Text {
                id: cardName
                anchors { top: parent.top; left: parent.left; right: parent.right; margins: 4 }
                horizontalAlignment: Text.AlignHCenter
                wrapMode: Text.Wrap
                text: I18n.tr(card.modelData.label)
                color: card.dynamic ? Theme.onSurface : card.sw[1]
                font.family: Theme.fontPrimary
                font.pixelSize: Theme.fontSm
            }

            // Dynamic themes render a 60px glyph instead of swatches.
            MaterialIcon {
                visible: card.dynamic
                anchors.centerIn: parent
                text: card.dynamic ? card.modelData.icon : ""
                font.pixelSize: 60
                color: Theme.onSurface
            }

            // A centred row of tall stadium pills of the theme's key roles
            // [onSurface, primary, secondary, tertiary, error] -- the colour combo.
            Item {
                id: cardPills
                visible: !card.dynamic && cardPills.count > 0
                readonly property int count: card.pills.length
                anchors.top: cardName.bottom
                anchors.bottom: parent.bottom
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.topMargin: 8
                anchors.bottomMargin: 12
                anchors.leftMargin: 12
                anchors.rightMargin: 12
                readonly property real gap: 6
                readonly property real pillW: cardPills.count > 0 ? (width - gap * (cardPills.count - 1)) / cardPills.count : 0
                Row {
                    anchors.centerIn: parent
                    height: parent.height
                    spacing: cardPills.gap
                    Repeater {
                        model: card.pills
                        delegate: Rectangle {
                            required property color modelData
                            width: cardPills.pillW
                            height: parent.height
                            radius: width / 2
                            color: modelData
                        }
                    }
                }
            }

            // Selected badge: a filled check, top-right, clear of the pills, in
            // the theme's own on-surface colour.
            MaterialIcon {
                visible: root.activeId === card.modelData.id
                anchors { right: parent.right; top: parent.top; rightMargin: 6; topMargin: 6 }
                text: "check_circle"
                fill: 1
                font.pixelSize: 20
                color: card.dynamic ? Theme.onSurface : card.sw[1]
            }
        }

        HoverHandler { id: cardHov; cursorShape: Qt.PointingHandCursor }
        TapHandler { onTapped: root.select(card.modelData.id) }
    }

    // --- extension point: the static-theme preview catalog (contract 08 sec 8.1)
    // Two dynamic variants (icon cards, excluded from any dark/light filter) then
    // the 57 static themes, each `sw` = [surface, onSurface, primary, secondary,
    // tertiary, error, outline]. Lifted verbatim from the resolved contract; the
    // Go theme layer owns the authoritative table and the full 30-role palettes.
    QtObject {
        id: schemeSource
        readonly property var catalog: [
            { id: "Default", label: I18n.tr("Default"), dynamic: true, icon: "palette" },
            { id: "Wallpaper", label: I18n.tr("Wallpaper"), dynamic: true, icon: "wallpaper" },
            { id: "Catppuccin Mocha", label: "Catppuccin Mocha", dark: true, sw: ["#1e1e2e", "#cdd6f4", "#b4befe", "#f2cdcd", "#94e2d5", "#f38ba8", "#a6adc8"] },
            { id: "Dracula", label: "Dracula", dark: true, sw: ["#282A36", "#F8F8F2", "#BD93F9", "#FF79C6", "#8BE9FD", "#FF5555", "#6272A4"] },
            { id: "Everforest Dark Medium", label: "Everforest Dark Medium", dark: true, sw: ["#232A2E", "#D3C6AA", "#A7C080", "#7FBBB3", "#83C092", "#E67E80", "#7A8478"] },
            { id: "Gruvbox Dark Medium", label: "Gruvbox Dark Medium", dark: true, sw: ["#282828", "#EBDBB2", "#83A598", "#B8BB26", "#8EC07C", "#FB4934", "#928374"] },
            { id: "Kanagawa Wave", label: "Kanagawa Wave", dark: true, sw: ["#1F1F28", "#DCD7BA", "#7E9CD8", "#957FB8", "#7AA89F", "#E82424", "#938AA9"] },
            { id: "Nord Dark", label: "Nord Dark", dark: true, sw: ["#2E3440", "#ECEFF4", "#88C0D0", "#81A1C1", "#8FBCBB", "#BF616A", "#4C566A"] },
            { id: "One Dark", label: "One Dark", dark: true, sw: ["#282C34", "#ABB2BF", "#61AFEF", "#C678DD", "#56B6C2", "#E06C75", "#636D83"] },
            { id: "Rose Pine", label: "Rosé Pine", dark: true, sw: ["#191724", "#E0DEF4", "#C4A7E7", "#9CCFD8", "#EBBCBA", "#EB6F92", "#6E6A86"] },
            { id: "Solarized Dark", label: "Solarized Dark", dark: true, sw: ["#002b36", "#93a1a1", "#268bd2", "#2aa198", "#b58900", "#dc322f", "#839496"] },
            { id: "Tokyo Night", label: "Tokyo Night", dark: true, sw: ["#1a1b26", "#a9b1d6", "#7aa2f7", "#bb9af7", "#73daca", "#f7768e", "#9aa5ce"] }
        ]
    }
}
