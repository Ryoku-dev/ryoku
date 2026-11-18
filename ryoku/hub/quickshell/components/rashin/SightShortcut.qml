import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"

Plaque {
    id: root

    required property var hub
    property var row: null
    function activate() { root.hub.navigate("shortcuts", "shell.sight"); }
    text: I18n.tr("Change shortcuts")
    kind: "quiet"
    small: true
    glyph: "arrow_forward"
    onClicked: root.activate()
}
