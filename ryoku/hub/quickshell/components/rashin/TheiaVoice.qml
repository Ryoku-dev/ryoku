import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"

Column {
    id: root
    readonly property bool wide: true
    property var row: null
    property var value: null
    property var commit: null
    property var store: null
    property var request: null
    property string learnRequest: ""
    property int generation: 0
    property bool loaded: false
    property string savedSamples: ""
    property string savedNotes: ""
    property string error: ""
    property string operation: ""
    readonly property bool busy: root.request !== null
    readonly property bool editing: samples.text !== root.savedSamples || notes.text !== root.savedNotes
    readonly property bool live: root.visible && root.enabled && (!root.store || root.store.active !== false)
    width: LithicTokens.size.settingsControlMax
    spacing: LithicTokens.space.s2

    function adopt(profile) {
        root.savedSamples = String(profile.samples || "");
        root.savedNotes = String(profile.notes || "");
        samples.text = root.savedSamples;
        notes.text = root.savedNotes;
        root.loaded = true;
    }
    function call(method, params, done) {
        if (!root.live || root.busy)
            return;
        root.error = "";
        var learnId = "";
        if (method === "writing.voice.learn") {
            learnId = "theia-voice-" + Date.now().toString(36) + "-" + Math.random().toString(36).slice(2);
            params = Object.assign({}, params, { request: learnId });
            root.learnRequest = learnId;
        }
        var version = ++root.generation;
        root.request = Changes.run(["rashin", "call", method, JSON.stringify(params)], (ok, out, err) => {
            if (version !== root.generation || (learnId !== "" && learnId !== root.learnRequest))
                return;
            root.request = null;
            if (learnId !== "") {
                root.learnRequest = "";
                if (!ok)
                    root.cancelLearn(learnId, null);
            }
            if (!root.live)
                return;
            if (!ok) {
                root.error = I18n.tr("Your writing voice could not be updated") + (err ? ": " + String(err).trim() : "");
                root.operation = "";
                return;
            }
            var profile;
            try {
                profile = JSON.parse(out);
                if (!profile || typeof profile.samples !== "string" || typeof profile.notes !== "string")
                    throw new Error("profile");
            } catch (e) {
                root.error = I18n.tr("Rashin returned an unreadable writing voice");
                root.operation = "";
                return;
            }
            done(profile);
        }, { timeout: method === "writing.voice.learn" ? 120000 : 15000 });
    }
    function refresh() {
        if (!root.live || root.busy || root.editing)
            return;
        root.operation = "read";
        root.call("writing.voice.get", {}, profile => {
            root.adopt(profile);
            root.operation = "";
        });
    }
    function save(learn) {
        if (!root.loaded)
            return;
        root.operation = learn ? "learn" : "save";
        root.call("writing.voice.set", { samples: samples.text, notes: notes.text }, profile => {
            root.adopt(profile);
            if (!learn) {
                root.operation = "";
                return;
            }
            root.call("writing.voice.learn", {
                model: String(ShellSettings.get("sight.model", "")),
                local: ShellSettings.get("sight.localOnly", false) === true
            }, learned => {
                root.adopt(learned);
                root.operation = "";
            });
        });
    }
    function cancelLearn(id, pending) {
        Changes.run(["rashin", "call", "writing.cancel", JSON.stringify({ request: id })], () => {
            if (pending && pending.running)
                pending.running = false;
        }, { timeout: 5000 });
    }
    function stop() {
        ++root.generation;
        var pending = root.request;
        var learnId = root.learnRequest;
        root.request = null;
        root.learnRequest = "";
        root.operation = "";
        if (learnId !== "")
            root.cancelLearn(learnId, pending);
        else if (pending)
            pending.running = false;
    }
    onLiveChanged: {
        if (root.live)
            root.refresh();
        else
            root.stop();
    }
    Component.onCompleted: root.refresh()
    Component.onDestruction: root.stop()

    VoiceField {
        id: samples
        width: Math.min(parent.width, LithicTokens.size.settingsTextMeasureMax)
        title: I18n.tr("Writing samples")
        hint: I18n.tr("Paste a few things you wrote, like messages, notes or emails")
        enabled: root.loaded && !root.busy
    }
    Plaque {
        small: true
        kind: "line"
        glyph: "person_edit"
        text: I18n.tr("Learn my voice")
        enabled: root.loaded && !root.busy && samples.text.trim().length > 0
        onClicked: root.save(true)
    }
    Row {
        visible: root.busy && (root.operation !== "read" || !root.loaded)
        spacing: LithicTokens.space.s2
        Working { active: root.live && root.busy; anchors.verticalCenter: parent.verticalCenter }
        Label {
            text: root.operation === "learn" ? I18n.tr("Learning your voice")
                : root.operation === "read" ? I18n.tr("Reading your voice") : I18n.tr("Saving your voice")
            typeRole: "detail"
            inkRole: "fg3"
        }
    }
    VoiceField {
        id: notes
        width: Math.min(parent.width, LithicTokens.size.settingsTextMeasureMax)
        title: I18n.tr("Voice notes")
        hint: I18n.tr("What Rashin should keep about your writing")
        enabled: root.loaded && !root.busy
    }
    Flow {
        width: parent.width
        spacing: LithicTokens.space.s2
        Plaque {
            small: true
            kind: "line"
            glyph: "save"
            text: I18n.tr("Save voice")
            enabled: root.loaded && root.editing && !root.busy
            onClicked: root.save(false)
        }
        Plaque {
            small: true
            kind: "quiet"
            glyph: "delete"
            text: I18n.tr("Clear")
            enabled: root.loaded && !root.busy && (samples.text !== "" || notes.text !== "")
            onClicked: {
                root.operation = "save";
                root.call("writing.voice.set", { samples: "", notes: "" }, profile => {
                    root.adopt(profile);
                    root.operation = "";
                });
            }
        }
        Plaque {
            visible: root.error !== ""
            small: true
            kind: "quiet"
            glyph: "refresh"
            text: I18n.tr("Read voice again")
            enabled: !root.busy && !root.editing
            onClicked: root.refresh()
        }
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

    component VoiceField: Column {
        id: field
        property alias text: edit.text
        property string title: ""
        property string hint: ""
        spacing: LithicTokens.space.s1
        Label { text: field.title; typeRole: "label" }
        CutRect {
            width: parent.width
            height: 144
            cut: Tone.rowCut
            fill: Lithic.field
            strokeColor: edit.activeFocus ? Tone.accent : Lithic.edge
            strokeWidth: LithicTokens.size.hairline
            Behavior on strokeColor { ColorAnimation { duration: Tone.quick } }
            Flickable {
                id: scroll
                anchors.fill: parent
                anchors.margins: LithicTokens.space.s2
                contentWidth: width
                contentHeight: Math.max(height, edit.contentHeight)
                clip: true
                boundsBehavior: Flickable.StopAtBounds
                TextEdit {
                    id: edit
                    width: scroll.width
                    height: Math.max(scroll.height, contentHeight)
                    wrapMode: TextEdit.Wrap
                    color: Lithic.fg
                    selectionColor: Lithic.sel
                    selectedTextColor: Lithic.onSel
                    font.family: Lithic.type.body.family
                    font.pixelSize: Lithic.type.body.pixelSize
                    selectByMouse: true
                    activeFocusOnTab: true
                    Accessible.name: field.title
                    onCursorRectangleChanged: {
                        if (cursorRectangle.y < scroll.contentY)
                            scroll.contentY = cursorRectangle.y;
                        else if (cursorRectangle.y + cursorRectangle.height > scroll.contentY + scroll.height)
                            scroll.contentY = cursorRectangle.y + cursorRectangle.height - scroll.height;
                    }
                }
                Label {
                    width: parent.width
                    visible: edit.text.length === 0
                    text: field.hint
                    typeRole: "body"
                    inkRole: "fg3"
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                    Accessible.ignored: true
                }
            }
        }
    }
}
