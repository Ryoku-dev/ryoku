import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"

// a Ryoku.Ui PackageJob under its row, in Settings' own drawing: nothing for 1 s, Working with
// the tool's counts, a blade with Cancel past 10 s (or as soon as the tool counts), an error
// with Retry
Column {
    id: line

    required property var job
    property string target: ""
    property string doing: ""

    readonly property bool mine: line.job.target === line.target && line.target !== ""
    readonly property bool showWorking: line.mine && line.job.running && line.job.working
    readonly property bool showBlade: line.showWorking && (line.job.lengthy || line.job.fraction >= 0)
    readonly property bool showError: line.job.errorFor(line.target) !== "" && line.target !== ""
    readonly property string counts: line.job.counting
        ? I18n.tr("%1 of %2").arg(line.job.done).arg(line.job.total) : ""

    width: parent ? parent.width : 0
    spacing: LithicTokens.space.s2
    // fades in where it appears, and fades out before it gives back its room
    visible: opacity > 0
    opacity: line.showWorking || line.showError ? 1 : 0
    Behavior on opacity {
        NumberAnimation { duration: Tone.quick; easing.type: Easing.OutCubic }
    }

    Row {
        visible: line.showWorking
        spacing: LithicTokens.space.s2

        Working {
            anchors.verticalCenter: parent.verticalCenter
            active: line.showWorking
        }
        Label {
            anchors.verticalCenter: parent.verticalCenter
            text: line.counts !== "" ? I18n.tr("%1, %2").arg(line.doing).arg(line.counts) : line.doing
            typeRole: "detail"
        }
    }

    Row {
        visible: line.showBlade
        width: parent.width
        spacing: LithicTokens.space.s3

        BladeProgress {
            anchors.verticalCenter: parent.verticalCenter
            width: stopPlaque.visible ? Math.max(80, parent.width - stopPlaque.width - parent.spacing) : parent.width
            indeterminate: line.job.fraction < 0
            value: Math.max(0, line.job.fraction)
            active: line.showBlade
            Behavior on value {
                NumberAnimation { duration: Tone.move; easing.type: Easing.OutCubic }
            }
        }
        Plaque {
            id: stopPlaque
            anchors.verticalCenter: parent.verticalCenter
            visible: line.job.cancellable
            kind: "quiet"
            small: true
            glyph: "close"
            text: I18n.tr("Cancel")
            accessibleName: I18n.tr("Stop %1").arg(line.job.label)
            onClicked: line.job.cancel()
        }
    }

    Label {
        visible: line.showWorking && line.job.lengthy && !line.job.cancellable
        width: parent.width
        text: I18n.tr("Once your password is in, this finishes on its own; it cannot be stopped from here.")
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
            text: line.job.error
            typeRole: "detail"
            ink: line.job.errorCancelled ? Lithic.fg2 : Lithic.jasper
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Plaque {
            id: retry
            objectName: "jobRetry"
            anchors.verticalCenter: parent.verticalCenter
            visible: !line.job.errorCancelled
            kind: "quiet"
            small: true
            glyph: "refresh"
            text: I18n.tr("Retry")
            onClicked: line.job.retry()
        }
    }
}
