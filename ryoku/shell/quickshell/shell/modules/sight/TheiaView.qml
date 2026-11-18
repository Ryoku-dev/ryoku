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
    property bool active: false
    readonly property var sources: ({
        issues: "TheiaIssues.qml", score: "TheiaScore.qml", rewrite: "TheiaRewrite.qml",
        summary: "TheiaSummary.qml", lenses: "TheiaLenses.qml", solve: "TheiaSolve.qml",
        quiz: "TheiaQuiz.qml", define: "TheiaDefine.qml"
    })
    readonly property bool typing: loader.item && typeof loader.item.typing === "boolean" ? loader.item.typing : false
    signal resultChanged(string text)
    implicitHeight: loader.status === Loader.Error ? failure.implicitHeight : loader.item ? loader.item.implicitHeight : 0

    function clear() { loader.source = ""; }
    function reload() { Qt.callLater(loadView); }
    function loadView() {
        loader.source = "";
        if (!active) return;
        const row = catalog.action(action);
        root.resultChanged("");
        const file = row ? sources[row.view] : "";
        if (!file) return;
        loader.setSource(Qt.resolvedUrl(file), {
            session: root.session, card: root.card, catalog: root.catalog, panel: root.panel,
            action: root.action, sourceText: Qt.binding(function() { return root.sourceText; })
        });
    }
    function handleKey(event) {
        if (!active || !loader.item || typeof loader.item.handleKey !== "function") return false;
        return loader.item.handleKey(event) === true;
    }
    onActionChanged: reload()
    onActiveChanged: reload()
    Component.onCompleted: reload()
    Loader {
        id: loader
        width: parent.width
        active: root.active
    }
    Connections {
        target: loader.item
        function onResultChanged(text) { root.resultChanged(text); }
    }
    Label {
        id: failure
        width: parent.width
        visible: loader.status === Loader.Error
        text: I18n.tr("This action could not open.")
        typeRole: "detail"; inkRole: "jasper"
        wrapMode: Text.Wrap; elide: Text.ElideNone
    }
}
