import QtQuick
import Lithic
import Ryoku.Ui.Singletons

// a block travels the track on the curve, once per change or click, so the
// easing is felt rather than read off the handles; it never loops
Item {
    id: pv

    property real x0: 0.25
    property real y0: 0.1
    property real x1: 0.25
    property real y1: 1
    property int duration: 640

    readonly property real block: 44
    readonly property real travel: Math.max(0, track.width - pv.block)

    implicitHeight: 84
    activeFocusOnTab: true
    Accessible.role: Accessible.Button
    Accessible.name: I18n.tr("Play the curve")
    Keys.onSpacePressed: pv.play()
    Keys.onReturnPressed: pv.play()

    function play() {
        replay.restart();
    }

    function run() {
        anim.stop();
        mover.x = 0;
        if (Lithic.reduceMotion) {
            mover.x = pv.travel;
            return;
        }
        anim.start();
    }

    onX0Changed: pv.play()
    onY0Changed: pv.play()
    onX1Changed: pv.play()
    onY1Changed: pv.play()
    Component.onCompleted: pv.play()

    Timer {
        id: replay
        interval: 150
        onTriggered: pv.run()
    }

    CutRect {
        anchors.fill: parent
        fill: hover.hovered ? Lithic.mix(Lithic.field, Lithic.fg, LithicTokens.alpha.hoverPlane) : Lithic.field
        strokeColor: Lithic.edge
        strokeWidth: LithicTokens.size.hairline
        cut: LithicTokens.cut.mid
    }

    FocusRing {
        x: -outset
        y: -outset
        width: parent.width + outset * 2
        height: parent.height + outset * 2
        keyboardFocusVisible: pv.activeFocus
        cut: LithicTokens.cut.mid
    }

    Label {
        x: LithicTokens.space.s3
        y: LithicTokens.space.s2
        text: I18n.tr("Click to play again")
        typeRole: "caption"
        inkRole: "fg3"
    }

    Item {
        id: track
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        anchors.leftMargin: LithicTokens.space.s3
        anchors.rightMargin: LithicTokens.space.s3
        anchors.bottomMargin: LithicTokens.space.s3
        height: 28

        Rectangle {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            height: LithicTokens.size.hairline
            color: Lithic.vein
        }

        CutRect {
            x: pv.travel
            width: pv.block
            height: 24
            anchors.bottom: parent.bottom
            anchors.bottomMargin: LithicTokens.space.s1
            fill: "transparent"
            strokeColor: Lithic.vein
            strokeWidth: LithicTokens.size.hairline
            cut: LithicTokens.cut.token
        }

        CutRect {
            id: mover
            width: pv.block
            height: 24
            anchors.bottom: parent.bottom
            anchors.bottomMargin: LithicTokens.space.s1
            fill: Lithic.gold
            cut: LithicTokens.cut.token
        }

        NumberAnimation {
            id: anim
            target: mover
            property: "x"
            from: 0
            to: pv.travel
            duration: pv.duration
            easing.type: Easing.BezierSpline
            easing.bezierCurve: [pv.x0, pv.y0, pv.x1, pv.y1, 1, 1]
        }
    }

    HoverHandler {
        id: hover
        cursorShape: Qt.PointingHandCursor
    }
    TapHandler {
        onTapped: {
            pv.forceActiveFocus();
            pv.run();
        }
    }
}
