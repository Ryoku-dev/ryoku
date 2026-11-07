pragma ComponentBehavior: Bound

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
    // One gap for the block: between the hairline and the Spending title, and
    // the least air a session keeps above the hairline.
    readonly property int spendGap: Math.round(10 * IrisStyle.density)
    // What the Spending block owns at the bottom of the card: the hairline, a
    // clear gap under it, the figures, and padY of card below the amount.
    readonly property int spendBlockHeight: page.integratedSpend
        ? 1 + card.spendGap + spendBody.implicitHeight + spendBody.padY : 0
    // Content-sized, two rules decide the height. The sessions keep at least
    // spendGap above and below, and the block takes the bottom half so the
    // hairline lands on the card's middle when there is one session (2 * the
    // block is exactly two equal halves). A second session grows the sessions
    // half past that, the block stays put at the bottom and the hairline drops
    // with it. Both rules come from one number each, so nothing is pinned to an
    // edge and no branch can put the sessions over the hairline.
    implicitHeight: page.integratedSpend
        ? Math.max(2 * card.spendBlockHeight,
                   body.implicitHeight + 2 * card.spendGap + card.spendBlockHeight)
        : body.implicitHeight + 2 * page.pad
    // The primary row sizes every card to the tallest one, so this body can
    // have spare height. Centre it in the room it owns, which ends at the
    // hairline when Spending shares the card. The block keeps the bottom edge
    // and the spare splits above the sessions and above the hairline, so the
    // content never drifts towards either edge.
    readonly property int bodyTop: Math.round(Math.max(0,
        card.height - body.implicitHeight
        - (page.integratedSpend ? card.spendBlockHeight : 0)) / 2)

    Column {
        id: body
        x: page.pad
        y: card.bodyTop
        width: card.width - 2 * page.pad
        spacing: page.step * 2

        Item {
            width: parent.width
            height: Math.max(agentLine.implicitHeight, agoText.implicitHeight)
            Row {
                id: agentLine
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                spacing: Math.round(5 * IrisStyle.density)
                Rectangle {
                    id: liveDot
                    anchors.verticalCenter: parent.verticalCenter
                    width: Math.round(5 * IrisStyle.density)
                    height: width
                    radius: width / 2
                    readonly property bool live: usage.runningSessions.length > 0
                    // A live run breathes; an idle one sits still.
                    property real pulse: 1
                    color: liveDot.live ? IrisStyle.success
                        : ColorUtils.applyAlpha(IrisStyle.accent, 0.45)
                    opacity: liveDot.live ? liveDot.pulse : 1
                    // The pulse runs on its own clock. It used to be
                    // `running: liveDot.live`, and every cache refresh emptied
                    // the session list for a frame, which restarted the
                    // animation and made a live dot look faster than its
                    // 1.6 s cycle. A constant loop keeps the speed honest.
                    SequentialAnimation {
                        running: true
                        loops: Animation.Infinite
                        NumberAnimation {
                            target: liveDot
                            property: "pulse"
                            from: 0.35
                            to: 1
                            duration: 800
                            easing.type: Easing.InOutQuad
                        }
                        NumberAnimation {
                            target: liveDot
                            property: "pulse"
                            from: 1
                            to: 0.35
                            duration: 800
                            easing.type: Easing.InOutQuad
                        }
                    }
                }
                IrisText {
                    anchors.verticalCenter: parent.verticalCenter
                    text: "Sessions"
                    font.pixelSize: Math.round(12 * IrisStyle.density)
                    font.weight: IrisStyle.weight(Font.DemiBold)
                }
            }
            IrisText {
                id: agoText
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                role: IrisText.Meta
                color: IrisStyle.muted
                font.pixelSize: Math.round(10 * IrisStyle.density)
                text: usage.relative(usage.lastActivityMs)
            }
        }

        // At most two live sessions; when none runs, say so.
        Column {
            width: parent.width
            spacing: Math.round(2 * IrisStyle.density)
            Repeater {
                model: usage.runningSessions.slice(0, 2)
                Column {
                    required property var modelData
                    width: parent.width
                    spacing: 0
                    IrisText {
                        width: parent.width
                        text: modelData?.title ?? ""
                        font.pixelSize: Math.round(11 * IrisStyle.density)
                        elide: Text.ElideRight
                        maximumLineCount: 1
                    }
                    IrisText {
                        width: parent.width
                        role: IrisText.Meta
                        color: IrisStyle.muted
                        font.pixelSize: Math.round(9 * IrisStyle.density)
                        elide: Text.ElideRight
                        maximumLineCount: 1
                        text: page.elapsedLabel(modelData?.started ?? 0) + " · "
                            + usage.kmg(Number(modelData?.tokens ?? 0)) + " tokens"
                    }
                }
            }
            IrisText {
                width: parent.width
                visible: usage.runningSessions.length === 0
                role: IrisText.Meta
                color: IrisStyle.muted
                font.pixelSize: Math.round(10 * IrisStyle.density)
                text: "no session running"
            }
        }
    }

    // Spending uses the empty room under the sessions instead of taking a row
    // of its own, and it keeps the Spending card's own layout so the number
    // reads the same on both agents. It sits on the card's bottom edge, but its
    // own pad keeps the amount off that edge.
    Item {
        id: spendBlock
        visible: page.integratedSpend
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        anchors.leftMargin: page.pad
        anchors.rightMargin: page.pad
        height: card.spendBlockHeight

        Rectangle {
            id: spendHairline
            width: parent.width
            height: 1
            color: ColorUtils.applyAlpha(IrisStyle.muted, 0.16)
        }

        AiUsageSpendBody {
            id: spendBody
            page: card.page
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: spendHairline.bottom
            anchors.bottom: parent.bottom
            anchors.topMargin: card.spendGap
            anchors.bottomMargin: spendBody.padY
        }
    }
}
