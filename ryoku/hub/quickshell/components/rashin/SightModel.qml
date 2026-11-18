import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"

Column {
    id: root

    required property var store
    property var row: null
    property var value: ""
    property var commit: null
    width: LithicTokens.size.settingsControlMax
    spacing: LithicTokens.space.s2

    TextField {
        id: field
        property bool edited: false
        width: parent.width
        placeholderText: I18n.tr("Engine default")
        Accessible.name: I18n.tr("Sight model")
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
        width: parent.width
        visible: root.store.suggestions.length > 0
        options: root.store.suggestions.map(m => m.name || m.id)
        currentIndex: root.store.suggestions.findIndex(m => m.id === root.value)
        Accessible.name: I18n.tr("Suggested models")
        onActivated: index => {
            field.edited = false;
            if (root.commit)
                root.commit(root.store.suggestions[index].id);
        }
    }
    Plaque {
        visible: root.store.effectiveEngine === "providers"
        text: I18n.tr("List models")
        kind: "quiet"
        small: true
        glyph: "refresh"
        onClicked: root.store.loadModels()
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
