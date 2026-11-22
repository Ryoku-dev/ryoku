import QtQuick
import Ryoku.Ui.Singletons
import ".."

CornerButton {
    id: root

    property bool spinning: false

    glyph: "refresh"
    Accessible.name: I18n.tr("Search again")

    function follow(): void {
        if (root.spinning) {
            spin.from = root.turn;
            // far more turns than a search lasts, so the turn never wraps back
            spin.to = root.turn + 360 * 100;
            spin.restart();
        } else {
            spin.stop();
            root.turn = Math.ceil(root.turn / 360) * 360;
        }
    }

    onSpinningChanged: root.follow()
    Component.onCompleted: if (root.spinning) root.follow()

    NumberAnimation {
        id: spin
        target: root
        property: "turn"
        duration: 900 * 100
    }
}
