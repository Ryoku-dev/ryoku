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
    signal resultChanged(string text)
    readonly property real s: panel.s
    readonly property real lithicScale: s
    property string form: panel.actionOptions && panel.actionOptions.form === "latex" ? "latex" : "steps"
    readonly property string answer: String(session.answers[action] || "")
    readonly property bool busy: (session.busy && session.turnMode === action) || (session.queued !== null && session.queued.mode === action)
    readonly property bool failed: session.errorText.length > 0 || ["off", "not-installed", "error"].indexOf(session.client.state) >= 0
    readonly property var planes: Lithic.planes(Lithic.design)
    readonly property color plane: Lithic.design === "lithic" ? planes.pane : Lithic.design === "fluent" ? Lithic.mix(planes.card, Lithic.sel, 0.025) : Lithic.stone4
    readonly property color stroke: Lithic.design === "material" ? "transparent" : Lithic.design === "adwaita" ? Lithic.alpha(Lithic.fg, Lithic.dark ? 0.10 : 0.16) : Lithic.design === "fluent" ? planes.cardStroke : Lithic.vein
    readonly property string latex: {
        const fenced = answer.match(/```(?:latex|tex)?[ \t]*\r?\n([\s\S]*?)(?:```|$)/i);
        return (fenced ? fenced[1] : answer).trim();
    }
    readonly property var solution: splitSteps(answer)
    readonly property string output: form === "latex" ? latex : answer
    implicitHeight: body.implicitHeight
    height: implicitHeight
    onOutputChanged: resultChanged(output)

    function splitSteps(text) {
        const result = { intro: "", steps: [], answer: "" };
        const lines = text.split("\n");
        var step = null, finalAnswer = false, fence = "";
        for (var i = 0; i < lines.length; ++i) {
            const line = lines[i];
            const marker = line.match(/^\s*(```+|~~~+)/);
            if (marker) fence = fence ? "" : marker[1][0];
            const finalLine = !fence && !marker ? line.match(/^\s*(?:\*\*|__)?Answer(?:\*\*|__)?\s*:(?:\*\*|__)?\s*(.*)$/i) : null;
            if (finalLine) {
                finalAnswer = true;
                result.answer = finalLine[1];
                continue;
            }
            if (finalAnswer) {
                result.answer += "\n" + line;
                continue;
            }
            const numbered = !fence && !marker ? line.match(/^\s*(\d+)[.)]\s+(.+)$/) : null;
            if (numbered) {
                step = { number: numbered[1], text: numbered[2] };
                result.steps.push(step);
            } else if (step) step.text += "\n" + line;
            else result.intro += (result.intro ? "\n" : "") + line;
        }
        result.intro = result.intro.trim();
        result.answer = result.answer.trim();
        return result;
    }
    function run() {
        if (session.client.state === "error") session.client.connectSocket();
        session.request("solve", "", sourceText, panel.languageName(panel.effectiveTarget), panel.languageName(panel.source), { form: form });
    }
    function retry() {
        if (session.errorCode === "no_vision") panel.settings(true);
        else if (session.errorCode === "local_only" || session.client.state === "off" || session.client.state === "not-installed") panel.settings(false);
        else run();
    }
    Component.onCompleted: run()
    Component.onDestruction: {
        if ((session.busy && session.turnMode === action) || (session.queued !== null && session.queued.mode === action)) session.cancel();
    }

    Column {
        id: body
        width: parent.width
        spacing: 13 * root.s
        Segmented {
            width: parent.width
            height: 32 * root.s
            model: [I18n.tr("Steps"), I18n.tr("LaTeX")]
            currentIndex: root.form === "latex" ? 1 : 0
            accessibleName: I18n.tr("Math format")
            enabled: !root.busy
            onActivated: index => {
                root.form = index === 1 ? "latex" : "steps";
                root.run();
            }
        }
        Column {
            width: parent.width
            visible: root.failed
            spacing: 8 * root.s
            Label {
                width: parent.width
                typeRole: "detail"; inkRole: "jasper"; wrapMode: Text.Wrap; elide: Text.ElideNone
                text: root.session.errorCode === "no_vision" ? I18n.tr("No model here can read pictures.")
                    : root.session.errorCode === "local_only" ? I18n.tr("Local models only is on, and no local model can read pictures.")
                    : root.session.client.state === "off" ? I18n.tr("Rashin is off.")
                    : root.session.client.state === "not-installed" ? I18n.tr("Rashin is not installed.")
                    : root.session.errorText || root.session.client.errorText
            }
            SightActionButton {
                s: root.s
                text: root.session.errorCode === "no_vision" ? I18n.tr("Add a provider")
                    : root.session.errorCode === "local_only" ? I18n.tr("Open Theia settings")
                    : root.session.client.state === "off" || root.session.client.state === "not-installed" ? I18n.tr("Open Rashin settings") : I18n.tr("Try again")
                onClicked: root.retry()
            }
        }
        Column {
            width: parent.width
            visible: root.form === "steps"
            spacing: 13 * root.s
            SightThread {
                width: parent.width
                s: root.s; keyTarget: root.panel
                turns: root.solution.intro ? [{ question: "", answer: root.solution.intro }] : []
            }
            Repeater {
                model: root.solution.steps
                delegate: Item {
                    id: stepRow
                    required property var modelData
                    width: parent.width
                    height: Math.max(26 * root.s, stepText.implicitHeight)
                    CutRect {
                        width: 26 * root.s; height: width
                        cut: 5
                        fill: Lithic.field
                        strokeColor: root.stroke; strokeWidth: 1
                        Label { anchors.centerIn: parent; text: stepRow.modelData.number; typeRole: "caption"; inkRole: "fg2" }
                    }
                    SightThread {
                        id: stepText
                        x: 36 * root.s
                        width: parent.width - x
                        s: root.s; keyTarget: root.panel
                        turns: [{ question: "", answer: stepRow.modelData.text.trim() }]
                    }
                }
            }
            Item {
                width: parent.width
                height: finalBody.implicitHeight + 26 * root.s
                visible: root.solution.answer.length > 0
                CutRect {
                    anchors.fill: parent
                    cut: 7
                    fill: Lithic.mix(root.plane, Lithic.sel, Lithic.design === "material" ? 0.45 : 0.22)
                    strokeColor: root.stroke; strokeWidth: 1
                }
                Column {
                    id: finalBody
                    x: 13 * root.s; y: 13 * root.s
                    width: parent.width - 26 * root.s
                    spacing: 8 * root.s
                    Label { text: I18n.tr("Answer"); typeRole: "label"; inkRole: "fg2" }
                    SightThread {
                        width: parent.width
                        s: root.s; keyTarget: root.panel
                        turns: [{ question: "", answer: root.solution.answer }]
                    }
                }
            }
        }
        Column {
            width: parent.width
            visible: root.form === "latex" && root.latex.length > 0
            spacing: 8 * root.s
            Item {
                width: parent.width
                height: mathText.implicitHeight + 20 * root.s
                CutRect { anchors.fill: parent; cut: 7; fill: Lithic.field; strokeColor: root.stroke; strokeWidth: 1 }
                TextEdit {
                    id: mathText
                    x: 13 * root.s; y: 10 * root.s
                    width: parent.width - 26 * root.s
                    text: root.latex
                    textFormat: TextEdit.PlainText
                    font.family: Lithic.font.mono; font.pixelSize: 12 * root.s
                    color: Lithic.fg
                    wrapMode: TextEdit.Wrap; readOnly: true; selectByMouse: true
                    selectionColor: Lithic.sel; selectedTextColor: Lithic.onSel
                    Keys.forwardTo: [root.panel]
                }
            }
            SightActionButton {
                s: root.s
                glyph: "content_copy"
                text: I18n.tr("Copy LaTeX")
                enabled: !root.busy
                onClicked: root.panel.copyText(root.latex)
            }
        }
        Row {
            visible: root.busy
            spacing: 8 * root.s
            Working { active: root.busy && root.visible; size: 14 * root.s; anchors.verticalCenter: parent.verticalCenter }
            Label { typeRole: "detail"; inkRole: "fg3"; text: root.form === "latex" ? I18n.tr("Reading the equation") : I18n.tr("Solving") }
            SightActionButton { s: root.s; text: I18n.tr("Stop"); onClicked: root.session.cancel() }
        }
    }
}
