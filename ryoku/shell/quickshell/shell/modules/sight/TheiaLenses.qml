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
    readonly property var lenses: {
        const all = catalog.lenses;
        const enabled = catalog.option("deepread.lenses", all.map(function(lens) { return lens.id; }));
        const rows = [];
        for (var i = 0; i < enabled.length; ++i) {
            for (var j = 0; j < all.length; ++j) {
                if (String(enabled[i]) === all[j].id && !rows.some(function(row) { return row.id === all[j].id; })) {
                    rows.push(all[j]);
                    break;
                }
            }
        }
        return rows;
    }
    property string lens: lenses.length > 0 ? lenses[0].id : ""
    readonly property string hint: {
        for (var i = 0; i < lenses.length; ++i)
            if (lenses[i].id === lens) return lenses[i].hint;
        return "";
    }
    readonly property string answer: String(session.answers[action] || "")
    readonly property bool busy: (session.busy && session.turnMode === action) || (session.queued !== null && session.queued.mode === action)
    readonly property bool failed: session.errorText.length > 0 || ["off", "not-installed", "error"].indexOf(session.client.state) >= 0
    implicitHeight: body.implicitHeight
    height: implicitHeight
    onAnswerChanged: resultChanged(answer)

    function run() {
        if (!lens) return;
        if (session.client.state === "error") session.client.connectSocket();
        session.request("deepread", "", sourceText, panel.languageName(panel.effectiveTarget), panel.languageName(panel.source), { lens: lens });
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
        Flow {
            width: parent.width
            spacing: 6 * root.s
            Repeater {
                model: root.lenses
                delegate: TheiaChip {
                    required property var modelData
                    text: modelData.label
                    selected: root.lens === modelData.id
                    enabled: !root.busy
                    small: true
                    onClicked: {
                        root.lens = modelData.id;
                        root.run();
                    }
                }
            }
        }
        Label {
            width: parent.width
            text: root.lenses.length > 0 ? root.hint : I18n.tr("Choose a lens in Theia settings to use Deep read.")
            typeRole: "detail"; inkRole: "fg3"; wrapMode: Text.Wrap; elide: Text.ElideNone
        }
        SightActionButton {
            visible: root.lenses.length === 0
            s: root.s
            text: I18n.tr("Open Theia settings")
            onClicked: root.panel.settings(false)
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
            Label { typeRole: "detail"; inkRole: "fg3"; text: I18n.tr("Reading") }
            SightActionButton { s: root.s; text: I18n.tr("Stop"); onClicked: root.session.cancel() }
        }
    }
}
