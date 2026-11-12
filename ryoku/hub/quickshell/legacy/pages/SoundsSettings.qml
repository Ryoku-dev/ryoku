pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../Singletons"

// General > Sounds: the desktop's own cues (notifications, power, battery,
// devices, screenshots, volume). One switch and one volume for all of them, then
// each cue can be played, given your own file, returned to its built-in clip or
// switched off. The shell daemon plays them and keeps the choices in the
// `sounds` key of shell.json.
Column {
    id: root

    spacing: Tokens.s5
    property string feedback: ""

    readonly property bool masterOn: {
        Settings.revision;
        return Settings.get("sounds.enabled") !== false;
    }
    readonly property int volume: {
        Settings.revision;
        var v = Settings.get("sounds.volume");
        return typeof v === "number" ? Math.max(0, Math.min(100, Math.round(v))) : 100;
    }
    // the slider's own value while it is dragged, written once it rests
    property int pendingVolume: -1

    function save(path, value) {
        root.feedback = "";
        Settings.patch(path, value, function(ok, error) {
            root.feedback = ok ? "" : I18n.tr(error);
        });
    }

    readonly property var groups: [
        { title: I18n.tr("NOTIFICATIONS"), kana: "通知", events: [
            { event: "notification", label: I18n.tr("New notification"), desc: I18n.tr("Plays when a notification arrives. Do Not Disturb keeps it quiet.") },
            { event: "notification-critical", label: I18n.tr("Urgent notification"), desc: I18n.tr("Plays for notifications an app marks as critical.") }
        ] },
        { title: I18n.tr("POWER"), kana: "電源", events: [
            { event: "power-plug", label: I18n.tr("Charger plugged in"), desc: I18n.tr("Plays when the power cable is connected.") },
            { event: "power-unplug", label: I18n.tr("Charger unplugged"), desc: I18n.tr("Plays when the power cable is removed.") },
            { event: "battery-low", label: I18n.tr("Battery critically low"), desc: I18n.tr("Repeats every minute while the battery is under 4% and discharging.") },
            { event: "battery-full", label: I18n.tr("Battery full"), desc: I18n.tr("Plays once when the battery finishes charging.") }
        ] },
        { title: I18n.tr("DEVICES & SYSTEM"), kana: "機器", events: [
            { event: "device-added", label: I18n.tr("USB device connected"), desc: I18n.tr("Plays when a USB device is plugged in.") },
            { event: "device-removed", label: I18n.tr("USB device removed"), desc: I18n.tr("Plays when a USB device is unplugged.") },
            { event: "shutter", label: I18n.tr("Screenshot"), desc: I18n.tr("The shutter when a capture is taken.") },
            { event: "volume-change", label: I18n.tr("Volume change"), desc: I18n.tr("A tick when the volume keys change the output level.") }
        ] }
    ]

    Timer {
        id: volumeSettle
        interval: 350
        onTriggered: {
            if (root.pendingVolume < 0)
                return;
            var v = root.pendingVolume;
            root.pendingVolume = -1;
            root.save("sounds.volume", v);
            Settings.send("sounds.preview", { event: "notification" }, function() {});
        }
    }

    SettingCard {
        width: parent.width
        title: I18n.tr("SOUNDS")
        kana: "音"

        SettingRow {
            width: parent.width
            label: I18n.tr("Desktop sounds")
            desc: root.feedback !== "" ? root.feedback
                : I18n.tr("Turn every sound below on or off at once. Apps and media are not affected.")
            controlWidth: 54
            enabled: Settings.ready
            Sw {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                on: root.masterOn
                onToggled: value => root.save("sounds.enabled", value)
            }
        }

        SettingRow {
            width: parent.width
            divider: true
            label: I18n.tr("Volume")
            desc: I18n.tr("How loud these sounds play, relative to the output volume: %1%.")
                .arg(root.pendingVolume >= 0 ? root.pendingVolume : root.volume)
            footH: Tokens.ctlH
            enabled: Settings.ready && root.masterOn
            Slid {
                anchors.fill: parent
                from: 0
                to: 100
                value: root.pendingVolume >= 0 ? root.pendingVolume : root.volume
                onModified: v => {
                    root.pendingVolume = Math.round(v);
                    volumeSettle.restart();
                }
            }
        }
    }

    Repeater {
        model: root.groups
        delegate: SettingCard {
            id: card
            required property var modelData
            width: root.width
            title: card.modelData.title
            kana: card.modelData.kana

            Repeater {
                model: card.modelData.events
                delegate: SoundEventRow {
                    required property var modelData
                    required property int index
                    width: card.width
                    divider: index > 0
                    event: modelData.event
                    label: modelData.label
                    desc: modelData.desc
                    masterOn: root.masterOn
                    onFailed: message => root.feedback = message
                }
            }
        }
    }
}
