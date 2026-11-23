import QtQuick
import Ryoku.Ui.Singletons
import "../kit" as K
import "../../../../modules/launcher/shared/Singletons" as Launcher
import "../../../../modules/launcher/shared/providers/web/engines.js" as Engines

// A web search: the query, the engine it goes to, and the DuckDuckGo instant
// answer under it once the web provider has fetched one for this query.
LeadGeneric {
    id: root

    readonly property string query: root.row ? String(root.row.title || "") : ""
    readonly property var answer: {
        void Launcher.Dispatcher.revision;
        const web = Launcher.Dispatcher.registry["web"];
        return web && web.answer && web.answer.available === true ? web.answer : null;
    }

    title: root.query.length > 0 ? Engines.parseBang(root.query).query || root.query : ""
    subtitle: root.query.length > 0 ? Engines.engineName(root.query, "g") : ""

    body: Column {
        visible: root.answer !== null
        width: root.width
        spacing: K.Style.s1

        K.Label {
            width: parent.width
            visible: text.length > 0
            text: root.answer && String(root.answer.source || "").length > 0
                ? I18n.tr("via %1").arg(root.answer.source) : ""
            role: "micro"
        }

        K.Label {
            width: parent.width
            visible: text.length > 0
            text: root.answer ? String(root.answer.heading || "") : ""
            role: "title"
            wrapMode: Text.Wrap
            maximumLineCount: 2
            elide: Text.ElideRight
        }

        K.Label {
            width: parent.width
            visible: text.length > 0
            text: root.answer ? String(root.answer.text || "") : ""
            role: "body"
            tone: "dim"
            wrapMode: Text.Wrap
            maximumLineCount: 6
            elide: Text.ElideRight
        }
    }
}
