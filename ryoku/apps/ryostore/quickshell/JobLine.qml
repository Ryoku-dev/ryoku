import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "Singletons"

// An app job under its card or row: nothing for the first second, Working
// after it, a blade with Cancel past ten seconds (or as soon as real counts
// arrive), and the end in words with Retry when it failed.
Column {
    id: line

    property string target: ""
    property string doing: ""
    property bool reducedMotion: false

    readonly property bool mine: line.target !== "" && Jobs.app.target === line.target
    readonly property bool showWorking: line.mine && Jobs.app.running && Jobs.app.working
    // a fill once anything countable is known, a sweep before that
    readonly property bool counted: Jobs.app.fraction >= 0
    // the blade waits for ten seconds, or for real counts from a progress event
    readonly property bool showBlade: line.showWorking && (Jobs.app.lengthy || Jobs.app.counting)
    readonly property bool showError: line.target !== "" && !Jobs.app.running
            && Jobs.app.errorTarget === line.target && Jobs.app.error !== ""
    readonly property bool animate: !line.reducedMotion && line.visible
            && !!line.Window.window && line.Window.window.active
    property real sweep: 0
    property real pulse: 0

    // which package, of how many, on the left; the counts on the right
    readonly property string stepText: {
        const parts = [];
        if (Jobs.app.stepName !== "")
            parts.push(Jobs.app.stepName.toUpperCase());
        if (Jobs.app.stepCount > 1 && Jobs.app.stepIndex >= 0)
            parts.push(I18n.tr("%1 OF %2").arg(Jobs.app.stepIndex + 1).arg(Jobs.app.stepCount));
        return parts.join("  ·  ");
    }
    readonly property string countText: {
        const parts = [];
        if (Jobs.app.counting)
            parts.push(String(Jobs.app.done) + " / " + String(Jobs.app.total));
        if (line.counted)
            parts.push(Math.floor(Jobs.app.fraction * 100) + "%");
        return parts.join("  ·  ");
    }

    spacing: Tokens.s2
    visible: line.showWorking || line.showError
    opacity: visible ? 1 : 0
    Behavior on opacity {
        enabled: !line.reducedMotion
        NumberAnimation { duration: Tokens.snap; easing.type: Tokens.easeSnap }
    }

    Row {
        objectName: "ryostore-job-working"
        visible: line.showWorking
        spacing: Tokens.s2

        Row {
            anchors.verticalCenter: parent.verticalCenter
            spacing: 3
            Repeater {
                model: 3
                delegate: Rectangle {
                    required property int index
                    width: 4
                    height: 4
                    color: Tokens.ink
                    // a wave across three squares; still and even with motion off
                    opacity: line.reducedMotion ? 0.7
                            : 0.25 + 0.75 * Math.max(0, Math.sin((line.pulse - index / 3) * Math.PI * 2))
                }
            }
        }
        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: line.doing !== "" ? line.doing : I18n.tr("Working")
            color: Tokens.ink
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
        }
    }

    Row {
        objectName: "ryostore-job-blade"
        visible: line.showBlade
        width: line.width
        spacing: Tokens.s3

        Rectangle {
            id: track
            anchors.verticalCenter: parent.verticalCenter
            width: cancel.visible ? Math.max(80, parent.width - cancel.width - parent.spacing) : parent.width
            height: Math.max(3, Tokens.border * 3)
            color: Tokens.lineSoft
            clip: true

            Rectangle {
                height: parent.height
                color: Tokens.bone
                width: line.counted ? track.width * Jobs.app.fraction : Math.max(Tokens.s6, track.width * 0.22)
                x: line.counted ? 0
                        : (line.reducedMotion ? (track.width - width) / 2
                                              : line.sweep * (track.width + width) - width)
                Behavior on width {
                    enabled: line.counted && !line.reducedMotion
                    NumberAnimation { duration: Tokens.move; easing.type: Tokens.ease }
                }
            }
        }

        Btn {
            id: cancel
            objectName: "ryostore-job-cancel"
            anchors.verticalCenter: parent.verticalCenter
            visible: Jobs.app.cancellable
            compact: true
            text: Jobs.app.cancelled ? I18n.tr("STOPPING") : I18n.tr("CANCEL")
            armed: !Jobs.app.cancelled
            motionEnabled: !line.reducedMotion
            onAct: Jobs.app.cancel()
            Accessible.role: Accessible.Button
            Accessible.name: I18n.tr("Cancel")
            Accessible.onPressAction: Jobs.app.cancel()
        }
    }

    Item {
        visible: line.showBlade && (line.stepText !== "" || line.countText !== "")
        width: line.width
        height: Math.max(stepLabel.implicitHeight, countLabel.implicitHeight)

        Text {
            id: stepLabel
            anchors { left: parent.left; right: countLabel.left; rightMargin: Tokens.s3 }
            text: line.stepText
            color: Tokens.inkDim
            font.family: Tokens.mono
            font.pixelSize: Tokens.fMicro
            font.letterSpacing: Tokens.trackLabel
            elide: Text.ElideRight
        }
        Text {
            id: countLabel
            anchors.right: parent.right
            text: line.countText
            color: Tokens.ink
            font.family: Tokens.mono
            font.pixelSize: Tokens.fMicro
            font.letterSpacing: Tokens.trackLabel
        }
    }

    Item {
        objectName: "ryostore-job-error"
        visible: line.showError
        width: line.width
        height: Math.max(errorText.implicitHeight, errorActions.implicitHeight)

        Text {
            id: errorText
            anchors { left: parent.left; right: errorActions.left; rightMargin: Tokens.s3; verticalCenter: parent.verticalCenter }
            text: Jobs.app.error
            color: Jobs.app.errorCancelled ? Tokens.inkDim : Tokens.alert
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            wrapMode: Text.Wrap
            maximumLineCount: 3
            elide: Text.ElideRight
            textFormat: Text.PlainText
        }

        Row {
            id: errorActions
            anchors { right: parent.right; verticalCenter: parent.verticalCenter }
            spacing: Tokens.s2

            Btn {
                objectName: "ryostore-job-retry"
                visible: !Jobs.app.errorCancelled
                compact: true
                text: I18n.tr("RETRY")
                motionEnabled: !line.reducedMotion
                onAct: Jobs.app.retry()
                Accessible.role: Accessible.Button
                Accessible.name: I18n.tr("Retry")
                Accessible.onPressAction: Jobs.app.retry()
            }
            Btn {
                compact: true
                text: I18n.tr("DISMISS")
                motionEnabled: !line.reducedMotion
                onAct: Jobs.app.dismiss()
                Accessible.role: Accessible.Button
                Accessible.name: I18n.tr("Dismiss")
                Accessible.onPressAction: Jobs.app.dismiss()
            }
        }
    }

    NumberAnimation {
        target: line
        property: "sweep"
        from: 0
        to: 1
        duration: 1100
        easing.type: Easing.InOutSine
        loops: Animation.Infinite
        running: line.showBlade && !line.counted && line.animate
    }

    NumberAnimation {
        target: line
        property: "pulse"
        from: 0
        to: 1
        duration: 900
        loops: Animation.Infinite
        running: line.showWorking && line.animate
    }
}
