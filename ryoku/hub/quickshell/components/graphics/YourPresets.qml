import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"

// the tuning presets you saved: add keeps the current tuning under a name,
// edit renames, remove deletes after a Confirm (a deleted preset cannot come back)
Column {
    id: root

    required property var hub
    required property var store
    readonly property var saved: root.store ? root.store.presets.filter(p => p.builtin !== true) : []
    readonly property var taken: root.store ? root.store.presets.map(p => p.name) : []
    property string error: ""
    property var retryArgv: []

    spacing: LithicTokens.space.s1

    function run(argv) {
        root.error = "";
        root.store.run(argv, function (ok, out, err, code) {
            if (ok) {
                root.store.loadTune();
            } else {
                root.error = root.store.failure(err, I18n.tr("Your presets"), code);
                root.retryArgv = argv;
            }
        }, 10000);
    }

    function remove(index) {
        var name = root.saved[index].name;
        root.hub.confirm({
            title: I18n.tr("Remove %1?").arg(name),
            text: I18n.tr("The preset goes for good. The tuning in use now stays as it is."),
            actionText: I18n.tr("Remove")
        }, function () {
            root.run(["ryoku-hub", "gpu", "tune", "preset", "delete", name]);
        });
    }

    Label {
        width: parent.width
        topPadding: LithicTokens.space.s2
        text: I18n.tr("Your presets")
        typeRole: "row"
    }
    Label {
        width: parent.width
        bottomPadding: LithicTokens.space.s2
        text: I18n.tr("Adding one keeps the tuning in use now under a name, ready for Apply a preset.")
        typeRole: "detail"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }

    ListEditor {
        width: parent.width
        model: root.saved
        titleOf: item => item.name
        detailOf: item => I18n.tr("%1 settings").arg((item.entries || []).length)
        addText: I18n.tr("Save the current tuning")
        doneText: I18n.tr("Save")
        emptyText: I18n.tr("No presets of your own yet.")
        accessibleName: I18n.tr("Your presets")
        editor: nameEditor
        onAdded: item => root.run(["ryoku-hub", "gpu", "tune", "preset", "save", item.name])
        onEdited: (index, item) => {
            var from = root.saved[index].name;
            if (item.name !== from)
                root.run(["ryoku-hub", "gpu", "tune", "preset", "rename", from, item.name]);
        }
        onRemoved: index => root.remove(index)
    }

    Row {
        width: parent.width
        visible: root.error !== ""
        spacing: LithicTokens.space.s3
        Label {
            anchors.verticalCenter: parent.verticalCenter
            width: Math.min(implicitWidth, parent.width - retry.width - parent.spacing)
            text: root.error
            typeRole: "detail"
            ink: Lithic.jasper
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Plaque {
            id: retry
            anchors.verticalCenter: parent.verticalCenter
            kind: "quiet"
            small: true
            glyph: "refresh"
            text: I18n.tr("Retry")
            onClicked: root.run(root.retryArgv)
        }
    }

    Component {
        id: nameEditor
        Item {
            id: ed
            property var item
            readonly property string name: field.text.trim()
            readonly property bool clash: ed.name !== "" && ed.name !== String((ed.item || {}).name || "") && root.taken.indexOf(ed.name) >= 0
            readonly property bool valid: ed.name !== "" && !ed.clash
            function value() {
                return Object.assign({}, ed.item || {}, { name: ed.name });
            }
            implicitHeight: field.implicitHeight
            TextField {
                id: field
                width: Math.min(parent.width, LithicTokens.size.settingsControlMax)
                label: I18n.tr("Name")
                placeholderText: I18n.tr("Gaming, Quiet nights")
                text: String((ed.item || {}).name || "")
                errorText: ed.clash ? I18n.tr("A preset with this name already exists") : ""
            }
        }
    }
}
