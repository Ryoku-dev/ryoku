import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import "rules.js" as Rules

// the editor for one shell layer rule: the layer's name, then what changes
Column {
    id: ed

    property var item: ({})
    property var spec: null
    property Item overlayParent: null

    property string layerName: ""
    property var values: ({})

    readonly property bool named: ed.layerName.trim() !== ""
    readonly property bool anything: Rules.layerAnySet({ values: ed.values, extras: ed.item.extras || [] })
    readonly property bool valid: ed.named && ed.anything

    function value() {
        return {
            namespace: ed.layerName.trim(), values: Changes.clone(ed.values),
            keep: Changes.clone(ed.item.keep || {}), extras: Changes.clone(ed.item.extras || [])
        };
    }

    function setValue(id, v) {
        var next = Object.assign({}, ed.values);
        if (v === undefined)
            delete next[id];
        else
            next[id] = v;
        ed.values = next;
    }

    Component.onCompleted: {
        ed.layerName = String(ed.item.namespace || "");
        ed.values = Changes.clone(ed.item.values || {});
        nsField.text = ed.layerName;
    }

    width: parent ? parent.width : 0
    spacing: LithicTokens.space.s3

    TextField {
        id: nsField
        width: Math.min(ed.width, LithicTokens.size.settingsControlMax)
        label: I18n.tr("Layer name")
        placeholderText: "launcher"
        helpText: I18n.tr("Such as launcher or ryoku-notifications")
        mono: true
        onTextEdited: ed.layerName = text
    }

    Column {
        width: ed.width
        spacing: 0

        Repeater {
            model: ed.spec ? ed.spec.props : []
            delegate: PropRow {
                required property var modelData
                required property int index
                prop: modelData
                first: index === 0
                value: ed.values[modelData.id]
                overlayParent: ed.overlayParent
                onChosen: v => ed.setValue(modelData.id, v)
            }
        }
    }

    Label {
        width: parent.width
        visible: !ed.anything
        text: I18n.tr("Choose at least one change.")
        typeRole: "detail"
        inkRole: "fg3"
    }
}
