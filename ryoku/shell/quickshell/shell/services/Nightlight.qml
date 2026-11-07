pragma Singleton
import QtQuick
import Quickshell
import Ryoku.Ui
import Ryoku.Ui.Singletons

// QML view of the daemon `nightlight` topic. The backend's state lives in
// ryoku-shell (nightlight.go), so QML never pgreps or shells out to
// ryoku-cmd-nightlight for state: the bus streams {on, temperature, schedule}
// on every change, and the `nightlight.toggle` / `nightlight.set` calls send
// the intent back. The daemon publishes an off frame at
// startup, so the tile is never blank before the first event.
Singleton {
    id: root

    property bool on: false
    property int temperature: 4000
    // "sun" follows sunset to sunrise where the user is; "clock" follows the
    // hours the user set; "off" is manual only.
    property string schedule: "off"

    function toggle() {
        root.call("nightlight.toggle", {});
    }

    function setEnabled(on, temperature) {
        root.call("nightlight.set", {
            on: on === true,
            temperature: typeof temperature === "number" ? temperature : 0
        });
    }

    // Same semantics as the Hub's Displays page: warmth applies live while the
    // light is on and is kept for the next switch-on otherwise (nightlight.set
    // with on:false only turns it off). Optimistic; the frame confirms it.
    function setTemperature(kelvin) {
        root.temperature = kelvin;
        if (root.on)
            root.setEnabled(true, kelvin);
    }

    function setSchedule(sun) {
        root.schedule = sun ? "sun" : "off";
        root.call("nightlight.set", { schedule: root.schedule });
    }

    function apply(f) {
        if (!f || typeof f !== "object")
            return;
        root.on = f.on === true;
        if (typeof f.temperature === "number" && f.temperature > 0)
            root.temperature = f.temperature;
        if (f.schedule === "off" || f.schedule === "sun" || f.schedule === "clock")
            root.schedule = f.schedule;
    }

    function call(method, args) {
        Bus.call(method, args);
    }

    BusTopic {
        id: topicFeed
        topic: "nightlight"
        onUpdated: root.apply(topicFeed.data)
    }
}
