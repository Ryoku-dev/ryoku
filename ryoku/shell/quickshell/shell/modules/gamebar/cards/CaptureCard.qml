pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import ".."
import Ryoku.Ui.Singletons
import Lithic
import "../look"

GameBarCard {
    id: card

    title: I18n.tr("Capture")

    Column {
        width: parent.width
        spacing: card.space3

        Grid {
            id: targets
            width: parent.width
            columns: 2
            spacing: card.space2

            Plaque {
                width: (targets.width - card.space2) / 2
                small: true
                kind: "quiet"
                glyph: "crop_free"
                text: I18n.tr("Region")
                onClicked: if (card.host) card.host.shoot("region")
            }

            Plaque {
                width: (targets.width - card.space2) / 2
                small: true
                kind: "quiet"
                glyph: "select_window"
                text: I18n.tr("Window")
                onClicked: if (card.host) card.host.shoot("window")
            }

            Plaque {
                width: (targets.width - card.space2) / 2
                small: true
                kind: "quiet"
                glyph: "screenshot_monitor"
                text: I18n.tr("Screen")
                onClicked: if (card.host) card.host.shoot("monitor")
            }

            Plaque {
                width: (targets.width - card.space2) / 2
                small: true
                kind: "quiet"
                glyph: "fullscreen"
                text: I18n.tr("All")
                onClicked: if (card.host) card.host.shoot("all")
            }
        }

        Row {
            id: recording
            width: parent.width
            spacing: card.space2

            Claim {
                id: state
                tone: Recorder.anyActive || Recorder.countingDown ? "alive" : "idle"
                text: Recorder.anyActive
                    ? I18n.tr("Recording")
                    : (Recorder.countingDown ? I18n.tr("Starting") : I18n.tr("Idle"))
            }

            Label {
                id: elapsed
                text: Recorder.countingDown
                    ? I18n.tr("Starting in %1").arg(Recorder.countdownSec)
                    : (Recorder.anyActive ? Recorder.elapsedText : I18n.tr("Ready"))
                typeRole: "mono"
                inkRole: "fg2"
                numeric: true
            }

            Item {
                width: parent.width - state.width - elapsed.width - record.width
                    - 3 * recording.spacing
                height: 1
            }

            Plaque {
                id: record
                small: true
                kind: Recorder.anyActive || Recorder.countingDown ? "dangerLine" : "gold"
                glyph: Recorder.anyActive || Recorder.countingDown ? "stop" : "fiber_manual_record"
                text: Recorder.countingDown ? I18n.tr("Cancel")
                    : (Recorder.anyActive ? I18n.tr("Stop") : I18n.tr("Record"))
                onClicked: {
                    if (!card.host)
                        return;
                    if (Recorder.anyActive || Recorder.countingDown)
                        Recorder.stop();
                    else
                        card.host.startRecording();
                }
            }
        }

        Label {
            width: parent.width
            text: I18n.tr("The bar hides while it captures.")
            typeRole: "detail"
            inkRole: "fg3"
            wrapMode: Text.WordWrap
        }
    }
}
