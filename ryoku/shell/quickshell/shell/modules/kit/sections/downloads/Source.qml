pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons
import "../../../ask/lib/format.js" as Format
import "../../../ask/lib/downloads.js" as Downloads
import "../../lib/search.js" as Search
import "activity.js" as Activity

Item {
    id: root

    property var memory: null
    property var downloads: null
    property bool current: false
    property bool searching: false
    readonly property bool ready: root.downloads !== null && root.downloads.ready === true
    readonly property var stats: root.ready ? root.downloads.stats : ({})
    readonly property var tasks: root.ready ? root.downloads.tasks : []
    property int revision: 0
    property var activity: Activity.empty()
    readonly property var figure: {
        const speed = Number(root.stats.down || 0) + Number(root.stats.up || 0);
        if (speed > 0)
            return { text: Format.speed(speed), value: -1, tone: "busy" };
        return { text: root.activity.finished > 0 ? I18n.tr("%1 finished").arg(root.activity.finished) : root.ready ? I18n.tr("Nothing moving") : "", value: -1, tone: root.activity.finished > 0 ? "ok" : "" };
    }
    readonly property var lit: root.activity.failed > 0
        ? ({ text: I18n.tr("%1 failed").arg(root.activity.failed), tone: "error" })
        : root.activity.finished > 0 ? ({ text: I18n.tr("%1 finished").arg(root.activity.finished), tone: "ok" }) : null

    function seen() {
        root.activity = Activity.seen(root.activity);
    }

    function searchAll(query) {
        return Search.listRows(root.tasks, query, function(task) {
            return {
                id: String(task.id), glyph: Downloads.glyph(task), title: Downloads.nameOf(task),
                subtitle: String(task.state || ""), texts: [String(task.url || "")],
                open: { page: "queue", args: { task: String(task.id) } }
            };
        });
    }

    onTasksChanged: {
        if (root.ready)
            root.activity = Activity.update(root.activity, root.tasks, root.current);
        root.revision++;
    }
    onReadyChanged: if (root.ready) root.activity = Activity.update(root.activity, root.tasks, root.current)
    onCurrentChanged: if (root.current) root.seen()
    function connectExisting() {
        if (root.downloads && typeof root.downloads.openExisting === "function")
            root.downloads.openExisting();
    }
    onDownloadsChanged: Qt.callLater(root.connectExisting)
    Component.onCompleted: Qt.callLater(root.connectExisting)
}
