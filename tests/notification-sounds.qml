import QtQuick
import Quickshell
import Quickshell.Io
import shell.services as Shell
import stage.services as Stage
import stage.modules.common as StageCommon

// Runs the shell's notification server beside the Stage one, as the desktop
// does, against shims for paplay and ryoku-shell
// (tests/notification-sounds.sh), and reads back what each was asked to play.
ShellRoot {
    id: root

    property int phase: 0
    property int waited: 0
    property int seen: 0
    readonly property var sounds: StageCommon.Config.options.sounds
    // Read both services while the config loads: a NotificationServer created
    // after that misses Quickshell's reload hook and never goes live.
    readonly property int stageCount: Stage.Notifications.list.length
    readonly property int shellCount: Shell.Notifs.popups.length

    FileView {
        id: calls
        path: Quickshell.env("RYOKU_TEST_CALLS")
        printErrors: false
    }

    // The plays logged since the last check.
    function lines(): var {
        calls.reload();
        calls.waitForJob();
        const all = calls.text().split("\n").filter(line => line.length > 0);
        const fresh = all.slice(root.seen);
        root.seen = all.length;
        return fresh;
    }

    function inHistory(summary): bool {
        return Shell.Notifs.history.some(n => n.summary === summary);
    }

    function inPopups(summary): bool {
        return Shell.Notifs.popups.some(n => n.summary === summary);
    }

    function expect(cond, message): void {
        console.log((cond ? "PASS: " : "FAIL: ") + message);
    }

    function publish(urgency): void {
        Stage.Notifications.publishInternalNotification({
            appName: "Reminder",
            summary: "Stretch",
            urgency: urgency,
            sound: true
        });
    }

    Timer {
        interval: 300
        running: true
        repeat: true
        onTriggered: {
            root.waited++;
            if (root.waited > 60) {
                console.log("FAIL: timed out in phase " + root.phase);
                Qt.quit();
                return;
            }
            switch (root.phase) {
            case 0:
                if (!StageCommon.Config.ready)
                    return;
                Quickshell.execDetached(["notify-send", "-a", "Mail", "New mail", "From the test"]);
                root.phase = 1;
                return;
            case 1:
                if (!Stage.Notifications.list.some(n => n.summary === "New mail")
                        || !Shell.Notifs.popups.some(n => n.summary === "New mail"))
                    return;
                root.phase = 2;
                return;
            case 2: {
                // The Stage config may post its own notice on a first start, so
                // count the chimes against what the shell put up.
                const chimes = root.lines();
                const shown = Shell.Notifs.popups.length;
                root.expect(chimes.length === shown && chimes.every(line => line === "ryoku-shell sound notification"),
                    "each of the " + shown + " notifications chimes once, through the Hub cue: "
                    + JSON.stringify(chimes));
                root.publish("critical");
                root.publish("normal");
                root.phase = 3;
                return;
            }
            case 3: {
                const cues = root.lines();
                root.expect(JSON.stringify(cues) === JSON.stringify([
                        "ryoku-shell sound notification-critical",
                        "ryoku-shell sound notification"]),
                    "a Stage notification plays the Hub cue for its urgency: " + JSON.stringify(cues));
                root.sounds.notifications = false;
                root.publish("normal");
                root.sounds.notifications = true;
                root.sounds.enable = false;
                root.publish("normal");
                root.sounds.enable = true;
                root.sounds.neverPlayApps = ["Reminder"];
                root.publish("normal");
                root.sounds.neverPlayApps = [];
                root.phase = 4;
                return;
            }
            case 4: {
                const muted = root.lines();
                root.expect(muted.length === 0,
                    "the Stage switches still silence it: " + JSON.stringify(muted));
                root.lines();
                Shell.Notifs.dnd = true;
                Quickshell.execDetached(["notify-send", "-a", "Mail", "Quiet one", "Held back"]);
                Quickshell.execDetached(["notify-send", "-a", "Bluetooth", "-u", "critical", "Pair now", "Match the code"]);
                root.phase = 5;
                return;
            }
            case 5: {
                if (!root.inHistory("Quiet one") || !root.inHistory("Pair now"))
                    return;
                root.expect(!root.inPopups("Quiet one"), "Do Not Disturb holds back a normal notification");
                root.expect(root.inPopups("Pair now"), "a critical notification gets through Do Not Disturb");
                const cues = root.lines();
                root.expect(JSON.stringify(cues) === JSON.stringify(["ryoku-shell sound notification-critical"]),
                    "only the critical one chimes under Do Not Disturb: " + JSON.stringify(cues));
                Shell.Notifs.gameMode = true;
                Quickshell.execDetached(["notify-send", "-a", "Bluetooth", "-u", "critical", "Pair in game", "Held too"]);
                root.phase = 6;
                return;
            }
            case 6: {
                if (!root.inHistory("Pair in game"))
                    return;
                root.expect(!root.inPopups("Pair in game"), "Game Mode keeps even a critical notification off the screen");
                const cues = root.lines();
                root.expect(cues.length === 0, "and silent: " + JSON.stringify(cues));
                // Nomarchy applies its own Do Not Disturb rule, and the shell's cue
                // follows it.
                Shell.Notifs.gameMode = false;
                Shell.Config.barStyle = "nomarchy";
                Quickshell.execDetached(["notify-send", "-a", "Bluetooth", "-u", "critical", "Pair in Nomarchy", "Its own rule"]);
                root.phase = 7;
                return;
            }
            case 7: {
                if (!root.inHistory("Pair in Nomarchy"))
                    return;
                root.expect(!root.inPopups("Pair in Nomarchy"), "Nomarchy's own rule holds back a critical notification from another app");
                const cues = root.lines();
                root.expect(cues.length === 0, "and the shell's cue stays silent for it: " + JSON.stringify(cues));
                // What Nomarchy does draw under Do Not Disturb, the shell sounds for:
                // a critical CLI alert, and an Omarchy action toast.
                Quickshell.execDetached(["notify-send", "-u", "critical", "CLI alarm", "Nomarchy draws it"]);
                Quickshell.execDetached(["notify-send", "-a", "omarchy-action", "Theme changed", "Nomarchy draws it"]);
                Quickshell.execDetached(["notify-send", "Chatter", "Nomarchy holds it back"]);
                root.phase = 8;
                return;
            }
            case 8: {
                if (!root.inHistory("CLI alarm") || !root.inHistory("Theme changed") || !root.inHistory("Chatter"))
                    return;
                root.expect(root.inPopups("CLI alarm") && root.inPopups("Theme changed"),
                    "the notifications Nomarchy admits get through Do Not Disturb");
                root.expect(!root.inPopups("Chatter"), "a normal CLI notification is held back");
                const cues = root.lines().sort();
                root.expect(JSON.stringify(cues) === JSON.stringify([
                        "ryoku-shell sound notification",
                        "ryoku-shell sound notification-critical"]),
                    "and the shell sounds exactly for the two it admits: " + JSON.stringify(cues));
                Qt.quit();
                return;
            }
            }
        }
    }
}
