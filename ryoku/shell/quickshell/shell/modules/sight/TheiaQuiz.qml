pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Templates as T
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
    property var questions: []
    property var picks: []
    property int current: 0
    property int requestedCount: 5
    property bool pending: false
    property bool ready: false
    property string problem: ""
    readonly property bool finished: questions.length > 0 && current >= questions.length
    readonly property var question: current < questions.length ? questions[current] : null
    readonly property int pick: current < picks.length ? picks[current] : -1
    readonly property int score: picks.reduce(function(total, value, index) { return total + (root.questions[index] && value === root.questions[index].answer ? 1 : 0); }, 0)
    signal resultChanged(string text)

    implicitHeight: layout.implicitHeight
    height: implicitHeight

    function start() {
        if (!ready) return;
        stop();
        questions = [];
        picks = [];
        current = 0;
        problem = "";
        resultChanged("");
        const count = Number(catalog.option("learn.count", 5));
        requestedCount = count === 3 || count === 8 ? count : 5;
        pending = true;
        session.request("learn", "", sourceText, panel.languageName(panel.effectiveTarget), panel.languageName(panel.source), { count: String(requestedCount) });
        Qt.callLater(settle);
    }
    function stop() {
        pending = false;
        if ((session.busy && session.turnMode === "learn") || (session.queued && session.queued.mode === "learn"))
            session.cancel();
    }
    function settle() {
        if (!pending || session.busy || session.queued) return;
        if (session.errorText) {
            pending = false;
            problem = session.errorText;
            return;
        }
        if (session.turnMode !== "learn") return;
        pending = false;
        const value = session.parseJson(String(session.answers.learn || ""));
        if (!valid(value)) {
            problem = I18n.tr("The questions did not come through fully. Try again.");
            return;
        }
        questions = value.questions;
        resultChanged(plainText());
    }
    function valid(value) {
        if (!value || !Array.isArray(value.questions) || value.questions.length !== requestedCount) return false;
        for (var i = 0; i < value.questions.length; ++i) {
            const q = value.questions[i];
            if (!q || typeof q.question !== "string" || !q.question.trim() || typeof q.why !== "string" || !q.why.trim()
                || !Array.isArray(q.choices) || q.choices.length !== 4 || typeof q.answer !== "number"
                || Math.floor(q.answer) !== q.answer || q.answer < 0 || q.answer > 3) return false;
            for (var j = 0; j < 4; ++j)
                if (typeof q.choices[j] !== "string" || !q.choices[j].trim()) return false;
        }
        return true;
    }
    function plainText() {
        const lines = [];
        for (var i = 0; i < questions.length; ++i) {
            const q = questions[i];
            lines.push(String(i + 1) + ". " + q.question);
            for (var j = 0; j < q.choices.length; ++j) lines.push(String(j + 1) + ". " + q.choices[j]);
            lines.push(I18n.tr("Answer: %1").arg(q.choices[q.answer]), q.why, "");
        }
        return lines.join("\n").trim();
    }
    function choose(index) {
        if (pending || !question || pick >= 0 || index < 0 || index > 3) return;
        const next = picks.slice();
        next[current] = index;
        picks = next;
    }
    function advance() {
        if (pick < 0 || !question) return;
        current += 1;
        if (finished) resultChanged(I18n.tr("You got %1 of %2").arg(score).arg(questions.length) + "\n\n" + plainText());
    }
    function handleKey(event) {
        if (!visible || event.modifiers !== Qt.NoModifier) return false;
        if (event.key >= Qt.Key_1 && event.key <= Qt.Key_4) {
            choose(event.key - Qt.Key_1);
            event.accepted = true;
            return true;
        }
        if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter) {
            if (pending) return false;
            if (problem || finished) start();
            else if (pick >= 0) advance();
            else return false;
            event.accepted = true;
            return true;
        }
        return false;
    }
    Keys.onPressed: event => handleKey(event)
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
        Column {
            width: parent.width
            visible: root.pending
            spacing: 10 * root.s
            Row {
                spacing: 8 * root.s
                Working { active: root.pending && root.visible; size: 14 * root.s; anchors.verticalCenter: parent.verticalCenter }
                Label { text: I18n.tr("Making questions"); typeRole: "detail"; inkRole: "fg3" }
            }
            Item {
                width: parent.width
                height: 66 * root.s
                CutRect { anchors.fill: parent; cut: 7; fill: root.plane; strokeColor: root.stroke; strokeWidth: Lithic.design === "material" ? 0 : 1 }
                Column {
                    x: 13 * root.s; y: 15 * root.s; width: parent.width - 26 * root.s; spacing: 9 * root.s
                    CutRect { width: parent.width * 0.8; height: 8 * root.s; cut: 2; fill: Lithic.alpha(Lithic.fg, 0.10) }
                    CutRect { width: parent.width * 0.55; height: 8 * root.s; cut: 2; fill: Lithic.alpha(Lithic.fg, 0.07) }
                }
            }
            Repeater {
                model: root.pending ? 4 : 0
                delegate: Item {
                    required property int index
                    width: root.width; height: 42 * root.s
                    CutRect { anchors.fill: parent; cut: 6; fill: Lithic.field }
                    CutRect { x: 13 * root.s; anchors.verticalCenter: parent.verticalCenter; width: parent.width * (0.45 + index * 0.07); height: 7 * root.s; cut: 2; fill: Lithic.alpha(Lithic.fg, 0.08) }
                }
            }
        }
        Column {
            width: parent.width
            visible: root.problem.length > 0
            spacing: 8 * root.s
            Label { width: parent.width; text: root.problem; typeRole: "detail"; inkRole: "fg2"; wrapMode: Text.Wrap; elide: Text.ElideNone }
            SightActionButton { s: root.s; text: I18n.tr("Retry"); glyph: "refresh"; onClicked: root.start() }
        }
        Row {
            visible: root.questions.length > 0
            spacing: 6 * root.s
            Accessible.role: Accessible.StaticText
            Accessible.name: root.finished ? I18n.tr("Quiz complete") : I18n.tr("Question %1 of %2").arg(root.current + 1).arg(root.questions.length)
            Repeater {
                model: root.questions.length
                delegate: CutRect {
                    required property int index
                    width: 7 * root.s; height: 7 * root.s; cut: 3
                    fill: index < root.picks.length ? (root.picks[index] === root.questions[index].answer ? Lithic.jade : Lithic.jasper) : index === root.current ? Lithic.gold : Lithic.alpha(Lithic.fg, 0.15)
                    Behavior on fill { ColorAnimation { duration: Lithic.duration(160) } }
                }
            }
        }
        Column {
            width: parent.width
            visible: root.question !== null && !root.pending
            spacing: 10 * root.s
            Item {
                width: parent.width; height: heading.implicitHeight + 26 * root.s
                CutRect { anchors.fill: parent; cut: 7; fill: root.plane; strokeColor: root.stroke; strokeWidth: Lithic.design === "material" ? 0 : 1 }
                Label { id: heading; x: 13 * root.s; y: 13 * root.s; width: parent.width - 26 * root.s; text: root.question ? root.question.question : ""; typeRole: "row"; wrapMode: Text.Wrap; elide: Text.ElideNone }
            }
            Repeater {
                model: root.question ? root.question.choices : []
                delegate: T.AbstractButton {
                    id: choice
                    required property string modelData
                    required property int index
                    readonly property bool revealed: root.pick >= 0
                    readonly property bool correctChoice: root.question && index === root.question.answer
                    readonly property bool chosen: index === root.pick
                    width: root.width
                    implicitHeight: Math.max(44 * root.s, choiceText.implicitHeight + 20 * root.s)
                    leftPadding: 13 * root.s; rightPadding: 13 * root.s; topPadding: 10 * root.s; bottomPadding: 10 * root.s
                    hoverEnabled: true
                    focusPolicy: Qt.StrongFocus
                    Accessible.role: Accessible.Button
                    Accessible.name: String(index + 1) + ". " + modelData + (revealed && correctChoice ? ". " + I18n.tr("Correct answer") : revealed && chosen ? ". " + I18n.tr("Your answer") : "")
                    scale: down && !revealed ? 0.98 : 1
                    Behavior on scale { NumberAnimation { duration: Lithic.duration(90) } }
                    background: Item {
                        CutRect { anchors.fill: parent; cut: 6; fill: Lithic.field; strokeColor: root.stroke; strokeWidth: Lithic.design === "material" ? 0 : 1 }
                        CutRect {
                            anchors.fill: parent; cut: 6
                            fill: choice.correctChoice ? Lithic.alpha(Lithic.jade, 0.17) : Lithic.alpha(Lithic.jasper, 0.15)
                            opacity: choice.revealed && (choice.correctChoice || choice.chosen) ? 1 : 0
                            Behavior on opacity { NumberAnimation { duration: Lithic.duration(160) } }
                        }
                        CutRect {
                            anchors.fill: parent; cut: 6; fill: Lithic.alpha(Lithic.fg, 0.08)
                            opacity: choice.hovered && !choice.revealed ? 1 : 0
                            Behavior on opacity { NumberAnimation { duration: Lithic.duration(choice.hovered ? 160 : 100) } }
                        }
                        FocusRing { anchors.fill: parent; cut: 6; keyboardFocusVisible: choice.visualFocus }
                    }
                    contentItem: Row {
                        spacing: 10 * root.s
                        Label { width: 16 * root.s; text: String(choice.index + 1); typeRole: "monoSm"; inkRole: "fg3"; anchors.verticalCenter: parent.verticalCenter }
                        Label { id: choiceText; width: choice.width - 52 * root.s; text: choice.modelData; typeRole: "body"; wrapMode: Text.Wrap; elide: Text.ElideNone; anchors.verticalCenter: parent.verticalCenter }
                    }
                    onClicked: root.choose(index)
                    Keys.onReturnPressed: if (revealed) root.advance(); else root.choose(index)
                    Keys.onEnterPressed: if (revealed) root.advance(); else root.choose(index)
                }
            }
            Column {
                width: parent.width
                visible: root.pick >= 0
                spacing: 8 * root.s
                opacity: visible ? 1 : 0
                Behavior on opacity { NumberAnimation { duration: Lithic.duration(160) } }
                Label { text: root.question && root.pick === root.question.answer ? I18n.tr("That's right") : I18n.tr("Not quite"); typeRole: "label"; ink: root.question && root.pick === root.question.answer ? Lithic.jade : Lithic.jasper }
                Label { width: parent.width; text: root.question ? root.question.why : ""; typeRole: "detail"; inkRole: "fg2"; wrapMode: Text.Wrap; elide: Text.ElideNone }
                SightActionButton { s: root.s; text: root.current + 1 === root.questions.length ? I18n.tr("See results") : I18n.tr("Next question"); glyph: "arrow_forward"; onClicked: root.advance() }
            }
        }
        Item {
            visible: root.finished
            width: parent.width; height: endCard.implicitHeight + 26 * root.s
            CutRect { anchors.fill: parent; cut: 7; fill: root.plane; strokeColor: root.stroke; strokeWidth: Lithic.design === "material" ? 0 : 1 }
            Column {
                id: endCard
                x: 13 * root.s; y: 13 * root.s; width: parent.width - 26 * root.s; spacing: 13 * root.s
                Label { width: parent.width; text: I18n.tr("You got %1 of %2").arg(root.score).arg(root.questions.length); typeRole: "title"; wrapMode: Text.Wrap; elide: Text.ElideNone }
                SightActionButton { s: root.s; text: I18n.tr("New questions"); glyph: "refresh"; onClicked: root.start() }
            }
        }
    }
}
