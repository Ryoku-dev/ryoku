pragma Singleton
import QtQuick
import Quickshell
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../utils/menupoll.js" as MenuPoll

// QML view of the daemon `powerprofiles` topic. The D-Bus provider lives behind
// ryoku-shell (powerprofiles.go), so QML never starts a second client: the bus
// streams {active_profile, profiles} on every change, and the
// `powerprofiles.setProfile` call sends the intent back. The list is service
// order with no client sort (contract 06 sec 2.9 / contract 11 sec 3.1).
Singleton {
    id: root

    // active profile name ("power-saver"/"balanced"/"performance"); the list in
    // service order; available when the daemon reports any profile.
    property string profile: ""
    property var profiles: []
    readonly property bool available: root.profiles.length > 0

    // active ownership is retained for the menu lifecycle (MenuPowerProfile
    // claims it on reveal); the topic streams regardless, so this now only
    // tracks owners for that contract.
    property bool active: false
    property var activeOwners: []
    function setActive(owner, enabled) {
        activeOwners = MenuPoll.setOwnership(activeOwners, owner, enabled);
        active = activeOwners.length > 0;
    }

    function setProfile(name) {
        if (typeof name !== "string" || !root.profiles.includes(name))
            return;
        root.call("powerprofiles.setProfile", { profile: name });
    }

    function apply(f) {
        if (!f || typeof f !== "object")
            return;
        root.profile = typeof f.active_profile === "string" ? f.active_profile : "";
        root.profiles = Array.isArray(f.profiles) ? f.profiles : [];
    }

    function call(method, args) {
        Bus.call(method, args);
    }

    BusTopic {
        id: topicFeed
        topic: "powerprofiles"
        onUpdated: root.apply(topicFeed.data)
    }

    Connections {
        target: Bus
        function onConnectedChanged() {
            if (!Bus.connected) {
                root.profile = "";
                root.profiles = [];
            }
        }
    }
}
