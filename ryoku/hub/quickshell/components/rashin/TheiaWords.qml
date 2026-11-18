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
    property var words: []
    property int generation: 0
    property bool loaded: false
    property string error: ""
    readonly property bool live: root.visible && root.enabled && (!root.store || root.store.active !== false)
    readonly property bool busy: root.request !== null
    readonly property bool editing: wordField.text.length > 0
    width: LithicTokens.size.settingsControlMax
    spacing: LithicTokens.space.s2

    function call(method, params, done) {
        if (!root.live || root.busy)
            return;
        root.error = "";
        var version = ++root.generation;
        root.request = Changes.run(["rashin", "call", method, JSON.stringify(params)], (ok, out, err) => {
            if (version !== root.generation)
                return;
            root.request = null;
            if (!root.live)
                return;
            if (!ok) {
                root.error = I18n.tr("Your words could not be updated") + (err ? ": " + String(err).trim() : "");
                return;
            }
            try {
                var result = JSON.parse(out);
                if (!result || !Array.isArray(result.words) || result.words.some(word => typeof word !== "string"))
                    throw new Error("words");
                root.words = result.words;
                root.loaded = true;
            } catch (e) {
                root.error = I18n.tr("Rashin returned an unreadable word list");
                return;
            }
            if (done)
                done();
        }, { timeout: 15000 });
    }
    function refresh() {
        if (!root.live || root.busy || (root.loaded && root.editing))
            return;
        root.call("writing.words", {}, null);
    }
    function add() {
        var word = wordField.text.trim();
        if (!root.loaded || !word)
            return;
        root.call("writing.words.add", { word: word }, () => {
            wordField.text = "";
        });
    }
    function remove(word) {
        if (!root.loaded)
            return;
        root.call("writing.words.remove", { word: word }, null);
    }
    function stop() {
        ++root.generation;
        var pending = root.request;
        root.request = null;
        if (pending)
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

    Flow {
        width: parent.width
        spacing: LithicTokens.space.s2
        readonly property bool inlineAdd: width >= LithicTokens.size.textFieldDefaultWidth
            + addWord.implicitWidth + spacing
        TextField {
            id: wordField
            width: Math.min(LithicTokens.size.settingsFieldMax,
                parent.inlineAdd ? parent.width - addWord.implicitWidth - parent.spacing : parent.width)
            placeholderText: I18n.tr("Add a word")
            Accessible.name: I18n.tr("Word to add to your dictionary")
            enabled: root.loaded && !root.busy
            onAccepted: root.add()
        }
        Item {
            width: addWord.implicitWidth
            height: wordField.height
            Plaque {
                id: addWord
                anchors.verticalCenter: parent.verticalCenter
                small: true
                kind: "line"
                glyph: "add"
                text: I18n.tr("Add word")
                enabled: root.loaded && !root.busy && wordField.text.trim().length > 0
                onClicked: root.add()
            }
        }
    }
    Row {
        visible: root.busy
        spacing: LithicTokens.space.s2
        Working { active: root.live && root.busy; anchors.verticalCenter: parent.verticalCenter }
        Label { text: I18n.tr("Updating your words"); typeRole: "detail"; inkRole: "fg3" }
    }
    Label {
        visible: root.loaded
        width: parent.width
        text: root.words.length === 1 ? I18n.tr("1 word") : I18n.tr("%1 words").arg(root.words.length)
        typeRole: "detail"
        inkRole: "fg3"
    }
    Label {
        visible: root.loaded && root.words.length === 0
        width: parent.width
        text: I18n.tr("Words you add will be accepted by the checker on this computer")
        typeRole: "detail"
        inkRole: "fg3"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
    Flow {
        width: parent.width
        spacing: LithicTokens.space.s2
        Repeater {
            model: root.words
            delegate: CutRect {
                id: wordRow
                required property string modelData
                width: Math.min(root.width, LithicTokens.size.settingsControlMax,
                    wordName.implicitWidth + removeButton.implicitWidth + LithicTokens.space.s2 * 2 + LithicTokens.space.s1)
                height: Math.max(40, wordName.implicitHeight + 2 * LithicTokens.space.s2)
                cut: Tone.rowCut
                fill: Lithic.field
                strokeColor: Lithic.edge
                strokeWidth: LithicTokens.size.hairline
                Label {
                    id: wordName
                    anchors.left: parent.left
                    anchors.leftMargin: LithicTokens.space.s2
                    anchors.right: removeButton.left
                    anchors.rightMargin: LithicTokens.space.s2
                    anchors.verticalCenter: parent.verticalCenter
                    text: wordRow.modelData
                    typeRole: "body"
                    elide: Text.ElideRight
                }
                IconPlaque {
                    id: removeButton
                    anchors.right: parent.right
                    anchors.rightMargin: LithicTokens.space.s1
                    anchors.verticalCenter: parent.verticalCenter
                    small: true
                    kind: "quiet"
                    glyph: "close"
                    accessibleName: I18n.tr("Remove %1 from your words").arg(wordRow.modelData)
                    enabled: !root.busy
                    onClicked: root.remove(wordRow.modelData)
                }
            }
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
    Plaque {
        visible: root.error !== ""
        small: true
        kind: "quiet"
        glyph: "refresh"
        text: I18n.tr("Read words again")
        enabled: !root.busy
        onClicked: root.call("writing.words", {}, null)
    }
}
