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
    readonly property real s: panel.s || 1
    readonly property var planes: Lithic.planes(Lithic.design)
    readonly property color plane: Lithic.design === "lithic" ? planes.pane : Lithic.design === "fluent" ? Lithic.mix(planes.card, Lithic.sel, 0.025) : Lithic.stone4
    readonly property color stroke: Lithic.design === "material" ? "transparent" : Lithic.design === "adwaita" ? Lithic.alpha(Lithic.fg, Lithic.dark ? 0.10 : 0.16) : Lithic.design === "fluent" ? planes.cardStroke : Lithic.vein
    property string variant: String(catalog.option("rewrite.variant", "rephrase"))
    property string audience: String(catalog.option("rewrite.audience", "general"))
    property string tone: String((catalog.option("tone.favorites", []) || [])[0] || "friendly")
    property bool useVoice: catalog.option("voice.use", true) === true
    property string output: ""
    property var replies: []
    property int beforeScore: -1
    property int afterScore: -1
    property string scoreError: ""
    property string errorText: ""
    property bool requested: false
    property bool finished: false
    property bool started: false
    property var lifetime: ({ closed: false, serial: 0 })
    readonly property bool busy: requested && (session.busy || session.queued !== null)
    readonly property var variants: [
        { id: "rephrase", label: I18n.tr("Rephrase") },
        { id: "shorter", label: I18n.tr("Shorter") },
        { id: "longer", label: I18n.tr("Longer") },
        { id: "simpler", label: I18n.tr("Simpler") }
    ]
    readonly property var audiences: [
        { id: "general", label: I18n.tr("General") },
        { id: "expert", label: I18n.tr("Expert") },
        { id: "esl", label: I18n.tr("Simple English") }
    ]
    readonly property var tones: {
        const favorites = catalog.option("tone.favorites", ["formal", "friendly", "confident", "direct", "empathetic"]) || [];
        return catalog.tones.filter(function(row) { return favorites.indexOf(row.id) >= 0; }).sort(function(a, b) { return favorites.indexOf(a.id) - favorites.indexOf(b.id); });
    }
    implicitHeight: body.implicitHeight

    function languageName(code) {
        for (let i = 0; i < I18n.langs.length; ++i)
            if (String(I18n.langs[i].code) === code) return String(I18n.langs[i].name);
        return "";
    }
    function publish(text) { panel.resultText = text; resultChanged(text); }
    function options() {
        const result = {};
        if (action === "rewrite") { result.variant = variant; result.audience = audience; }
        if (action === "tone" || action === "reply") result.tone = tone;
        if (action === "fluent") result.audience = audience === "esl" ? "esl" : "general";
        if (catalog.feature("voice") && useVoice) result.voice = "on";
        return result;
    }
    function checkScore(text, before, serial) {
        const token = lifetime;
        session.check(text, ["aitell"], function(result, error) {
            if (token.closed || serial !== token.serial) return;
            if (error || !result || !result.aitell) {
                root.scoreError = String(error && error.message || I18n.tr("The writing score is unavailable."));
                return;
            }
            const score = Math.max(0, Math.min(100, Number(result.aitell.score) || 0));
            if (before) root.beforeScore = score; else root.afterScore = score;
        });
    }
    function run() {
        if (!started) return;
        session.cancel();
        session.cancelWriting();
        const serial = ++lifetime.serial;
        output = "";
        replies = [];
        errorText = "";
        scoreError = "";
        beforeScore = -1;
        afterScore = -1;
        requested = false;
        finished = false;
        publish("");
        if (!sourceText.trim()) return;
        if (action === "humanize") checkScore(sourceText, true, serial);
        requested = true;
        session.request(action, "", sourceText, languageName(panel.target || ""), languageName(panel.source || ""), options());
    }
    function updateAnswer() {
        if (!requested || session.turnMode !== action) return;
        const answer = String(session.answers[action] || "");
        if (action !== "reply") {
            output = answer;
            publish(output);
        }
        finish();
    }
    function finish() {
        if (!requested || finished || session.busy || session.queued !== null) return;
        finished = true;
        if (session.errorText) { errorText = session.errorText; return; }
        const answer = String(session.answers[action] || "");
        if (action === "reply") {
            const parsed = session.parseJson(answer);
            if (!parsed || !Array.isArray(parsed.replies) || parsed.replies.length !== 3 || parsed.replies.some(function(reply) { return !reply || typeof reply.text !== "string" || !reply.text.trim(); })) {
                errorText = I18n.tr("The model did not return three readable replies.");
                return;
            }
            replies = parsed.replies;
            publish(replies.map(function(reply, index) { return root.replyLabel(index) + "\n" + reply.text; }).join("\n\n"));
        } else {
            output = answer;
            publish(output);
            if (!output.trim()) { errorText = I18n.tr("The model returned no text."); return; }
            if (action === "humanize") checkScore(output, false, lifetime.serial);
        }
    }
    function replyLabel(index) { return index === 0 ? I18n.tr("Short") : index === 1 ? I18n.tr("Warm") : I18n.tr("Detailed"); }
    Connections {
        target: root.session
        function onAnswersChanged() { root.updateAnswer(); }
        function onBusyChanged() { Qt.callLater(root.finish); }
        function onQueuedChanged() { Qt.callLater(root.finish); }
        function onErrorTextChanged() { if (root.requested && root.session.errorText) root.errorText = root.session.errorText; }
    }
    onSourceTextChanged: if (started) run()
    onActionChanged: if (started) run()
    Component.onCompleted: { started = true; run(); }
    Component.onDestruction: { lifetime.closed = true; ++lifetime.serial; session.cancel(); session.cancelWriting(); }

    Column {
        id: body
        width: parent.width; spacing: 10 * root.s
        Flow {
            width: parent.width; spacing: 6 * root.s; visible: root.action === "rewrite"
            Repeater {
                model: root.variants
                delegate: TheiaChip {
                    required property var modelData
                    text: modelData.label; selected: root.variant === modelData.id; enabled: !root.busy
                    onClicked: { root.variant = modelData.id; root.run(); }
                }
            }
        }
        Flow {
            width: parent.width; spacing: 6 * root.s; visible: root.action === "tone" || root.action === "reply"
            Repeater {
                model: root.tones
                delegate: TheiaChip {
                    required property var modelData
                    text: modelData.label; selected: root.tone === modelData.id; enabled: !root.busy
                    onClicked: { root.tone = modelData.id; root.run(); }
                }
            }
        }
        Flow {
            width: parent.width; spacing: 6 * root.s; visible: root.action === "rewrite" || root.action === "fluent"
            Repeater {
                model: root.action === "fluent" ? root.audiences.filter(function(row) { return row.id !== "expert"; }) : root.audiences
                delegate: TheiaChip {
                    required property var modelData
                    text: modelData.label; selected: root.audience === modelData.id; enabled: !root.busy
                    onClicked: { root.audience = modelData.id; root.run(); }
                }
            }
        }
        TheiaChip {
            visible: root.catalog.feature("voice"); text: I18n.tr("In my voice"); icon: "record_voice_over"; selected: root.useVoice; enabled: !root.busy
            onClicked: { root.useVoice = !root.useVoice; root.run(); }
        }
        Row {
            spacing: 8 * root.s; visible: root.busy
            Working { active: root.busy && root.visible; size: 14 * root.s; anchors.verticalCenter: parent.verticalCenter }
            Label { typeRole: "detail"; inkRole: "fg3"; text: root.action === "reply" ? I18n.tr("Writing replies") : I18n.tr("Rewriting") }
        }
        Column {
            width: parent.width; spacing: 8 * root.s; visible: root.errorText.length > 0
            Label { width: parent.width; typeRole: "detail"; inkRole: "jasper"; wrapMode: Text.Wrap; elide: Text.ElideNone; text: root.errorText }
            TheiaChip { text: I18n.tr("Try again"); icon: "refresh"; enabled: !root.busy; onClicked: root.run() }
        }
        CutRect {
            width: parent.width; visible: root.action !== "reply" && root.output.length > 0
            implicitHeight: result.contentHeight + 26 * root.s; height: implicitHeight
            cut: 6; fill: root.plane; strokeColor: root.stroke; strokeWidth: 1
            TextEdit {
                id: result
                x: 13 * root.s; y: 13 * root.s; width: parent.width - 26 * root.s; height: contentHeight
                text: root.output; textFormat: TextEdit.PlainText; readOnly: true; selectByMouse: true; wrapMode: TextEdit.Wrap
                font.family: Lithic.type.body.family; font.pixelSize: 14 * root.s; color: Lithic.fg
                selectionColor: Lithic.sel; selectedTextColor: Lithic.onSel
            }
        }
        Column {
            width: parent.width; spacing: 6 * root.s; visible: root.action === "humanize" && (root.beforeScore >= 0 || root.afterScore >= 0 || root.scoreError.length > 0)
            Label { width: parent.width; typeRole: "detail"; inkRole: "fg2"; text: I18n.tr("Machine-writing check") }
            Label {
                width: parent.width; typeRole: "detail"; wrapMode: Text.Wrap; elide: Text.ElideNone
                text: root.beforeScore >= 0 && root.afterScore >= 0 ? I18n.tr("%1 before → %2 after").arg(root.beforeScore).arg(root.afterScore) : root.beforeScore >= 0 ? I18n.tr("%1 before").arg(root.beforeScore) : root.afterScore >= 0 ? I18n.tr("%1 after").arg(root.afterScore) : ""
            }
            Label { width: parent.width; typeRole: "detail"; inkRole: "fg3"; wrapMode: Text.Wrap; elide: Text.ElideNone; text: root.scoreError || I18n.tr("A rough guide, not proof.") }
        }
        Repeater {
            model: root.replies
            delegate: CutRect {
                id: replyCard
                required property var modelData
                required property int index
                width: body.width; implicitHeight: replyBody.implicitHeight + 26 * root.s; height: implicitHeight
                cut: 6; fill: root.plane; strokeColor: root.stroke; strokeWidth: 1
                Column {
                    id: replyBody
                    x: 13 * root.s; y: 13 * root.s; width: parent.width - 26 * root.s; spacing: 8 * root.s
                    Label { width: parent.width; typeRole: "label"; text: root.replyLabel(replyCard.index) }
                    TextEdit {
                        width: parent.width; height: contentHeight; text: replyCard.modelData.text
                        textFormat: TextEdit.PlainText; readOnly: true; selectByMouse: true; wrapMode: TextEdit.Wrap
                        font.family: Lithic.type.body.family; font.pixelSize: 14 * root.s; color: Lithic.fg
                        selectionColor: Lithic.sel; selectedTextColor: Lithic.onSel
                    }
                    Flow {
                        width: parent.width; spacing: 8 * root.s
                        TheiaChip { text: I18n.tr("Copy"); icon: "content_copy"; small: true; onClicked: root.panel.copyText(replyCard.modelData.text) }
                        TheiaChip { text: I18n.tr("Replace"); icon: "find_replace"; small: true; visible: root.catalog.feature("inplace") && root.card.source === "highlight"; onClicked: root.panel.replaceWith(replyCard.modelData.text) }
                    }
                }
            }
        }
        TheiaChip { visible: root.finished && !root.errorText && (root.output.length > 0 || root.replies.length > 0); text: I18n.tr("Another version"); icon: "refresh"; enabled: !root.busy; onClicked: root.run() }
    }
}
