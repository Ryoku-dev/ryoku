import QtQuick
import QtQuick.Templates as T

T.Control {
    id: root

    property alias model: view.model
    property alias delegate: view.delegate
    property alias currentIndex: view.currentIndex
    property alias contentY: view.contentY
    property string previewState: ""
    readonly property alias listView: view

    implicitWidth: LithicTokens.size.listMedium
    implicitHeight: LithicTokens.size.listMedium
    focusPolicy: Qt.StrongFocus
    Accessible.role: Accessible.List

    function select(index) {
        if (index >= 0 && index < view.count) {
            view.currentIndex = index;
            view.positionViewAtIndex(index, ListView.Contain);
        }
    }

    function scrollByNotch(delta) {
        var maximum = Math.max(0, view.contentHeight - view.height);
        var direction = delta > 0 ? -1 : 1;
        wheelAnimation.stop();
        wheelAnimation.from = view.contentY;
        wheelAnimation.to = Math.max(0, Math.min(maximum,
            view.contentY + direction * LithicTokens.size.rowListWheelNotch));
        wheelAnimation.start();
    }

    contentItem: ListView {
        id: view
        clip: true
        spacing: LithicTokens.size.rowListGap
        boundsBehavior: Flickable.StopAtBounds
        highlightFollowsCurrentItem: true
        highlightMoveDuration: Lithic.duration(LithicTokens.motion.select)
        highlightResizeDuration: Lithic.duration(LithicTokens.motion.select)
        highlightRangeMode: ListView.ApplyRange
        keyNavigationEnabled: true
        activeFocusOnTab: false

        highlight: CutRect {
            fill: Lithic.sel
            z: -1

            PlaceBlade {
                anchors.fill: parent
            }
        }

        Keys.onUpPressed: root.select(Math.max(0, currentIndex - 1))
        Keys.onDownPressed: root.select(Math.min(count - 1, currentIndex + 1))
        Keys.onPressed: function(event) {
            if (event.key === Qt.Key_Home) {
                root.select(0);
                event.accepted = true;
            } else if (event.key === Qt.Key_End) {
                root.select(count - 1);
                event.accepted = true;
            }
        }
    }

    WheelHandler {
        target: null
        onWheel: function(event) {
            if (event.pixelDelta.y !== 0) {
                wheelAnimation.stop();
                var maximum = Math.max(0, view.contentHeight - view.height);
                view.contentY = Math.max(0, Math.min(maximum,
                    view.contentY - event.pixelDelta.y));
            } else {
                root.scrollByNotch(event.angleDelta.y);
            }
            event.accepted = true;
        }
    }

    NumberAnimation {
        id: wheelAnimation
        target: view
        property: "contentY"
        duration: Lithic.duration(LithicTokens.motion.wheel)
        easing.type: Easing.BezierSpline
        easing.bezierCurve: Lithic.curve
    }

    CutRect {
        visible: view.contentHeight > view.height
        anchors.right: parent.right
        width: LithicTokens.size.scrollThumb
        height: Math.max(LithicTokens.size.scrollThumbMin,
                         view.height * view.height / view.contentHeight)
        y: view.contentHeight <= view.height
            ? 0
            : view.contentY * (view.height - height)
              / (view.contentHeight - view.height)
        fill: Lithic.vein
        cutTopLeft: LithicTokens.cut.token
        cutBottomLeft: LithicTokens.cut.token
    }

    FocusRing {
        x: -outset
        y: -outset
        width: root.width + outset * 2
        height: root.height + outset * 2
        keyboardFocusVisible: root.visualFocus || root.previewState === "focus"
        cut: 0
    }
}
