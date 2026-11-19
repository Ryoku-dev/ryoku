import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"

Column {
    id: root
    readonly property bool topAligned: true

    required property var store
    property var row: null
    property var value: ""
    property var commit: null
    readonly property bool editing: field.edited
    readonly property var suggestions: root.store.modelOptions(field.edited ? field.text : "")
    width: LithicTokens.size.settingsControlMax
    spacing: LithicTokens.space.s2

    TextField {
        id: field
        property bool edited: false
        width: Math.min(parent.width, LithicTokens.size.settingsFieldMax)
        placeholderText: I18n.tr("Light model")
        Accessible.name: I18n.tr("Theia model")
        onTextEdited: field.edited = true
        onEditingFinished: {
            if (!field.edited)
                return;
            var typed = field.text.trim();
            if (typed !== String(root.value || "") && root.commit)
                root.commit(typed);
            field.edited = false;
        }
        Binding {
            target: field
            property: "text"
            value: String(root.value || "")
            when: !field.edited
        }
    }
    Picker {
        width: Math.min(parent.width, LithicTokens.size.settingsControlMax)
        visible: root.suggestions.length > 0
        options: root.suggestions.map(m => m.name || m.id)
        currentIndex: root.suggestions.findIndex(m => m.id === root.value)
        Accessible.name: I18n.tr("Suggested models")
        onActivated: index => {
            const model = root.suggestions[index].id;
            field.edited = false;
            if (root.commit)
                root.commit(model);
        }
    }
    Plaque {
        text: I18n.tr("List models")
        kind: "quiet"
        small: true
        glyph: "refresh"
        onClicked: root.store.loadModels()
    }
    Label {
        visible: root.store.modelIssue !== "" && String(root.value || "") !== ""
        width: parent.width
        text: root.store.modelIssue === "no-key"
            ? I18n.tr("%1 needs a key for %2. Add one in Rashin > Settings > Providers, or use the light model.").arg(root.store.modelIdentity.model).arg(root.store.modelIdentity.provider)
            : I18n.tr("%1 is not available now. Check Rashin > Settings > Providers, or use the light model.").arg(root.store.modelIdentity.model)
        typeRole: "detail"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
    Plaque {
        visible: root.store.modelIssue !== "" && String(root.value || "") !== ""
        text: I18n.tr("Use light model")
        kind: "quiet"
        small: true
        onClicked: {
            field.edited = false;
            if (root.commit) root.commit("");
        }
    }
    Label {
        visible: root.store.modelError !== ""
        width: parent.width
        text: root.store.modelError
        typeRole: "detail"
        ink: Lithic.jasper
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
}
