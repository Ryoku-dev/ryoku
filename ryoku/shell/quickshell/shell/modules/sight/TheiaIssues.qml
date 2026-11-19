pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "issuetext.js" as IssueText

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
    property string currentText: ""
    property var issues: []
    property string engine: ""
    property string errorText: ""
    property bool checking: false
    property bool addingWord: false
    property string wordError: ""
    property bool started: false
    property var lifetime: ({ closed: false, serial: 0 })
    property var pendingFixes: []
    property var lines: []
    implicitHeight: body.implicitHeight

    function publish() { panel.resultText = currentText; resultChanged(currentText); }
    function run(keepText) {
        if (!started) return;
        session.cancelWriting();
        fixMotion.stop();
        fieldText.opacity = 1;
        fieldShift.y = 0;
        const token = lifetime;
        const serial = ++token.serial;
        if (keepText !== true) currentText = sourceText;
        issues = [];
        engine = "";
        addingWord = false;
        wordError = "";
        errorText = "";
        publish();
        if (!currentText.trim()) { checking = false; return; }
        checking = true;
        session.check(currentText, [action === "inclusive" ? "inclusive" : "grammar"], function(result, error) {
            if (token.closed || token.serial !== serial) return;
            root.checking = false;
            if (error) { root.errorText = String(error.message || I18n.tr("The text could not be checked.")); return; }
            root.engine = String(result.engine || "");
            root.issues = (result.issues || []).filter(function(issue) {
                return Number(issue.start) >= 0 && Number(issue.end) > Number(issue.start) && Number(issue.end) <= root.currentText.length;
            });
            Qt.callLater(root.layoutLines);
        });
    }
    function ignore(issue) { issues = issues.filter(function(row) { return row.id !== issue.id; }); }
    function addWord(issue) {
        if (addingWord || checking || fixMotion.running || issue.category !== "spelling") return;
        const word = currentText.substring(issue.start, issue.end);
        if (!word.trim()) return;
        const token = lifetime;
        const serial = token.serial;
        addingWord = true;
        wordError = "";
        session.addWord(word, function(result, error) {
            if (token.closed || token.serial !== serial) return;
            root.addingWord = false;
            if (error) {
                root.wordError = String(error.message || I18n.tr("The word could not be added."));
                return;
            }
            root.run(true);
        });
    }
    function hideKind(issue) {
        const off = (catalog.option("proofread.off", []) || []).slice();
        if (off.indexOf(issue.category) < 0) off.push(issue.category);
        ShellSettings.patch("sight.proofread.off", off);
        issues = issues.filter(function(row) { return row.category !== issue.category; });
    }
    function accept(rows) {
        if (fixMotion.running || addingWord) return;
        pendingFixes = rows.filter(function(issue) { return (issue.replacements || []).length > 0; }).slice().sort(function(a, b) { return a.start - b.start; });
        if (pendingFixes.length) fixMotion.start();
    }
    function applyFixes() {
        const accepted = [];
        let end = -1;
        for (let i = 0; i < pendingFixes.length; ++i) {
            const issue = pendingFixes[i];
            if (issue.start < end) continue;
            accepted.push(issue);
            end = issue.end;
        }
        let text = currentText;
        for (let j = accepted.length - 1; j >= 0; --j) {
            const issue = accepted[j];
            text = text.slice(0, issue.start) + String(issue.replacements[0]) + text.slice(issue.end);
        }
        const remaining = [];
        for (let k = 0; k < issues.length; ++k) {
            const issue = issues[k];
            let delta = 0;
            let overlaps = false;
            for (let n = 0; n < accepted.length; ++n) {
                const fix = accepted[n];
                if (issue.start < fix.end && issue.end > fix.start) { overlaps = true; break; }
                if (fix.end <= issue.start) delta += String(fix.replacements[0]).length - (fix.end - fix.start);
            }
            if (!overlaps) remaining.push(Object.assign({}, issue, { start: issue.start + delta, end: issue.end + delta }));
        }
        currentText = text;
        issues = remaining;
        pendingFixes = [];
        publish();
    }
    function layoutLines() {
        const result = [];
        for (let i = 0; i < issues.length; ++i) {
            const issue = issues[i];
            let segment = null;
            for (let p = issue.start; p < issue.end; ++p) {
                const here = fieldText.positionToRectangle(p);
                const next = fieldText.positionToRectangle(p + 1);
                if (currentText[p] === "\n") { segment = null; continue; }
                const left = next.y === here.y ? Math.min(here.x, next.x) : here.x;
                const right = next.y === here.y ? Math.max(here.x, next.x) : here.x + fieldMetrics.advanceWidth(currentText[p]);
                if (!segment || segment.y !== here.y + here.height - root.s) {
                    segment = { x: left, y: here.y + here.height - root.s, width: right - left, error: issue.severity === "error" };
                    result.push(segment);
                } else {
                    segment.width = Math.max(segment.x + segment.width, right) - Math.min(segment.x, left);
                    segment.x = Math.min(segment.x, left);
                }
            }
        }
        lines = result;
    }
    onIssuesChanged: Qt.callLater(layoutLines)
    onCurrentTextChanged: Qt.callLater(layoutLines)
    onSourceTextChanged: if (started) run()
    onActionChanged: if (started) run()
    Component.onCompleted: { started = true; run(); }
    Component.onDestruction: { lifetime.closed = true; ++lifetime.serial; session.cancel(); session.cancelWriting(); }
    FontMetrics { id: fieldMetrics; font: fieldText.font }

    SequentialAnimation {
        id: fixMotion
        NumberAnimation { target: fieldText; property: "opacity"; to: 0; duration: Lithic.duration(80) }
        ScriptAction { script: { root.applyFixes(); fieldShift.y = 4 * root.s; } }
        ParallelAnimation {
            NumberAnimation { target: fieldText; property: "opacity"; to: 1; duration: Lithic.duration(80) }
            NumberAnimation { target: fieldShift; property: "y"; to: 0; duration: Lithic.duration(80); easing.type: Easing.OutCubic }
        }
    }
    Column {
        id: body
        width: parent.width
        spacing: 10 * root.s
        Row {
            visible: root.checking
            spacing: 8 * root.s
            Working { active: root.checking && root.visible; size: 14 * root.s; anchors.verticalCenter: parent.verticalCenter }
            Label { typeRole: "detail"; inkRole: "fg3"; text: I18n.tr("Checking writing") }
        }
        Column {
            width: parent.width; visible: root.errorText.length > 0; spacing: 8 * root.s
            Label { width: parent.width; typeRole: "detail"; inkRole: "jasper"; wrapMode: Text.Wrap; elide: Text.ElideNone; text: root.errorText }
            TheiaChip { text: I18n.tr("Try again"); icon: "refresh"; onClicked: root.run() }
        }
        Flow {
            width: parent.width; spacing: 8 * root.s; visible: !root.checking && !root.errorText && root.engine.length > 0
            Label { height: 28 * root.s; verticalAlignment: Text.AlignVCenter; typeRole: "label"; text: root.issues.length === 1 ? I18n.tr("1 fix") : root.issues.length ? I18n.tr("%1 fixes").arg(root.issues.length) : I18n.tr("No fixes needed") }
            TheiaChip { text: I18n.tr("Fix all"); small: true; enabled: root.issues.some(function(issue) { return (issue.replacements || []).length > 0; }); onClicked: root.accept(root.issues) }
        }
        Label {
            width: parent.width; visible: root.engine.length > 0; typeRole: "detail"; inkRole: "fg3"; wrapMode: Text.Wrap; elide: Text.ElideNone
            text: root.engine === "local" ? I18n.tr("Checked on this computer") : I18n.tr("Checked by %1").arg(root.session.model || I18n.tr("your model"))
        }
        CutRect {
            width: parent.width; implicitHeight: fieldText.contentHeight + 26 * root.s; height: implicitHeight
            cut: 6; fill: root.plane; strokeColor: root.stroke; strokeWidth: 1
            TextEdit {
                id: fieldText
                x: 13 * root.s; y: 13 * root.s; width: parent.width - 26 * root.s; height: contentHeight
                text: root.currentText; textFormat: TextEdit.PlainText; readOnly: true; selectByMouse: true
                color: Lithic.fg; selectionColor: Lithic.sel; selectedTextColor: Lithic.onSel
                font.family: Lithic.type.body.family; font.pixelSize: 14 * root.s; wrapMode: TextEdit.Wrap
                transform: Translate { id: fieldShift }
                onContentHeightChanged: Qt.callLater(root.layoutLines)
                onWidthChanged: Qt.callLater(root.layoutLines)
                Repeater {
                    model: root.lines
                    delegate: CutRect {
                        required property var modelData
                        x: modelData.x; y: modelData.y; width: Math.max(root.s, modelData.width); height: 2 * root.s
                        cut: 0; fill: modelData.error ? Lithic.jasper : Lithic.sel
                    }
                }
            }
        }
        Label {
            width: parent.width; visible: root.wordError.length > 0; typeRole: "detail"; inkRole: "jasper"
            wrapMode: Text.Wrap; elide: Text.ElideNone; text: root.wordError
        }
        Repeater {
            model: root.issues
            delegate: CutRect {
                id: issueCard
                required property var modelData
                width: body.width; implicitHeight: issueBody.implicitHeight + 20 * root.s; height: implicitHeight
                cut: 6; fill: root.plane; strokeColor: root.stroke; strokeWidth: 1
                Column {
                    id: issueBody
                    x: 10 * root.s; y: 10 * root.s; width: parent.width - 20 * root.s; spacing: 8 * root.s
                    Label {
                        width: parent.width; typeRole: "detail"; wrapMode: Text.Wrap; elide: Text.ElideNone
                        textFormat: Text.RichText
                        text: IssueText.messageHtml(issueCard.modelData.message, Lithic.font.mono)
                    }
                    Text {
                        width: parent.width; wrapMode: Text.Wrap; color: Lithic.fg2; font.family: Lithic.font.mono; font.pixelSize: 12 * root.s
                        textFormat: Text.PlainText
                        text: (issueCard.modelData.replacements || []).length ? I18n.tr("%1 → %2").arg(root.currentText.substring(issueCard.modelData.start, issueCard.modelData.end)).arg(issueCard.modelData.replacements[0]) : root.currentText.substring(issueCard.modelData.start, issueCard.modelData.end)
                    }
                    Flow {
                        width: parent.width; spacing: 6 * root.s
                        TheiaChip { text: I18n.tr("Accept"); icon: "check"; small: true; enabled: (issueCard.modelData.replacements || []).length > 0 && !fixMotion.running && !root.addingWord; onClicked: root.accept([issueCard.modelData]) }
                        TheiaChip { text: I18n.tr("Ignore"); small: true; onClicked: root.ignore(issueCard.modelData) }
                        TheiaChip { text: I18n.tr("Add to dictionary"); icon: "add"; small: true; visible: issueCard.modelData.category === "spelling"; enabled: !root.addingWord && !root.checking && !fixMotion.running; onClicked: root.addWord(issueCard.modelData) }
                        TheiaChip { text: I18n.tr("Hide this kind"); small: true; enabled: !!issueCard.modelData.category; onClicked: root.hideKind(issueCard.modelData) }
                    }
                }
            }
        }
    }
}
