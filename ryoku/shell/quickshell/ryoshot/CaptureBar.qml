pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons
import "look"
import "designs"

// The capture bar in the desktop's design (theme.design), switched live. It
// fills its output's window; each design places itself where its platform puts
// it and owns its motion. One output holds it at a time (the shell says which
// through active), and it steps aside while a region is dragged or a colour
// picked, taking its input with it.
Item {
    id: bar

    property real u: 1
    property bool active: false
    // what a top bar's exclusive zone holds back on this output
    property real topInset: 0
    property bool aside: false
    readonly property bool shown: bar.active && !bar.aside

    property var actions: []
    property string activeAction: "shot"
    // region, window or monitor
    property string target: "region"
    property bool windowTarget: false
    property bool audioOn: false
    property bool micOn: false
    property bool keypressesOn: false
    property bool webcamOn: false
    property int delaySeconds: 0
    readonly property var delays: [0, 1, 3, 5, 10]
    // the menu the bar holds open: "", "more" or "delay"
    property string menu: ""

    readonly property bool recording: bar.activeAction === "record"
    // how far the bar is still on screen, and where its plane stands
    readonly property real reveal: bar.item ? bar.item.reveal : 0
    readonly property rect plane: bar.item ? bar.item.plane : Qt.rect(0, 0, 0, 0)
    readonly property Item item: loader.item

    signal actionPicked(string id)
    signal targetPicked(string target)
    signal delayPicked(int seconds)
    signal delayCycled()
    signal colorPickRequested()
    signal screenRequested()
    signal audioToggled()
    signal micToggled()
    signal keypressesToggled()
    signal webcamToggled()
    signal folderRequested()
    signal settingsRequested()
    signal menuRequested(string name)
    signal closeRequested()

    function delayLabel(seconds) {
        return seconds === 0 ? I18n.tr("No delay") : I18n.tr("%1 s").arg(seconds);
    }

    visible: bar.reveal > 0

    Loader {
        id: loader
        anchors.fill: parent
        sourceComponent: Look.material ? material
            : Look.adwaita ? adwaita
            : Look.fluent ? fluent
            : lithic
    }

    Component {
        id: lithic
        LithicCaptureBar { bar: bar }
    }
    Component {
        id: material
        MaterialCaptureBar { bar: bar }
    }
    Component {
        id: adwaita
        AdwaitaCaptureBar { bar: bar }
    }
    Component {
        id: fluent
        FluentCaptureBar { bar: bar }
    }
}
