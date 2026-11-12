import QtQuick

Plaque {
    id: root

    property bool compact: false
    property bool menuOpen: false

    width: compact ? LithicTokens.size.newPlaqueWidthCompact
                   : LithicTokens.size.newPlaqueWidth
    height: compact ? LithicTokens.size.newPlaqueHeightCompact
                    : LithicTokens.size.newPlaqueHeight
    kind: "gold"
    small: compact
    glyph: menuOpen ? "close" : "add"
    text: compact ? "" : Lithic.tr("New")
    accessibleName: menuOpen ? Lithic.tr("Close New menu") : Lithic.tr("New")

    contentItem: Column {
        spacing: root.compact ? 0 : LithicTokens.size.newPlaqueLabelGap
        transform: Translate {
            y: root._pressActive ? Lithic.pressSink : 0
            ChiselNumberBehavior on y { duration: LithicTokens.motion.press }
        }

        Glyph {
            anchors.horizontalCenter: parent.horizontalCenter
            name: root.glyph
            size: root.compact ? LithicTokens.size.newPlaqueIconCompact
                               : LithicTokens.size.newPlaqueIcon
            ink: root.ink
        }
        Label {
            visible: !root.compact
            anchors.horizontalCenter: parent.horizontalCenter
            text: root.text
            typeRole: "label"
            ink: root.ink
        }
    }
}
