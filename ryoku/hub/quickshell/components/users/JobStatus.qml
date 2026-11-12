import QtQuick
import Lithic
import Ryoku.Ui.Singletons

// a flow's state under its form: Working past 1 s, a progress blade with
// Cancel past 10 s, and the failure in jasper with Retry
Column {
    id: root

    property bool running: false
    property bool slow: false
    property bool lengthy: false
    property string waitingText: I18n.tr("Waiting for an administrator's password")
    property string error: ""
    property bool canRetry: true
    property bool canCancel: true
    // 0 to 1 when the job reports its steps, otherwise a sweep
    property real progress: -1

    signal retry()
    signal cancel()

    spacing: LithicTokens.space.s2
    visible: (root.running && root.slow) || root.error.length > 0

    Row {
        visible: root.running && root.slow
        width: parent.width
        spacing: LithicTokens.space.s2

        Working {
            anchors.verticalCenter: parent.verticalCenter
            active: parent.visible
        }
        Label {
            anchors.verticalCenter: parent.verticalCenter
            width: parent.width - LithicTokens.size.iconPlaqueIcon - LithicTokens.space.s2
            text: root.waitingText
            typeRole: "detail"
            inkRole: "fg2"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
    }

    Row {
        visible: root.running && root.lengthy
        width: parent.width
        spacing: LithicTokens.space.s3

        BladeProgress {
            anchors.verticalCenter: parent.verticalCenter
            width: parent.width - (stop.visible ? stop.width + LithicTokens.space.s3 : 0)
            active: parent.visible
            indeterminate: root.progress < 0
            value: Math.max(0, root.progress)
        }
        Plaque {
            id: stop
            visible: root.canCancel
            // a hidden plaque keeps keyboard focus and would still act on Return, so it is disabled with its row
            enabled: root.running && root.lengthy
            kind: "quiet"
            small: true
            text: I18n.tr("Cancel")
            onClicked: root.cancel()
        }
    }

    Row {
        visible: !root.running && root.error.length > 0
        width: parent.width
        spacing: LithicTokens.space.s3

        Label {
            anchors.verticalCenter: parent.verticalCenter
            width: parent.width - (again.visible ? again.width + LithicTokens.space.s3 : 0)
            text: root.error
            typeRole: "detail"
            ink: Lithic.jasper
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Plaque {
            id: again
            visible: root.canRetry
            enabled: !root.running && root.error.length > 0
            anchors.verticalCenter: parent.verticalCenter
            kind: "quiet"
            small: true
            glyph: "refresh"
            text: I18n.tr("Retry")
            onClicked: root.retry()
        }
    }
}
