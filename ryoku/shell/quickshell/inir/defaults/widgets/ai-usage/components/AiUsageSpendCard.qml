pragma ComponentBehavior: Bound

// The standalone Spending card OpenCode shows beside Now. The figures are the
// shared body, so this card is only the glass around them.
import QtQuick
import QtQuick.Effects
import Quickshell
import Quickshell.Widgets
import inir
import inir.services
import inir.modules.common
import inir.modules.common.widgets
import inir.modules.common.functions
import inir.modules.iris.style
import inir.modules.iris.components

AiUsageGlassCard {
    required property var usage
    id: card
    radius: page.radius
    // padY, not the plain card pad: the figures need more room above and below
    // than a label does, and a taller Now card beside this one must never
    // squeeze the amount towards the bottom edge.
    implicitHeight: body.implicitHeight + 2 * body.padY
    // The primary row stretches this card to its neighbours' height. Centre the
    // shared body instead of pinning it to the top, so the spare height splits
    // evenly above and below the figures.
    readonly property real spareHeight: card.height - card.implicitHeight

    AiUsageSpendBody {
        id: body
        page: card.page
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.leftMargin: page.pad
        anchors.rightMargin: page.pad
        height: implicitHeight
        y: body.padY + Math.round(Math.max(0, card.spareHeight) / 2)
    }
}
