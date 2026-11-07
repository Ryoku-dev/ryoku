pragma Singleton
import QtQuick
import Quickshell
import Ryoku.Ui
import Ryoku.Ui.Singletons

Singleton {
    id: root

    property var frame: ({ status: "loading", region: "", source: "", days: [] })
    readonly property string status: frame.status || "loading"
    readonly property string region: frame.region || ""
    readonly property string errorText: frame.error || ""
    readonly property var days: Array.isArray(frame.days) ? frame.days : []
    property var byDate: ({})
    property string requestedRegion: ""
    property var requestedYears: []
    property bool configured: false

    function forDate(key) { return root.byDate[key] || []; }
    function hasDate(key) { return root.forDate(key).length > 0; }

    function configure(region, years) {
        const normalizedYears = (years || []).slice().sort();
        const signature = normalizedYears.join(",");
        if (root.requestedRegion === region && root.requestedYears.join(",") === signature)
            return;
        root.requestedRegion = region;
        root.requestedYears = normalizedYears;
        root.configured = true;
        root.sendConfigure();
    }

    function sendConfigure() {
        root.send("calendar.configure", { region: root.requestedRegion, years: root.requestedYears });
    }

    function apply(frame) {
        if (!frame || typeof frame !== "object")
            return;
        const days = Array.isArray(frame.days) ? frame.days : [];
        const index = {};
        for (const holiday of days) {
            if (!holiday || typeof holiday.date !== "string")
                continue;
            if (!index[holiday.date])
                index[holiday.date] = [];
            index[holiday.date].push(holiday);
        }
        root.frame = Array.isArray(frame.days) ? frame : Object.assign({}, frame, { days: days });
        root.byDate = index;
    }

    function send(method, args) {
        Bus.call(method, args);
    }

    BusTopic {
        id: topicFeed
        topic: "calendar"
        onUpdated: root.apply(topicFeed.data)
    }

    // A restarted daemon has forgotten the region and years and would stay on
    // "loading"; hand them over again whenever the bus (re)connects.
    Connections {
        target: Bus
        function onConnectedChanged() {
            if (Bus.connected && root.configured)
                root.sendConfigure();
        }
    }
}
