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
    // the life stands still while the pointer rests on the placard or its action holds focus
    readonly property bool held: placardHover.hovered || action.activeFocus
    property real _lifeLeft: LithicTokens.motion.placardLife
    property real _lifeSince: 0

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
        _lifeLeft = LithicTokens.motion.placardLife
        if (!autoDismiss)
            return
        if (Lithic.reduceMotion) {
            if (!held)
                _runLife()
        } else {
            drain.restart()
            if (held)
                drain.pause()
        }
    }
    onClosed: {
        drain.stop()
        lifeTimer.stop()
    }
    onHeldChanged: {
        if (!opened || !autoDismiss)
            return
        if (held) {
            if (drain.running)
                drain.pause()
            if (lifeTimer.running) {
                lifeTimer.stop()
                _lifeLeft -= Date.now() - _lifeSince
            }
        } else if (Lithic.reduceMotion) {
            _runLife()
        } else if (drain.paused) {
            drain.resume()
        }
    }

    // reduced motion keeps the blade whole, so a timer counts the life instead
    function _runLife() {
        _lifeSince = Date.now()
        lifeTimer.interval = Math.max(1, _lifeLeft)
        lifeTimer.restart()
    }

    background: PlacardPlane {
        transform: Translate { y: root.slideOffset }
    }

    contentItem: Item {
        id: placardContent
        transform: Translate { y: root.slideOffset }

        HoverHandler { id: placardHover }

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
        id: lifeTimer
        repeat: false
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
