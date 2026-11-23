import QtQuick
import Ryoku.Ui.Singletons
import "../kit" as K
import "../../../../modules/launcher/shared" as Shared

// Asking Rashin: the launcher's shared ask panel, hosted in the lead. The sheet
// sends Enter and Up/Down here through the lead while an ask row is selected.
// The panel opens its Rashin bridge only once a question is sent.
Column {
    id: root

    property var lead: null
    readonly property var row: root.lead ? root.lead.row : null
    readonly property string question: root.row
        ? String(root.row.question || root.row.title || "").trim() : ""
    readonly property int chipCount: panel.resumeMode ? panel.recent.length : panel.chips.length
    readonly property bool busy: panel.busy

    function step(delta) {
        panel.move(delta);
    }

    function enter() {
        if (panel.resumeMode || panel.busy
                || (panel.answerCurrent && panel.askedQuestion.trim() === root.question)) {
            panel.activate();
            return;
        }
        panel.run();
    }

    function cancel() {
        panel.cancel();
    }

    Component.onDestruction: panel.reset()

    spacing: K.Style.s3

    IconPlate {
        glyph: "auto_awesome"
        rise: root.lead ? root.lead.rise : 0
    }

    K.Label {
        width: root.width
        text: I18n.tr("Ask Rashin")
        role: "headline"
    }

    Shared.AskPanel {
        id: panel
        width: root.width
        question: root.question
        onFinished: {
            if (root.lead && root.lead.engine)
                root.lead.engine.closeRequested();
        }
    }
}
