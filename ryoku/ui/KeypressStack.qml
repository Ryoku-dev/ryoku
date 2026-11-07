pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons
import "lib/keypress.js" as KeypressPolicy

Item {
    id: stack

    property string theme: "dark"
    property bool preview: false
    property bool motionEnabled: true
    property var previewChords: [
        ["Super", "Shift", "R"],
        ["Ctrl", "K"],
        ["Space"]
    ]
    property int holdMs: 1800
    property int serial: 0
    // Per-monitor UI scale, threaded from the hosting overlay (default 1 = no-op).
    property real us: 1

    readonly property bool dark: theme !== "light"
    readonly property int rowCount: rows.count


    implicitWidth: strip.implicitWidth
    implicitHeight: strip.implicitHeight

    function snapshot() {
        var out = [];
        for (var i = 0; i < rows.count; i++) {
            var row = rows.get(i);
            out.push({ uid: row.uid, keys: JSON.parse(row.keyJson), count: row.count, time: row.time, held: row.held });
        }
        return out;
    }

    function beginRemoval(uid) {
        for (var i = 0; i < rows.count; i++) {
            var row = rows.get(i);
            if (row.uid !== uid)
                continue;
            var delegate = activeRepeater.itemAt(i);
            ghosts.append({
                uid: row.uid,
                keyJson: row.keyJson,
                count: row.count,
                time: row.time,
                held: row.held,
                ghostParentX: delegate ? delegate.mapToItem(stack.parent, 0, 0).x : stack.x
            });
            rows.remove(i);
            return;
        }
    }

    function removeOldestActive() {
        if (rows.count > 0)
            beginRemoval(rows.get(0).uid);
    }

    function release(keys, timestamp) {
        var now = timestamp > 0 ? timestamp : Date.now();
        var after = KeypressPolicy.releaseChords(snapshot(), keys, now);
        for (var i = 0; i < rows.count; i++) {
            var row = rows.get(i);
            if (row.held && after[i] && !after[i].held) {
                rows.setProperty(i, "held", false);
                rows.setProperty(i, "time", now);
            }
        }
    }

    function push(keys, repeat, state, timestamp) {
        var now = timestamp > 0 ? timestamp : Date.now();
        if (state === "released") {
            release(keys, now);
            return;
        }
        var held = state === "pressed";
        var before = snapshot();
        var after = KeypressPolicy.addChord(before, keys, repeat, now);
        var last = rows.count - 1;
        var merged = after.length === before.length && before.length > 0 && last >= 0
            && KeypressPolicy.sameKeys(before[before.length - 1].keys, keys)
            && after[after.length - 1].count > before[before.length - 1].count;
        if (merged) {
            rows.setProperty(last, "count", after[after.length - 1].count);
            rows.setProperty(last, "time", now);
            rows.setProperty(last, "held", held);
            return;
        }
        if (before.length >= 3)
            removeOldestActive();
        rows.append({
            uid: String(++serial),
            keyJson: JSON.stringify(keys),
            count: 1,
            time: now,
            held: held
        });
    }

    function clear() {
        rows.clear();
        ghosts.clear();
    }

    function removeGhost(uid) {
        for (var i = 0; i < ghosts.count; i++) {
            if (ghosts.get(i).uid === uid) {
                ghosts.remove(i);
                return;
            }
        }
    }


    ListModel {
        id: rows
        dynamicRoles: true
    }

    ListModel {
        id: ghosts
        dynamicRoles: true
    }

    Row {
        id: strip
        spacing: Tokens.s3 * stack.us

        move: Transition {
            NumberAnimation {
                properties: "x"
                duration: stack.motionEnabled ? Tokens.swap : 0
                easing.type: Tokens.ease
            }
        }

        Repeater {
            id: activeRepeater
            model: rows

            delegate: Item {
                id: wrap

                required property string uid
                required property string keyJson
                required property int count
                required property real time
                required property bool held
                property bool entered: false

                width: chord.implicitWidth
                height: chord.implicitHeight
                opacity: entered ? 1 : 0

                KeyChord {
                    id: chord
                    us: stack.us
                    keys: JSON.parse(wrap.keyJson)
                    count: wrap.count
                    pulse: wrap.count
                    held: wrap.held
                    dark: stack.dark
                    motionEnabled: stack.motionEnabled
                }

                Behavior on opacity {
                    NumberAnimation {
                        duration: stack.motionEnabled ? Tokens.move : 0
                        easing.type: Tokens.ease
                    }
                }

                Component.onCompleted: entered = true

            }
        }

        Repeater {
            model: stack.preview && rows.count === 0 ? stack.previewChords : []

            delegate: KeyChord {
                required property var modelData
                us: stack.us
                keys: modelData
                dark: stack.dark
                motionEnabled: stack.motionEnabled
            }
        }
    }

    Repeater {
        model: ghosts

        delegate: Item {
            id: ghost

            required property string uid
            required property string keyJson
            required property int count
            required property real time
            required property bool held
            required property real ghostParentX
            property bool fading: false

            x: ghostParentX - stack.x
            y: 0
            z: 2
            width: ghostChord.implicitWidth
            height: ghostChord.implicitHeight
            opacity: fading ? 0 : 1

            KeyChord {
                id: ghostChord
                us: stack.us
                keys: JSON.parse(ghost.keyJson)
                count: ghost.count
                held: ghost.held
                dark: stack.dark
                motionEnabled: stack.motionEnabled
            }

            Behavior on opacity {
                NumberAnimation {
                    duration: stack.motionEnabled ? Tokens.swap : 0
                    easing.type: Tokens.ease
                }
            }

            Timer {
                interval: stack.motionEnabled ? Tokens.swap : 1
                running: ghost.fading
                onTriggered: stack.removeGhost(ghost.uid)
            }

            Component.onCompleted: fading = true
        }
    }

    Timer {
        interval: 80
        repeat: true
        running: rows.count > 0
        onTriggered: {
            var after = KeypressPolicy.expireChords(stack.snapshot(), Date.now(), stack.holdMs);
            var retained = {};
            for (var i = 0; i < after.length; i++)
                retained[after[i].uid] = true;
            for (var j = rows.count - 1; j >= 0; j--) {
                var row = rows.get(j);
                if (!retained[row.uid])
                    stack.beginRemoval(row.uid);
            }
        }
    }

}
