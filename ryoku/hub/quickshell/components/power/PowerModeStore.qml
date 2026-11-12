import QtQuick
import Ryoku.Ui as RU
import Ryoku.Ui.Singletons
import "../../Singletons"

// the power mode as a Changes store: the daemon's powerprofiles topic is the
// truth and powerprofiles.setProfile the one writer, so its bank of the user's
// pick and game mode's stash stay intact
QtObject {
    id: store

    property string profile: ""
    property var profiles: []
    property bool heard: false
    // no frame within a few seconds: the desktop service is not answering
    property bool silent: false
    property int calls: 0
    readonly property bool unsettled: store.calls > 0

    function read(key) {
        if (key === "profile")
            return store.profile === "" ? undefined : store.profile;
        return undefined;
    }

    function write(key, v, done) {
        if (key !== "profile") {
            done(true, "");
            return;
        }
        var before = store.profile;
        store.profile = String(v);
        Changes.touch();
        store.calls++;
        Changes.call("powerprofiles.setProfile", { profile: store.profile }, function (ok, error) {
            store.calls--;
            Changes.touch();
            if (!ok) {
                store.profile = before;
                Changes.touch();
            }
            done(ok, ok ? "" : store.failure(error));
        });
    }

    // the daemon's own reason, said as the change that did not happen and what to do;
    // a call that never answered already says so in a whole sentence
    function failure(error) {
        var e = String(error || "").trim().replace(/[.\s]+$/, "");
        if (e === "")
            return I18n.tr("The power service did not take the change. Retry, and if it keeps failing, run ryoku doctor.");
        if (/did not answer/i.test(e))
            return String(error);
        return I18n.tr("The power service did not take the change: %1. Retry once that is fixed.").arg(e);
    }

    // shared bus frame, never mutate it
    function applyFrame(f) {
        if (!f || typeof f !== "object")
            return;
        store.heard = true;
        store.profile = String(f.active_profile || "");
        store.profiles = Array.isArray(f.profiles) ? f.profiles.slice() : [];
        Changes.touch();
    }

    property Timer wait: Timer {
        interval: 5000
        running: !store.heard
        onTriggered: {
            store.silent = true;
            Changes.touch();
        }
    }

    property QtObject feed: RU.BusTopic {
        topic: "powerprofiles"
        onUpdated: store.applyFrame(store.feed.data)
    }

    Component.onCompleted: Changes.registerStore("powermode", store)
}
