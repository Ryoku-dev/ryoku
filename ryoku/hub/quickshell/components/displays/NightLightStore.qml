import QtQuick
import Ryoku.Ui as RU
import Ryoku.Ui.Singletons
import "../../Singletons"

// the night light as a Changes store: the daemon's nightlight topic is the truth,
// nightlight.set the write; Changes owns it so undo works from any page
QtObject {
    id: store

    property bool on: false
    property int temperature: 4000
    property string schedule: "off"
    property int marginMin: 60
    property int startAt: 22 * 60
    property int stopAt: 7 * 60

    function read(key) {
        switch (key) {
        case "on": return store.on;
        case "temperature": return store.temperature;
        case "schedule": return store.schedule;
        case "marginMin": return store.marginMin;
        case "startAt": return store.startAt;
        case "stopAt": return store.stopAt;
        }
        return undefined;
    }

    // optimistic: the confirming frame lands on the same values; a failed write puts the old one back
    function write(key, v, done) {
        var before = store.read(key);
        var args = null;
        switch (key) {
        case "on":
            store.on = v === true;
            args = { on: store.on, temperature: store.temperature };
            break;
        case "temperature":
            store.temperature = Math.round(v);
            // warmth reaches the screen only while it is warm
            if (store.on)
                args = { on: true, temperature: store.temperature };
            break;
        case "schedule":
            store.schedule = String(v);
            args = { schedule: store.schedule, marginMin: store.marginMin, startAt: store.startAt, stopAt: store.stopAt };
            break;
        case "marginMin":
            store.marginMin = Math.round(v);
            args = { schedule: store.schedule, marginMin: store.marginMin };
            break;
        case "startAt":
            store.startAt = Math.round(v);
            args = { schedule: store.schedule, startAt: store.startAt };
            break;
        case "stopAt":
            store.stopAt = Math.round(v);
            args = { schedule: store.schedule, stopAt: store.stopAt };
            break;
        }
        Changes.touch();
        if (!args) {
            done(true, "");
            return;
        }
        Changes.call("nightlight.set", args, function (ok, error) {
            if (!ok) {
                console.warn("nightlight.set failed:", error);
                store[key] = before;
                Changes.touch();
            }
            done(ok, ok ? "" : I18n.tr("Night light did not change. Run ryoku doctor in a terminal, then retry."));
        });
    }

    // shared bus frame, never mutate it
    function applyFrame(f) {
        if (!f || typeof f !== "object" || Array.isArray(f))
            return;
        store.on = f.on === true;
        if (typeof f.temperature === "number" && f.temperature > 0)
            store.temperature = f.temperature;
        if (f.schedule === "off" || f.schedule === "sun" || f.schedule === "clock")
            store.schedule = f.schedule;
        if (typeof f.marginMin === "number" && f.marginMin >= 0)
            store.marginMin = f.marginMin;
        if (typeof f.startAt === "number" && f.startAt >= 0 && f.startAt < 1440)
            store.startAt = f.startAt;
        if (typeof f.stopAt === "number" && f.stopAt >= 0 && f.stopAt < 1440)
            store.stopAt = f.stopAt;
        Changes.touch();
    }

    property QtObject feed: RU.BusTopic {
        topic: "nightlight"
        onUpdated: store.applyFrame(store.feed.data)
    }

    Component.onCompleted: Changes.registerStore("nightlight", store)
}
