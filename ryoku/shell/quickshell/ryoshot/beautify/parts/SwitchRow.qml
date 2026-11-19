import QtQuick
import Ryoku.Ui.Singletons
import "../../look"
import "../.."

// A setting that is on or off, as the design's switch row: the whole row
// toggles it. Fluent's ToggleSwitch says On or Off beside it, as WinUI does.
Field {
    id: sw

    property bool checked: false
    signal toggled(bool v)

    activatable: true
    onClicked: sw.toggled(!sw.checked)

    Row {
        spacing: Math.round(12 * sw.u)
        Text {
            visible: Look.fluent
            anchors.verticalCenter: parent.verticalCenter
            text: sw.checked ? I18n.tr("On") : I18n.tr("Off")
            color: Look.fg
            font.family: Look.sans
            font.pixelSize: Math.round(14 * sw.u)
            renderType: Text.CurveRendering
        }
        Switch {
            anchors.verticalCenter: parent.verticalCenter
            u: sw.u
            checked: sw.checked
            onToggled: (v) => sw.toggled(v)
        }
    }
}
