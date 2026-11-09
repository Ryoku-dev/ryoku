import QtQuick
import QtQuick.Templates as T

T.Popup {
    id: root

    property string message: ""
    property string actionText: ""
    property int stackIndex: 0
    property bool autoDismiss: true
    property bool previewOpen: false
    property string previewState: ""
    property real remaining: 1
    property real slideOffset: 0

    signal actionTriggered()

    component PlacardPlane: CutRect {
        fill: Lithic.sel
        strokeColor: Lithic.vein
        strokeWidth: LithicTokens.size.hairline
        cut: LithicTokens.cut.placard
    }

    modal: false
    focus: false
    closePolicy: T.Popup.NoAutoClose
    padding: 0
    width: parent ? Math.min(parent.width, LithicTokens.size.placardWidthMax)
                  : LithicTokens.size.placardWidthMax
    height: LithicTokens.size.placardHeight
    x: parent ? (parent.width - width) / 2 : 0
    y: parent
       ? Math.max(0, parent.height - LithicTokens.size.placardBottom
                  - height - Math.min(stackIndex,
                                      LithicTokens.size.placardStackCount - 1)
                             * (height + LithicTokens.size.placardStackGap))
       : 0

    onOpened: {
        remaining = 1
        if (autoDismiss && !Lithic.reduceMotion)
            drain.restart()
    }
    onClosed: drain.stop()

    background: PlacardPlane {
        transform: Translate { y: root.slideOffset }
    }

    contentItem: Item {
        id: placardContent
        transform: Translate { y: root.slideOffset }

        Label {
            anchors.left: parent.left
            anchors.leftMargin: LithicTokens.size.placardInset
            anchors.right: action.visible ? action.left : parent.right
            anchors.rightMargin: action.visible
                                 ? LithicTokens.size.placardTrailingInset
                                 : LithicTokens.size.placardInset
            anchors.verticalCenter: parent.verticalCenter
            text: root.message
            typeRole: "body"
            ink: Lithic.onSel
            elide: Text.ElideRight
        }

        Plaque {
            id: action
            visible: root.actionText.length > 0
            anchors.right: parent.right
            anchors.rightMargin: LithicTokens.size.placardTrailingInset
            anchors.verticalCenter: parent.verticalCenter
            height: LithicTokens.size.placardActionHeight
            text: root.actionText
            kind: "on"
            small: true
            previewState: root.previewState
            onClicked: {
                root.actionTriggered()
                root.close()
            }
        }

        CutRect {
            anchors.left: parent.left
            anchors.bottom: parent.bottom
            width: parent.width * Math.max(0, Math.min(1, root.remaining))
            height: LithicTokens.size.placardBlade
            fill: Lithic.onSel
            cut: 0
        }
    }

    PlacardPlane {
        id: previewPlane
        parent: root.parent
        visible: root.previewOpen
        x: root.x
        y: root.y
        width: root.width
        height: root.height
        z: 100
    }

    StateGroup {
        states: State {
        name: "preview"
        when: root.previewOpen
        ParentChange {
            target: placardContent
            parent: previewPlane
            x: 0
            y: 0
            width: previewPlane.width
            height: previewPlane.height
        }
        PropertyChanges {
            target: placardContent
            visible: true
            enabled: true
        }
    }
        }

    NumberAnimation {
        id: drain
        target: root
        property: "remaining"
        from: 1
        to: 0
        duration: LithicTokens.motion.placardLife
        onFinished: root.close()
    }

    Timer {
        interval: LithicTokens.motion.placardLife
        repeat: false
        running: root.opened && root.autoDismiss && Lithic.reduceMotion
        onTriggered: root.close()
    }

    enter: Transition {
        ParallelAnimation {
            NumberAnimation {
                target: root
                property: "opacity"
                from: 0
                to: 1
                duration: Lithic.duration(LithicTokens.motion.placardIn)
                easing.type: Easing.BezierSpline
                easing.bezierCurve: Lithic.curve
            }
            NumberAnimation {
                target: root
                property: "slideOffset"
                from: LithicTokens.size.placardRise
                to: 0
                duration: Lithic.duration(LithicTokens.motion.placardIn)
                easing.type: Easing.BezierSpline
                easing.bezierCurve: Lithic.curve
            }
        }
    }

}
