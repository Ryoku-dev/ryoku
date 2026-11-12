import QtQuick
import Lithic
import Ryoku.Ui.Singletons

// a task under its row: Working past 1 s, a blade with Cancel past 10 s, an error with Retry
Column {
    id: line

    required property var task
    property string target: ""
    property string doing: ""
    // shown past 10 s when the task cannot be stopped from here
    property string stopHint: ""

    readonly property bool mine: line.task.target === line.target && line.target !== ""
    readonly property bool showWorking: line.mine && line.task.running && line.task.working
    readonly property bool showError: line.mine && !line.task.running && line.task.error.length > 0

    width: parent ? parent.width : 0
    spacing: LithicTokens.space.s2
    visible: line.showWorking || line.showError

    Row {
        visible: line.showWorking
        spacing: LithicTokens.space.s2

        Working {
            anchors.verticalCenter: parent.verticalCenter
            active: line.showWorking
        }
        Label {
            anchors.verticalCenter: parent.verticalCenter
            text: line.doing
            typeRole: "detail"
        }
    }

    Row {
        visible: line.showWorking && line.task.longRunning
        width: parent.width
        spacing: LithicTokens.space.s3

        BladeProgress {
            id: blade
            anchors.verticalCenter: parent.verticalCenter
            width: stopPlaque.visible ? Math.max(80, parent.width - stopPlaque.width - parent.spacing) : parent.width
            indeterminate: true
            active: line.showWorking && line.task.longRunning
        }
        Plaque {
            id: stopPlaque
            anchors.verticalCenter: parent.verticalCenter
            visible: line.task.cancellable
            kind: "quiet"
            small: true
            glyph: "close"
            text: I18n.tr("Cancel")
            onClicked: line.task.cancel()
        }
    }

    Label {
        visible: line.showWorking && line.task.longRunning && !line.task.cancellable && line.stopHint.length > 0
        width: parent.width
        text: line.stopHint
        typeRole: "detail"
        inkRole: "fg3"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }

    Row {
        visible: line.showError
        width: parent.width
        spacing: LithicTokens.space.s3

        Label {
            anchors.verticalCenter: parent.verticalCenter
            width: Math.min(implicitWidth, parent.width - retry.width - parent.spacing)
            text: line.task.error
            typeRole: "detail"
            ink: line.task.canceled ? Lithic.fg2 : Lithic.jasper
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Plaque {
            id: retry
            anchors.verticalCenter: parent.verticalCenter
            visible: !line.task.canceled
            kind: "quiet"
            small: true
            glyph: "refresh"
            text: I18n.tr("Retry")
            onClicked: line.task.retry()
        }
    }
}
