import QtQuick
import Lithic
import Ryoku.Ui as RU
import Ryoku.Ui.Singletons
import "../../Singletons"
import "../installed-apps"

Column {
    id: root
    readonly property bool topAligned: true
    property var row: null
    property var value: ""
    property var commit: null
    property var store: null
    property var voices: []
    property string missing: ""
    property string error: ""
    property bool reading: false
    property bool testing: false
    property string testId: ""
    property int generation: 0
    readonly property bool live: root.visible && root.enabled && (!root.store || root.store.active !== false)
    readonly property bool busy: root.reading || root.testing || install.running
    readonly property bool editing: false
    readonly property var options: {
        var list = [{ id: "", name: I18n.tr("Default voice"), language: "" }].concat(root.voices);
        var selected = String(root.value || "");
        if (selected && !list.some(voice => voice.id === selected))
            list.push({ id: selected, name: I18n.tr("Current voice: %1").arg(selected), language: "" });
        return list;
    }
    width: LithicTokens.size.settingsControlMax
    spacing: LithicTokens.space.s2

    function refresh() {
        if (!root.live || root.reading || install.running)
            return;
        root.reading = true;
        root.error = "";
        var version = ++root.generation;
        Bus.call("sight.voices", {}, (ok, result, error) => {
            if (version !== root.generation)
                return;
            root.reading = false;
            if (!root.live)
                return;
            if (!ok || !result || result.voices === undefined) {
                root.error = I18n.tr("Voices could not be read") + (error ? ": " + String(error) : "");
                return;
            }
            root.voices = Array.from(result.voices);
            root.missing = String(result.missing || "");
        });
    }
    function test() {
        if (!root.live || root.testing || root.missing !== "")
            return;
        root.testing = true;
        root.error = "";
        root.testId = "theia-settings-" + Date.now();
        var version = root.generation;
        Bus.call("sight.speak", {
            id: root.testId,
            card: "settings",
            text: I18n.tr("This is the voice Theia will use to read aloud"),
            voice: String(root.value || ""),
            rate: Number(ShellSettings.get("sight.readaloud.rate", 1.0))
        }, (ok, result, error) => {
            if (version !== root.generation)
                return;
            root.testing = false;
            if (root.live && !ok)
                root.error = I18n.tr("The voice could not be played") + (error ? ": " + String(error) : "");
        });
    }
    function stopPreview() {
        var speaking = preview.data && preview.data.speaking;
        if (root.testing || (speaking && speaking.id === root.testId))
            Bus.call("sight.stopSpeaking", {});
        root.testing = false;
    }
    function suspend() {
        root.stopPreview();
        ++root.generation;
        root.reading = false;
    }
    onLiveChanged: {
        if (root.live)
            root.refresh();
        else
            root.suspend();
    }
    Component.onCompleted: root.refresh()
    Component.onDestruction: root.suspend()

    RU.BusTopic {
        id: preview
        topic: "sight"
        active: root.live && root.testId !== ""
    }
    RU.PackageJob {
        id: install
        onFinished: (target, action, ok) => {
            if (root.live && ok)
                root.refresh();
        }
    }
    Picker {
        width: Math.min(parent.width, LithicTokens.size.settingsControlMax)
        options: root.options.map(voice => voice.language ? I18n.tr("%1 (%2)").arg(voice.name).arg(voice.language) : voice.name)
        currentIndex: Math.max(0, root.options.findIndex(voice => voice.id === String(root.value || "")))
        enabled: !root.reading && !install.running && root.missing === ""
        Accessible.name: I18n.tr("Read aloud voice")
        onActivated: index => {
            root.stopPreview();
            if (root.commit && index >= 0 && index < root.options.length)
                root.commit(root.options[index].id);
        }
    }
    Flow {
        width: parent.width
        spacing: LithicTokens.space.s2
        Plaque {
            small: true
            kind: "line"
            glyph: "volume_up"
            text: I18n.tr("Test")
            busy: root.live && root.testing
            enabled: root.live && !root.busy && root.missing === ""
            onClicked: root.test()
        }
        Plaque {
            small: true
            kind: "quiet"
            glyph: "stop"
            text: I18n.tr("Stop")
            visible: root.testing || !!(preview.data && preview.data.speaking && preview.data.speaking.id === root.testId)
            onClicked: root.stopPreview()
        }
        Plaque {
            small: true
            kind: "quiet"
            glyph: "refresh"
            text: I18n.tr("Refresh voices")
            enabled: !root.busy
            onClicked: root.refresh()
        }
    }
    Plaque {
        visible: root.missing !== ""
        small: true
        kind: "line"
        glyph: "download"
        text: I18n.tr("Install read aloud voices")
        enabled: !install.running
        onClicked: {
            if (root.live)
                install.start("theia-voices", "install", I18n.tr("Read aloud voices"), ["pkexec", "ryoku-host", "pkg", "install", "espeak-ng"]);
        }
    }
    Label {
        visible: root.missing !== ""
        width: parent.width
        text: I18n.tr("Installs espeak-ng, your password is required")
        typeRole: "detail"
        inkRole: "fg3"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
    JobLine {
        visible: root.live
        job: install
        target: "theia-voices"
        doing: I18n.tr("Installing read aloud voices")
    }
    Label {
        visible: root.error !== ""
        width: parent.width
        text: root.error
        typeRole: "detail"
        ink: Lithic.jasper
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
}
