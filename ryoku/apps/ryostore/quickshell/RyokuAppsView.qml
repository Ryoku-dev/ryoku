import QtQuick
import QtQuick.Controls
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "Singletons"

// The Ryoku apps shelf: Ryoku's own apps, side by side, each with its version,
// size, update and the actions that apply. Installs run here, in place.
Item {
    id: view

    property bool reducedMotion: false

    readonly property int columns: width >= 900 ? 3 : (width >= 560 ? 2 : 1)
    readonly property real cardW: (scroller.width - Tokens.s4 * (view.columns - 1)) / view.columns

    Flickable {
        id: scroller
        anchors.fill: parent
        anchors.leftMargin: Tokens.s6
        anchors.rightMargin: Tokens.s6
        contentWidth: width
        contentHeight: content.implicitHeight + Tokens.s6
        boundsBehavior: Flickable.StopAtBounds
        clip: true

        Column {
            id: content
            width: scroller.width
            topPadding: Tokens.s5
            spacing: Tokens.s5

            Flow {
                objectName: "ryostore-ryoku-apps"
                width: parent.width
                spacing: Tokens.s4
                visible: RyokuApps.apps.length > 0

                Repeater {
                    model: RyokuApps.apps
                    delegate: Item {
                        id: cell
                        required property var modelData
                        required property int index
                        width: view.cardW
                        height: reveal.implicitHeight

                        // shown follows the shelf, so the stagger replays each time it opens
                        Entrance {
                            id: reveal
                            width: parent.width
                            index: cell.index
                            shown: view.visible

                            RyokuAppCard {
                                width: view.cardW
                                app: cell.modelData
                                reducedMotion: view.reducedMotion
                            }
                        }
                    }
                }
            }

            // first read, or the tool that reports them is missing
            Item {
                width: parent.width
                height: 220
                visible: RyokuApps.apps.length === 0

                Column {
                    anchors.centerIn: parent
                    spacing: Tokens.s3

                    Text {
                        anchors.horizontalCenter: parent.horizontalCenter
                        text: RyokuApps.loading ? I18n.tr("READING RYOKU'S APPS") : I18n.tr("RYOKU'S APPS UNAVAILABLE")
                        color: Tokens.ink
                        font.family: Tokens.mono
                        font.pixelSize: Tokens.fSmall
                        font.letterSpacing: Tokens.trackLabel
                    }
                    Text {
                        anchors.horizontalCenter: parent.horizontalCenter
                        width: Math.min(view.width - Tokens.s7 * 2, 440)
                        visible: !RyokuApps.loading && RyokuApps.error !== ""
                        text: RyokuApps.error
                        horizontalAlignment: Text.AlignHCenter
                        color: Tokens.inkDim
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                        wrapMode: Text.Wrap
                    }
                    Btn {
                        anchors.horizontalCenter: parent.horizontalCenter
                        visible: !RyokuApps.loading
                        text: I18n.tr("RETRY")
                        onAct: RyokuApps.refresh()
                    }
                }
            }

            Rectangle {
                width: parent.width
                height: Tokens.border
                color: Tokens.line
            }

            Row {
                width: parent.width
                spacing: Tokens.s4

                Text {
                    text: UiPreferences.text("店", "")
                    visible: text !== ""
                    color: Tokens.inkFaint
                    font.family: Tokens.jp
                    font.pixelSize: Tokens.fBody
                }
                Text {
                    width: parent.width - Tokens.s6 * 2
                    text: I18n.tr("Ryostore is built in and updates with Ryoku. Apps installed here appear in app search. Settings › Apps › Installed apps shows how much space each one takes.")
                    color: Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                    wrapMode: Text.Wrap
                }
            }
        }

        WheelScroll { }
        ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
    }
}
