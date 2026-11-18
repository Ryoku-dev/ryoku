pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Shapes
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
    signal resultChanged(string text)
    readonly property real s: panel.s || 1
    readonly property var planes: Lithic.planes(Lithic.design)
    readonly property color plane: Lithic.design === "lithic" ? planes.pane : Lithic.design === "fluent" ? Lithic.mix(planes.card, Lithic.sel, 0.025) : Lithic.stone4
    readonly property color stroke: Lithic.design === "material" ? "transparent" : Lithic.design === "adwaita" ? Lithic.alpha(Lithic.fg, Lithic.dark ? 0.10 : 0.16) : Lithic.design === "fluent" ? planes.cardStroke : Lithic.vein
    property var report: null
    property var opinion: null
    property bool checking: false
    property bool modelRequested: false
    property bool modelFinished: false
    property bool started: false
    property string errorText: ""
    property var lifetime: ({ closed: false, serial: 0 })
    property real shownScore: 0
    readonly property bool modelBusy: modelRequested && (session.busy || session.queued !== null)
    readonly property int score: report ? Math.max(0, Math.min(100, Number(report.score) || 0)) : 0
    readonly property string highlighted: highlightedText()
    implicitHeight: body.implicitHeight
    Behavior on shownScore { NumberAnimation { duration: Lithic.duration(600); easing.type: Easing.OutCubic } }

    function verdict(value) {
        return value === "human" ? I18n.tr("Reads human") : value === "machine" ? I18n.tr("Reads machine-written") : I18n.tr("Mixed");
    }
    function escapeHtml(text) { return String(text).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/\n/g, "<br>"); }
    function highlightedText() {
        const phrases = report && report.phrases || [];
        const ordered = phrases.slice().sort(function(a, b) { return a.start - b.start; });
        let html = "";
        let cursor = 0;
        for (let i = 0; i < ordered.length; ++i) {
            const phrase = ordered[i];
            if (phrase.start < cursor || phrase.end <= phrase.start || phrase.end > sourceText.length) continue;
            html += escapeHtml(sourceText.slice(cursor, phrase.start));
            html += '<font color="' + Lithic.sel.toString() + '"><u>' + escapeHtml(sourceText.slice(phrase.start, phrase.end)) + '</u></font>';
            cursor = phrase.end;
        }
        return html + escapeHtml(sourceText.slice(cursor));
    }
    function publish() {
        if (!report) { panel.resultText = ""; resultChanged(""); return; }
        let text = I18n.tr("%1 / 100 · %2").arg(score).arg(verdict(report.verdict)) + "\n" + I18n.tr("A rough guide, not proof.");
        const signals = report.signals || [];
        for (let i = 0; i < signals.length; ++i) text += "\n" + signals[i].label + (signals[i].detail ? ": " + signals[i].detail : "");
        if (opinion) text += "\n\n" + I18n.tr("Model's view") + "\n" + I18n.tr("%1 / 100 · %2").arg(Math.max(0, Math.min(100, Number(opinion.score) || 0))).arg(verdict(opinion.verdict)) + (opinion.note ? "\n" + opinion.note : "");
        panel.resultText = text;
        resultChanged(text);
    }
    function run() {
        if (!started) return;
        session.cancelWriting();
        if (modelRequested) session.cancel();
        const token = lifetime;
        const serial = ++token.serial;
        report = null;
        opinion = null;
        modelRequested = false;
        modelFinished = false;
        errorText = "";
        shownScore = 0;
        publish();
        if (!sourceText.trim()) { checking = false; return; }
        checking = true;
        session.check(sourceText, ["aitell"], function(result, error) {
            if (token.closed || serial !== token.serial) return;
            root.checking = false;
            if (error) { root.errorText = String(error.message || I18n.tr("The text could not be checked.")); return; }
            if (!result || !result.aitell) { root.errorText = I18n.tr("The check did not return a score."); return; }
            root.report = result.aitell;
            root.shownScore = root.score;
            root.publish();
        });
    }
    function askModel() {
        errorText = "";
        opinion = null;
        modelFinished = false;
        modelRequested = true;
        session.request("aicheck", "", sourceText, "", "", {});
    }
    function finishModel() {
        if (!modelRequested || modelFinished || session.busy || session.queued !== null) return;
        modelFinished = true;
        if (session.errorText) { errorText = session.errorText; return; }
        const answer = session.parseJson(String(session.answers.aicheck || ""));
        if (!answer || typeof answer.score !== "number" || ["human", "mixed", "machine"].indexOf(answer.verdict) < 0) {
            errorText = I18n.tr("The model did not return a readable check.");
            return;
        }
        opinion = answer;
        publish();
    }
    Connections {
        target: root.session
        function onBusyChanged() { Qt.callLater(root.finishModel); }
        function onQueuedChanged() { Qt.callLater(root.finishModel); }
        function onAnswersChanged() { Qt.callLater(root.finishModel); }
        function onErrorTextChanged() { if (root.modelRequested && root.session.errorText) root.errorText = root.session.errorText; }
    }
    onSourceTextChanged: if (started) run()
    onActionChanged: if (started) run()
    Component.onCompleted: { started = true; run(); }
    Component.onDestruction: { lifetime.closed = true; ++lifetime.serial; session.cancel(); session.cancelWriting(); }

    Column {
        id: body
        width: parent.width; spacing: 13 * root.s
        Row {
            spacing: 8 * root.s; visible: root.checking || root.modelBusy
            Working { active: (root.checking || root.modelBusy) && root.visible; size: 14 * root.s; anchors.verticalCenter: parent.verticalCenter }
            Label { typeRole: "detail"; inkRole: "fg3"; text: root.modelBusy ? I18n.tr("Asking your model") : I18n.tr("Checking writing") }
        }
        Column {
            width: parent.width; spacing: 8 * root.s; visible: root.errorText.length > 0
            Label { width: parent.width; typeRole: "detail"; inkRole: "jasper"; wrapMode: Text.Wrap; elide: Text.ElideNone; text: root.errorText }
            TheiaChip { text: I18n.tr("Try again"); icon: "refresh"; onClicked: { if (root.modelRequested) root.askModel(); else root.run(); } }
        }
        Column {
            width: parent.width; spacing: 10 * root.s; visible: !!root.report
            Item {
                width: parent.width; height: 155 * root.s
                Item {
                    id: gauge
                    width: 154 * root.s; height: 154 * root.s; anchors.horizontalCenter: parent.horizontalCenter
                    Shape {
                        anchors.fill: parent
                        ShapePath {
                            fillColor: "transparent"; strokeColor: root.stroke === "transparent" ? Lithic.alpha(Lithic.fg, 0.12) : root.stroke; strokeWidth: 7 * root.s; capStyle: ShapePath.RoundCap
                            PathAngleArc { centerX: gauge.width / 2; centerY: gauge.height / 2; radiusX: 64 * root.s; radiusY: 64 * root.s; startAngle: 140; sweepAngle: 220 }
                        }
                        ShapePath {
                            fillColor: "transparent"; strokeColor: Lithic.sel; strokeWidth: 7 * root.s; capStyle: ShapePath.RoundCap
                            PathAngleArc { centerX: gauge.width / 2; centerY: gauge.height / 2; radiusX: 64 * root.s; radiusY: 64 * root.s; startAngle: 140; sweepAngle: 220 * root.shownScore / 100 }
                        }
                    }
                    Column {
                        anchors.centerIn: parent; spacing: 2 * root.s
                        Text { anchors.horizontalCenter: parent.horizontalCenter; text: Math.round(root.shownScore); color: Lithic.fg; font.family: Lithic.font.mono; font.pixelSize: 34 * root.s }
                        Label { anchors.horizontalCenter: parent.horizontalCenter; text: I18n.tr("out of 100"); typeRole: "detail"; inkRole: "fg3" }
                    }
                }
            }
            Label { width: parent.width; horizontalAlignment: Text.AlignHCenter; typeRole: "label"; text: root.verdict(root.report ? root.report.verdict : "mixed") }
            Label { width: parent.width; horizontalAlignment: Text.AlignHCenter; typeRole: "detail"; inkRole: "fg3"; wrapMode: Text.Wrap; elide: Text.ElideNone; text: I18n.tr("A rough guide, not proof.") }
            Repeater {
                model: root.report && root.report.signals || []
                delegate: Column {
                    id: signalRow
                    required property var modelData
                    width: body.width; spacing: 4 * root.s
                    Label { width: parent.width; typeRole: "detail"; wrapMode: Text.Wrap; elide: Text.ElideNone; text: signalRow.modelData.label }
                    CutRect {
                        width: parent.width; height: 3 * root.s; cut: 0; fill: Lithic.alpha(Lithic.fg, 0.1)
                        CutRect { width: parent.width * Math.max(0, Math.min(1, Number(signalRow.modelData.value) || 0)); height: parent.height; cut: 0; fill: Lithic.sel }
                    }
                    Label { width: parent.width; visible: text.length > 0; typeRole: "detail"; inkRole: "fg3"; wrapMode: Text.Wrap; elide: Text.ElideNone; text: signalRow.modelData.detail || "" }
                }
            }
            CutRect {
                width: parent.width; implicitHeight: source.contentHeight + 26 * root.s; height: implicitHeight
                cut: 6; fill: root.plane; strokeColor: root.stroke; strokeWidth: 1
                TextEdit {
                    id: source
                    x: 13 * root.s; y: 13 * root.s; width: parent.width - 26 * root.s; height: contentHeight
                    text: root.highlighted; textFormat: TextEdit.RichText; readOnly: true; selectByMouse: true; wrapMode: TextEdit.Wrap
                    font.family: Lithic.type.body.family; font.pixelSize: 14 * root.s; color: Lithic.fg
                    selectionColor: Lithic.sel; selectedTextColor: Lithic.onSel
                }
            }
            Repeater {
                model: root.report && root.report.phrases || []
                delegate: Column {
                    required property var modelData
                    width: body.width; spacing: 3 * root.s
                    Label { width: parent.width; typeRole: "detail"; inkRole: "fg2"; wrapMode: Text.Wrap; elide: Text.ElideNone; text: root.sourceText.slice(parent.modelData.start, parent.modelData.end) }
                    Label { width: parent.width; typeRole: "detail"; inkRole: "fg3"; wrapMode: Text.Wrap; elide: Text.ElideNone; text: parent.modelData.message }
                }
            }
            Flow {
                width: parent.width; spacing: 8 * root.s
                TheiaChip { visible: root.catalog.feature("humanize"); text: I18n.tr("Sound like you"); icon: "person_edit"; enabled: !root.modelBusy; onClicked: root.panel.runAction("humanize", {}) }
                TheiaChip { text: I18n.tr("Ask the model too"); icon: "chat_bubble"; enabled: !root.modelBusy; onClicked: root.askModel() }
            }
            CutRect {
                width: parent.width; visible: !!root.opinion; implicitHeight: opinionBody.implicitHeight + 26 * root.s; height: implicitHeight
                cut: 6; fill: root.plane; strokeColor: root.stroke; strokeWidth: 1
                Column {
                    id: opinionBody
                    x: 13 * root.s; y: 13 * root.s; width: parent.width - 26 * root.s; spacing: 8 * root.s
                    Label { width: parent.width; typeRole: "label"; text: I18n.tr("Model's view") }
                    Label { width: parent.width; typeRole: "detail"; wrapMode: Text.Wrap; elide: Text.ElideNone; text: root.opinion ? I18n.tr("%1 / 100 · %2").arg(Math.max(0, Math.min(100, Number(root.opinion.score) || 0))).arg(root.verdict(root.opinion.verdict)) : "" }
                    Label { width: parent.width; typeRole: "detail"; inkRole: "fg2"; wrapMode: Text.Wrap; elide: Text.ElideNone; text: root.opinion && root.opinion.note || "" }
                    Repeater {
                        model: root.opinion && root.opinion.phrases || []
                        delegate: Label {
                            required property var modelData
                            width: opinionBody.width; typeRole: "detail"; inkRole: "fg2"; wrapMode: Text.Wrap; elide: Text.ElideNone
                            text: I18n.tr("%1: %2").arg(modelData.text || "").arg(modelData.why || "")
                        }
                    }
                }
            }
        }
    }
}
