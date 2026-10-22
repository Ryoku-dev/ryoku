pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Quickshell
import shell.services

// Static per-item chrome for every dock form except islands (whose pill lives in
// DockItem's magnified content). This layer is a child of the item, not its
// transformed content, so a rail underline, ledger cell or tanzaku strip stays a
// fixed part of the sheet while only the icon rises under the cursor. Each form
// is one small Component the Loader swaps in when `dock.style` changes.
Item {
    id: chrome

    required property var item
    required property var band

    readonly property string style: chrome.band.style
    readonly property bool horizontal: chrome.item.horizontal
    readonly property string edge: chrome.item.edge

    // outer = the screen-edge side; inner is the desktop-facing one.
    readonly property bool outerTop: chrome.horizontal && chrome.edge === "top"
    readonly property bool outerBottom: chrome.horizontal && chrome.edge === "bottom"
    readonly property bool outerLeft: !chrome.horizontal && chrome.edge === "left"
    readonly property bool outerRight: !chrome.horizontal && chrome.edge === "right"

    // Place across the whole run so the ledger sheet knows its own two ends.
    readonly property int total: chrome.band.pins.length + chrome.band.running.length
    readonly property bool firstCell: chrome.item.ordinal === 0
    readonly property bool lastCell: chrome.item.ordinal === chrome.total - 1

    Loader {
        anchors.fill: parent
        sourceComponent: chrome.style === "rail" ? railC
            : chrome.style === "ledger" ? ledgerC
            : chrome.style === "tanzaku" ? tanzakuC
            : chrome.style === "seal" ? sealC
            : chrome.style === "capsule" ? capsuleC
            : null
    }

    // ── rail: a bone underline on the outer edge the width of the cell, its
    // thickness the window count (1 px for one window, 2 px for more) ──────────
    Component {
        id: railC
        Item {
            id: rail
            anchors.fill: parent
            readonly property real inset: 10
            readonly property real thick: chrome.item.count === 1 ? 1 : 2
            Rectangle {
                visible: chrome.item.count >= 1
                color: chrome.band.bone
                width: chrome.horizontal ? chrome.band.baseSize - rail.inset : rail.thick
                height: chrome.horizontal ? rail.thick : chrome.band.baseSize - rail.inset
                anchors.horizontalCenter: chrome.horizontal ? rail.horizontalCenter : undefined
                anchors.verticalCenter: chrome.horizontal ? undefined : rail.verticalCenter
                anchors.top: chrome.outerTop ? rail.top : undefined
                anchors.bottom: chrome.outerBottom ? rail.bottom : undefined
                anchors.left: chrome.outerLeft ? rail.left : undefined
                anchors.right: chrome.outerRight ? rail.right : undefined
            }
        }
    }

    // ── ledger: a numbered cell -- a leading hairline divider, a mono index in
    // the inner corner, and a bone inversion when the app is active ────────────
    Component {
        id: ledgerC
        Item {
            id: ledger
            anchors.fill: parent
            readonly property real r: chrome.band.radius

            // Active cell inverts; rounded only at the corners it shares with the
            // plate ends, so the fill never pokes past a rounded plate corner.
            Rectangle {
                visible: chrome.item.isActive
                anchors.fill: parent
                color: chrome.band.bone
                topLeftRadius:     chrome.firstCell ? ledger.r : 0
                bottomRightRadius: chrome.lastCell ? ledger.r : 0
                topRightRadius:    (chrome.horizontal ? chrome.lastCell : chrome.firstCell) ? ledger.r : 0
                bottomLeftRadius:  (chrome.horizontal ? chrome.firstCell : chrome.lastCell) ? ledger.r : 0
                Behavior on color { enabled: chrome.band.animate; ColorAnimation { duration: Motion.thumbHover } }
            }

            // Leading divider: between adjacent cells, but not at the run start
            // and not across the separator (which draws that boundary itself).
            Rectangle {
                visible: chrome.item.ordinal > 0
                    && !(chrome.band.sepShown && chrome.item.ordinal === chrome.band.pins.length)
                color: chrome.band.hairline
                width: chrome.horizontal ? 1 : chrome.band.baseSize
                height: chrome.horizontal ? chrome.band.baseSize : 1
                x: chrome.horizontal ? -chrome.band.gap / 2 : 0
                y: chrome.horizontal ? 0 : -chrome.band.gap / 2
            }

            Text {
                text: (chrome.item.ordinal + 1 < 10 ? "0" : "") + (chrome.item.ordinal + 1)
                color: chrome.item.isActive ? chrome.band.inkOnBone : chrome.band.inkFaint
                font.family: Theme.mono
                font.pixelSize: 8
                anchors.margins: 4
                anchors.left:   (chrome.horizontal || chrome.outerRight) ? ledger.left : undefined
                anchors.right:  (!chrome.horizontal && chrome.outerLeft) ? ledger.right : undefined
                anchors.top:    (chrome.outerBottom || !chrome.horizontal) ? ledger.top : undefined
                anchors.bottom: chrome.outerTop ? ledger.bottom : undefined
            }
        }
    }

    // ── tanzaku: a hanging paper strip, square on the screen-edge side and
    // rounded on the inner one, running to the screen edge; a two-letter mono
    // abbreviation at its foot; the whole strip inverts when the app is running ─
    Component {
        id: tanzakuC
        Item {
            id: tanzaku
            anchors.fill: parent
            readonly property bool run: chrome.item.count >= 1
            readonly property real r: chrome.band.radius
            readonly property string appName: {
                const e = DesktopEntries.heuristicLookup(chrome.item.className);
                return (e && e.name) ? e.name : chrome.item.className;
            }
            readonly property string abbr: tanzaku.appName.substring(0, 2).toUpperCase()

            Rectangle {
                id: strip
                anchors.fill: parent
                anchors.topMargin:    chrome.outerTop ? -chrome.band.edgeReach : 0
                anchors.bottomMargin: chrome.outerBottom ? -chrome.band.edgeReach : 0
                anchors.leftMargin:   chrome.outerLeft ? -chrome.band.edgeReach : 0
                anchors.rightMargin:  chrome.outerRight ? -chrome.band.edgeReach : 0
                color: tanzaku.run ? chrome.band.bone
                    : Qt.rgba(Theme.surfaceContainer.r, Theme.surfaceContainer.g, Theme.surfaceContainer.b, chrome.band.frost ? 0.6 : 0.94)
                border.width: 1
                border.color: chrome.band.hairline
                topLeftRadius:     (chrome.outerBottom || chrome.outerRight) ? tanzaku.r : 0
                topRightRadius:    (chrome.outerBottom || chrome.outerLeft) ? tanzaku.r : 0
                bottomLeftRadius:  (chrome.outerTop || chrome.outerRight) ? tanzaku.r : 0
                bottomRightRadius: (chrome.outerTop || chrome.outerLeft) ? tanzaku.r : 0
                Behavior on color { enabled: chrome.band.animate; ColorAnimation { duration: Motion.thumbHover } }

                RectangularShadow {
                    anchors.fill: parent
                    radius: tanzaku.r
                    blur: 12
                    spread: 0
                    offset: chrome.horizontal
                        ? Qt.vector2d(0, chrome.edge === "bottom" ? -2 : 2)
                        : Qt.vector2d(chrome.edge === "right" ? -2 : 2, 0)
                    color: Qt.rgba(Theme.shadow.r, Theme.shadow.g, Theme.shadow.b, 0.5)
                    visible: chrome.band.shadow && !Perf.shadowsDisabled
                    z: -1
                }
            }

            // Anchored to the extended strip so the abbreviation sits at the foot,
            // clear of the icon centred in the cell above it.
            Text {
                text: tanzaku.abbr
                color: tanzaku.run ? chrome.band.inkOnBone : chrome.band.inkFaint
                font.family: Theme.mono
                font.pixelSize: 9
                font.weight: Font.Medium
                anchors.margins: 3
                anchors.horizontalCenter: chrome.horizontal ? strip.horizontalCenter : undefined
                anchors.verticalCenter: chrome.horizontal ? undefined : strip.verticalCenter
                anchors.top: chrome.outerTop ? strip.top : undefined
                anchors.bottom: chrome.outerBottom ? strip.bottom : undefined
                anchors.left: chrome.outerLeft ? strip.left : undefined
                anchors.right: chrome.outerRight ? strip.right : undefined
            }
        }
    }

    // ── seal: a square plate with a hairline border and no rounding; the running
    // mark is the icon's colour (drawn by the item), and the active app carries a
    // bone corner tick ─────────────────────────────────────────────────────────
    Component {
        id: sealC
        Item {
            id: seal
            anchors.fill: parent
            Rectangle {
                id: sealPlate
                anchors.fill: parent
                radius: 0
                color: Qt.rgba(Theme.surfaceContainer.r, Theme.surfaceContainer.g, Theme.surfaceContainer.b, chrome.band.frost ? 0.6 : 0.94)
                border.width: 1
                border.color: chrome.band.hairline

                RectangularShadow {
                    anchors.fill: parent
                    radius: 0
                    blur: 12
                    spread: 0
                    offset: chrome.horizontal
                        ? Qt.vector2d(0, chrome.edge === "bottom" ? -2 : 2)
                        : Qt.vector2d(chrome.edge === "right" ? -2 : 2, 0)
                    color: Qt.rgba(Theme.shadow.r, Theme.shadow.g, Theme.shadow.b, 0.5)
                    visible: chrome.band.shadow && !Perf.shadowsDisabled
                    z: -1
                }
            }

            // A bone L-bracket in the corner: the active app's seal.
            Item {
                visible: chrome.item.isActive
                anchors.top: sealPlate.top
                anchors.left: sealPlate.left
                anchors.margins: 3
                width: 9
                height: 9
                Rectangle { width: parent.width; height: 2; color: chrome.band.bone }
                Rectangle { width: 2; height: parent.height; color: chrome.band.bone }
            }
        }
    }

    // ── capsule: the Material look. The plate is one rounded capsule behind the run
    // (DockBackdrop); here the focused app sits in a tonal pill, and an app that is
    // running but not focused carries a small dot on the outer edge ──────────────
    Component {
        id: capsuleC
        Item {
            id: capsule
            anchors.fill: parent
            Rectangle {
                visible: chrome.item.isActive
                anchors.fill: parent
                anchors.margins: 3
                radius: width / 2
                color: Theme.secondaryContainer
                Behavior on color { enabled: chrome.band.animate; ColorAnimation { duration: Motion.thumbHover } }
            }
            Rectangle {
                visible: chrome.item.count >= 1 && !chrome.item.isActive
                width: 4; height: 4; radius: 2
                color: Theme.onSurfaceVariant
                anchors.horizontalCenter: chrome.horizontal ? capsule.horizontalCenter : undefined
                anchors.verticalCenter: chrome.horizontal ? undefined : capsule.verticalCenter
                anchors.bottom: chrome.outerBottom ? capsule.bottom : undefined
                anchors.top: chrome.outerTop ? capsule.top : undefined
                anchors.right: chrome.outerRight ? capsule.right : undefined
                anchors.left: chrome.outerLeft ? capsule.left : undefined
                anchors.margins: 2
            }
        }
    }
}
