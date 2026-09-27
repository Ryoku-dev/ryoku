import QtQuick
import Ryoku.Ui.Singletons

// One product tile in the browse grid: the shared cover art inset as a card,
// with the cover's own accent selection frame and a keyboard-focus ring. The
// grid sizes it; hover and tap are surfaced to the grid.
Item {
    id: card

    required property var item
    property bool selected: false
    property bool focusVisible: false
    property bool reducedMotion: false

    signal hoverChanged(bool hovered)
    signal activated()

    // Immediate local feedback on hover (rise to full size), independent of the
    // hero's dwell delay, so the tile under the pointer always responds at once.
    scale: card.selected ? 1 : (hover.hovered ? 1 : 0.975)
    Behavior on scale {
        enabled: !card.reducedMotion
        NumberAnimation { duration: Tokens.snap; easing.type: Tokens.easeSnap }
    }

    ProductCover {
        id: cover
        anchors.fill: parent
        anchors.margins: Tokens.s2
        item: card.item
        selected: card.selected
        active: true
    }

    // keyboard focus ring, distinct from the accent selection frame the cover
    // already draws, so a highlighted-but-unfocused tile still reads as selected
    Rectangle {
        anchors.fill: cover
        color: "transparent"
        border.width: Tokens.border * 2
        border.color: Tokens.bone
        visible: card.focusVisible
    }

    HoverHandler {
        id: hover
        cursorShape: Qt.PointingHandCursor
        onHoveredChanged: card.hoverChanged(hover.hovered)
    }

    TapHandler { onTapped: card.activated() }
}
