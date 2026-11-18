pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons

Item {
    id: root
    required property var session
    required property var card
    required property var catalog
    required property var panel
    required property string action
    property string sourceText: ""
    readonly property real s: panel.s
    readonly property real lithicScale: s
    readonly property var planes: Lithic.planes(Lithic.design)
    readonly property color plane: Lithic.design === "lithic" ? planes.pane : Lithic.design === "fluent" ? Lithic.mix(planes.card, Lithic.sel, 0.025) : Lithic.stone4
    readonly property color stroke: Lithic.design === "material" ? "transparent" : Lithic.design === "adwaita" ? Lithic.alpha(Lithic.fg, Lithic.dark ? 0.10 : 0.16) : Lithic.design === "fluent" ? planes.cardStroke : Lithic.vein
    property var entry: null
    property bool pending: false
    property bool ready: false
    property string problem: ""
    signal resultChanged(string text)

    implicitHeight: layout.implicitHeight
    height: implicitHeight

    function start() {
        if (!ready) return;
        stop();
        entry = null;
        problem = "";
        resultChanged("");
        pending = true;
        session.request("define", "", sourceText, panel.languageName(panel.effectiveTarget), panel.languageName(panel.source), {});
        Qt.callLater(settle);
    }
    function stop() {
        pending = false;
        if ((session.busy && session.turnMode === "define") || (session.queued && session.queued.mode === "define"))
            session.cancel();
    }
    function valid(value) {
        if (!value || typeof value.term !== "string" || !value.term.trim() || typeof value.reading !== "string"
            || typeof value.note !== "string" || typeof value.translation !== "string"
            || !Array.isArray(value.senses) || value.senses.length === 0) return false;
        for (var i = 0; i < value.senses.length; ++i) {
            const sense = value.senses[i];
            if (!sense || typeof sense.part !== "string" || !sense.part.trim() || typeof sense.meaning !== "string"
                || !sense.meaning.trim() || typeof sense.example !== "string" || typeof sense.here !== "boolean") return false;
        }
        return true;
    }
    function settle() {
        if (!pending || session.busy || session.queued) return;
        if (session.errorText) {
            pending = false;
            problem = session.errorText;
            return;
        }
        if (session.turnMode !== "define") return;
        pending = false;
        const value = session.parseJson(String(session.answers.define || ""));
        if (!valid(value)) {
            problem = I18n.tr("The definition did not come through fully. Try again.");
            return;
        }
        entry = value;
        resultChanged(plainText());
    }
    function plainText() {
        if (!entry) return "";
        const lines = [entry.term];
        if (entry.reading.trim()) lines.push(entry.reading);
        for (var i = 0; i < entry.senses.length; ++i) {
            const sense = entry.senses[i];
            lines.push("", String(i + 1) + ". " + sense.part + (sense.here ? " (" + I18n.tr("Used here") + ")" : ""), sense.meaning);
            if (sense.example.trim()) lines.push(sense.example);
        }
        if (entry.note.trim()) lines.push("", entry.note);
        if (entry.translation.trim()) lines.push("", I18n.tr("Translation: %1").arg(entry.translation));
        return lines.join("\n");
    }
    onSourceTextChanged: if (ready && sourceText.trim()) start()
    Component.onCompleted: { ready = true; start(); }
    Component.onDestruction: stop()
    Connections {
        target: root.session
        function onBusyChanged() { Qt.callLater(root.settle); }
        function onQueuedChanged() { Qt.callLater(root.settle); }
        function onErrorTextChanged() { Qt.callLater(root.settle); }
    }

    Column {
        id: layout
        width: root.width
        spacing: 13 * root.s
        Row {
            visible: root.pending
            spacing: 8 * root.s
            Working { active: root.pending && root.visible; size: 14 * root.s; anchors.verticalCenter: parent.verticalCenter }
            Label { text: I18n.tr("Finding the meaning"); typeRole: "detail"; inkRole: "fg3" }
        }
        Column {
            width: parent.width
            visible: root.problem.length > 0
            spacing: 8 * root.s
            Label { width: parent.width; text: root.problem; typeRole: "detail"; inkRole: "fg2"; wrapMode: Text.Wrap; elide: Text.ElideNone }
            SightActionButton { s: root.s; text: I18n.tr("Retry"); glyph: "refresh"; onClicked: root.start() }
        }
        Column {
            width: parent.width
            visible: root.entry !== null
            spacing: 13 * root.s
            Column {
                width: parent.width
                spacing: 4 * root.s
                Label { width: parent.width; text: root.entry ? root.entry.term : ""; typeRole: "title"; wrapMode: Text.Wrap; elide: Text.ElideNone }
                Label { width: parent.width; visible: text.trim().length > 0; text: root.entry ? root.entry.reading : ""; typeRole: "mono"; inkRole: "fg2"; wrapMode: Text.Wrap; elide: Text.ElideNone }
            }
            Repeater {
                model: root.entry ? root.entry.senses : []
                delegate: Item {
                    id: senseCard
                    required property var modelData
                    required property int index
                    width: root.width
                    height: senseBody.implicitHeight + 26 * root.s
                    CutRect {
                        anchors.fill: parent; cut: 7
                        fill: senseCard.modelData.here ? Lithic.mix(root.plane, Lithic.sel, 0.07) : root.plane
                        strokeColor: root.stroke; strokeWidth: Lithic.design === "material" ? 0 : 1
                    }
                    Column {
                        id: senseBody
                        x: 13 * root.s; y: 13 * root.s; width: parent.width - 26 * root.s
                        spacing: 8 * root.s
                        Flow {
                            width: parent.width
                            spacing: 8 * root.s
                            Item {
                                width: Math.min(part.implicitWidth + 14 * root.s, senseBody.width)
                                height: part.implicitHeight + 6 * root.s
                                CutRect { anchors.fill: parent; cut: 4; fill: Lithic.field }
                                Label { id: part; x: 7 * root.s; y: 3 * root.s; width: parent.width - 14 * root.s; text: senseCard.modelData.part; typeRole: "caption"; inkRole: "fg2" }
                            }
                            Row {
                                visible: senseCard.modelData.here
                                spacing: 6 * root.s
                                height: part.implicitHeight + 6 * root.s
                                CutRect { width: 6 * root.s; height: width; cut: 3; fill: Lithic.gold; anchors.verticalCenter: parent.verticalCenter }
                                Label { text: I18n.tr("Used here"); typeRole: "caption"; inkRole: "fg2"; anchors.verticalCenter: parent.verticalCenter }
                            }
                        }
                        Label { width: parent.width; text: senseCard.modelData.meaning; typeRole: "body"; wrapMode: Text.Wrap; elide: Text.ElideNone }
                        Label { width: parent.width; visible: text.trim().length > 0; text: senseCard.modelData.example; typeRole: "detail"; inkRole: "fg2"; font.italic: true; wrapMode: Text.Wrap; elide: Text.ElideNone }
                    }
                }
            }
            Label { width: parent.width; visible: text.trim().length > 0; text: root.entry ? root.entry.note : ""; typeRole: "detail"; inkRole: "fg2"; wrapMode: Text.Wrap; elide: Text.ElideNone }
            Item {
                width: parent.width
                visible: root.entry !== null && root.entry.translation.trim().length > 0
                height: translation.implicitHeight + 26 * root.s
                CutRect { anchors.fill: parent; cut: 7; fill: Lithic.field }
                Column {
                    id: translation
                    x: 13 * root.s; y: 13 * root.s; width: parent.width - 26 * root.s; spacing: 4 * root.s
                    Label { text: I18n.tr("Translation"); typeRole: "caption"; inkRole: "fg3" }
                    Label { width: parent.width; text: root.entry ? root.entry.translation : ""; typeRole: "body"; wrapMode: Text.Wrap; elide: Text.ElideNone }
                }
            }
        }
    }
}
