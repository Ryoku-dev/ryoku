import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons

Item {
    id: root

    property string text: ""
    property bool primary: false
    property bool armed: true
    property bool busy: false
    property bool installed: false
    property bool reducedMotion: false
    property real progressPosition: 0.5

    signal act()

    readonly property string displayText: installed ? "\u2713  " + text : text

    implicitWidth: button.implicitWidth
    implicitHeight: button.implicitHeight
    scale: confirmation.running ? confirmationScale : 1
    property real confirmationScale: 1

    onInstalledChanged: {
        if (installed && !reducedMotion) {
            confirmation.stop();
            confirmation.restart();
        }
    }
    onBusyChanged: {
        if (!busy)
            progressPosition = 0.5;
    }
    onReducedMotionChanged: {
        if (reducedMotion) {
            progressSweep.stop();
            confirmation.stop();
            confirmationScale = 1;
            progressPosition = 0.5;
        }
    }
    Btn {
        id: button
        anchors.fill: parent
        text: root.displayText
        primary: root.primary
        armed: root.armed
        enabled: !root.busy
        motionEnabled: !root.reducedMotion
        Accessible.role: Accessible.Button
        Accessible.name: text
        Accessible.onPressAction: if (button.enabled && button.armed) root.act()
        onAct: if (!root.busy) root.act()
    }

    Item {
        anchors.fill: parent
        clip: true
        visible: root.busy

        Rectangle {
            width: Math.max(Tokens.s5, parent.width * 0.28)
            height: Tokens.border * 2
            y: parent.height - height
            x: root.reducedMotion
               ? (parent.width - width) / 2
               : root.progressPosition * (parent.width + width) - width
            color: root.primary ? Tokens.inkOnBone : Tokens.bone
        }
    }

    NumberAnimation {
        id: progressSweep
        target: root
        property: "progressPosition"
        from: 0
        to: 1
        duration: Tokens.swap
        easing.type: Tokens.ease
        loops: Animation.Infinite
        running: root.busy && !root.reducedMotion && root.visible
    }

    SequentialAnimation {
        id: confirmation
        NumberAnimation {
            target: root
            property: "confirmationScale"
            from: 1
            to: 1.035
            duration: Tokens.snap
            easing.type: Tokens.easeSnap
        }
        NumberAnimation {
            target: root
            property: "confirmationScale"
            from: 1.035
            to: 1
            duration: Tokens.snap
            easing.type: Tokens.ease
        }
    }
}
