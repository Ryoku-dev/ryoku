pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import ".."
import Ryoku.Ui.Singletons
import "../look"

// Capture: the four shot targets and the screen recorder, each one hiding the
// Game Bar first so the overlay is never in its own shot. The recorder is the
// existing Recorder singleton, not a second capture path.
GameBarCard {
    id: card

    title: I18n.tr("Capture")

    Column {
        width: parent.width
        spacing: 12 * card.s

        Row {
            width: parent.width
            spacing: 14 * card.s

            Grid {
                id: targets
                columns: 2
                spacing: 8 * card.s

                GameBarAction {
                    s: card.s
                    icon: "region"
                    label: I18n.tr("Region")
                    onClicked: if (card.host) card.host.shoot("region")
                }
                GameBarAction {
                    s: card.s
                    icon: "window"
                    label: I18n.tr("Window")
                    onClicked: if (card.host) card.host.shoot("window")
                }
                GameBarAction {
                    s: card.s
                    icon: "monitor"
                    label: I18n.tr("Screen")
                    onClicked: if (card.host) card.host.shoot("monitor")
                }
                GameBarAction {
                    s: card.s
                    icon: "screens"
                    label: I18n.tr("All")
                    onClicked: if (card.host) card.host.shoot("all")
                }
            }

            Column {
                width: parent.width - targets.width - 14 * card.s
                spacing: 4 * card.s

                Text {
                    text: I18n.tr("Recording")
                    color: GameBarLook.subtext
                    font.family: GameBarLook.face
                    font.pixelSize: GameBarLook.smallie * card.s
                }

                Text {
                    text: Recorder.anyActive ? Recorder.elapsedText : I18n.tr("Idle")
                    color: Recorder.anyActive ? GameBarLook.accent : GameBarLook.fg
                    font.family: GameBarLook.face
                    font.pixelSize: GameBarLook.huge * card.s
                }

                GameBarTextButton {
                    s: card.s
                    label: Recorder.anyActive ? I18n.tr("Stop") : I18n.tr("Record")
                    primary: !Recorder.anyActive
                    danger: Recorder.anyActive
                    onClicked: {
                        if (!card.host)
                            return;
                        if (Recorder.anyActive)
                            Recorder.stop();
                        else
                            card.host.startRecording();
                    }
                }
            }
        }

        Text {
            width: parent.width
            text: I18n.tr("The bar hides while it captures.")
            color: GameBarLook.subtext
            font.family: GameBarLook.face
            font.pixelSize: GameBarLook.smallie * card.s
            wrapMode: Text.WordWrap
        }
    }
}
