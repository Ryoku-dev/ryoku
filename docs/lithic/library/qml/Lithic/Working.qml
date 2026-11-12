import QtQuick

Item {
    id: root

    property bool active: false
    property real size: LithicTokens.size.iconPlaqueIcon
    property real _phase: 0

    implicitWidth: size
    implicitHeight: size * LithicTokens.size.workingFlakeHeightRatio

    // the flakes exist only while the spinner shows, so an idle control carries none
    Loader {
        anchors.centerIn: parent
        active: root.visible
        sourceComponent: Component {
            Row {
                spacing: root.size * LithicTokens.size.workingFlakeGapRatio

                Repeater {
                    model: LithicTokens.size.workingFlakeCount
                    delegate: Flake {
                        required property int index
                        width: root.size * LithicTokens.size.workingFlakeWidthRatio
                        height: root.size * LithicTokens.size.workingFlakeHeightRatio
                        size: height / LithicTokens.size.flakeHeightRatio
                        tone: "becoming"
                        outline: Lithic.alpha(Lithic.vein, LithicTokens.alpha.workingBase)
                        fill: Lithic.lapis
                        fillFraction: !root.active
                            ? 0
                            : Lithic.reduceMotion
                              ? 1
                              : Math.max(0, Math.min(1, root._phase - index))
                    }
                }
            }
        }
    }

    NumberAnimation on _phase {
        from: 0
        to: LithicTokens.size.workingFlakeCount
        duration: Lithic.duration(LithicTokens.motion.working)
        easing.type: Easing.BezierSpline
        easing.bezierCurve: Lithic.curve
        loops: Animation.Infinite
        running: root.active && !Lithic.reduceMotion && root.visible
    }
}
