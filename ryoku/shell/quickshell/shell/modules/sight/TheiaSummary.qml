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
    readonly property var styles: ["points", "tldr", "main"]
    property string style: {
        const value = String(catalog.option("summarize.style", "points"));
        return styles.indexOf(value) >= 0 ? value : "points";
    }
    readonly property string answer: String(session.answers[action] || "")
    readonly property bool busy: (session.busy && session.turnMode === action) || (session.queued !== null && session.queued.mode === action)
    readonly property bool failed: session.errorText.length > 0 || ["off", "not-installed", "error"].indexOf(session.client.state) >= 0
    property real readingMinutes: 0
    property string checkId: ""
    property var lifetime: ({ closed: false })
    readonly property int savedMinutes: {
        const plain = answer.replace(/```[\s\S]*?```/g, " ").replace(/[#*_`>\[\]()]/g, " ").trim();
        const words = plain ? plain.split(/\s+/).length : 0;
        return Math.max(0, Math.floor(readingMinutes - words / 230));
    }
    implicitHeight: body.implicitHeight
    height: implicitHeight
    onAnswerChanged: resultChanged(answer)

    function run() {
        if (session.client.state === "error") session.client.connectSocket();
        session.request("summarize", "", sourceText, panel.languageName(panel.effectiveTarget), panel.languageName(panel.source), { style: style });
    }
    function retry() {
        if (session.errorCode === "no_vision") panel.settings(true);
        else if (session.errorCode === "local_only" || session.client.state === "off" || session.client.state === "not-installed") panel.settings(false);
        else run();
    }
    Component.onCompleted: {
        run();
        if (sourceText.trim()) {
            const token = lifetime;
            checkId = session.check(sourceText, ["clarity"], function(result, error) {
                if (token.closed || error) return;
                root.readingMinutes = Math.max(0, Number(result && result.stats && result.stats.minutes || 0));
            });
        }
    }
    Component.onDestruction: {
        lifetime.closed = true;
        if (checkId) session.cancelWriting(checkId);
        if ((session.busy && session.turnMode === action) || (session.queued !== null && session.queued.mode === action)) session.cancel();
    }

    Column {
        id: body
        width: parent.width
        spacing: 13 * root.s
        Segmented {
            width: parent.width
            height: 32 * root.s
            model: [I18n.tr("Key points"), I18n.tr("TL;DR"), I18n.tr("Main point")]
            currentIndex: root.styles.indexOf(root.style)
            accessibleName: I18n.tr("Summary style")
            enabled: !root.busy
            onActivated: index => {
                root.style = root.styles[index];
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
        SightThread {
            width: parent.width
            s: root.s
            keyTarget: root.panel
            turns: root.answer ? [{ question: "", answer: root.answer }] : []
        }
        Row {
            visible: root.busy
            spacing: 8 * root.s
            Working { active: root.busy && root.visible; size: 14 * root.s; anchors.verticalCenter: parent.verticalCenter }
            Label { typeRole: "detail"; inkRole: "fg3"; text: I18n.tr("Summarizing") }
            SightActionButton { s: root.s; text: I18n.tr("Stop"); onClicked: root.session.cancel() }
        }
        Label {
            width: parent.width
            visible: root.answer.trim().length > 0 && !root.busy && !root.failed && root.savedMinutes >= 1
            text: I18n.tr("Saves about %1 min of reading").arg(root.savedMinutes)
            typeRole: "caption"; inkRole: "fg3"; wrapMode: Text.Wrap; elide: Text.ElideNone
        }
    }
}
