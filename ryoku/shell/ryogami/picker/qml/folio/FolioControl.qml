import QtQuick
import Ryoku.Ui.Singletons
import "FolioConditions.js" as Cond

Item {
    id: fc

    property var control: ({})
    property var state
    property var options
    property var host
    // Omitted for unbound list-item editors.
    property var condCtx
    // Bumped on any settings/schema change so the gates re-evaluate.
    property int rev: 0
    property real reveal: 1

    property bool bound: true
    property string keyOverride: ""
    property var boundValue: undefined
    signal edited(var value)

    property var paramKeys: []
    property var staticValues: ({})
    property string staticText: ""
    property var actionArgs: ({})

    // A static row with control.status shows live daemon state: the RPC's answer
    // (or its error) goes through control.format into the row's %N values.
    Component.onCompleted: {
        if (!fc.control || !fc.control.status)
            return
        Daemon.call(fc.control.status, {}, function (result, err) {
            fc.staticValues = fc.control.format ? fc.control.format(result, err) : (result || ({}))
        })
    }

    readonly property string kind: fc.control && fc.control.kind ? fc.control.kind : ""
    readonly property bool _gated: fc.condCtx !== undefined && fc.condCtx !== null
    readonly property bool _visible: !fc._gated || (fc.rev, Cond.evaluate(fc.control.visibleWhen, fc.condCtx))
    readonly property bool _enabled: !fc._gated
        ? true
        : (fc.rev, Cond.evaluate(fc.control.enabledWhen === undefined ? "" : fc.control.enabledWhen, fc.condCtx))
    readonly property string _reason: fc.control.disabledReason ? fc.control.disabledReason : I18n.tr("Not available on this setup.")

    visible: fc._visible
    width: parent ? parent.width : implicitWidth
    implicitWidth: 200 * Theme.scale
    implicitHeight: fc._visible ? content.implicitHeight : 0
    height: implicitHeight

    Column {
        id: content
        width: fc.width
        spacing: 4 * Theme.scale

        Loader {
            id: loader
            width: parent.width
            active: fc._visible
            enabled: fc._enabled
            sourceComponent: fc._componentFor(fc.kind)
        }

        Text {
            width: parent.width
            visible: fc._visible && !fc._enabled && text.length > 0
            text: fc._reason
            font.family: Theme.sans
            font.weight: Font.Normal
            font.pixelSize: Theme.fontBase
            color: Theme.withAlpha(Theme.tertiary, 0.85 * fc.reveal)
            wrapMode: Text.WordWrap
            renderType: Text.NativeRendering
        }
    }

    function _componentFor(kind) {
        switch (kind) {
        case "toggle": return toggleC;
        case "number": return numberC;
        case "text": return textC;
        case "chips":
        case "dropdown": return choiceC;
        case "resolution": return resolutionC;
        case "keybind": return keybindC;
        case "action": return actionC;
        case "motion": return motionC;
        case "presets": return presetsC;
        case "static": return staticC;
        case "preview": return previewC;
        case "appTheme": return appThemeC;
        case "segment": return segmentC;
        case "lighting": return lightingC;
        }
        return null;
    }

    Component { id: toggleC
        ToggleRow { width: loader.width; control: fc.control; state: fc.state; options: fc.options
            reveal: fc.reveal; bound: fc.bound; keyOverride: fc.keyOverride; boundValue: fc.boundValue
            onEdited: (v) => fc.edited(v) } }
    Component { id: numberC
        NumberRow { width: loader.width; control: fc.control; state: fc.state; options: fc.options
            reveal: fc.reveal; bound: fc.bound; keyOverride: fc.keyOverride; boundValue: fc.boundValue
            onEdited: (v) => fc.edited(v) } }
    Component { id: textC
        TextRow { width: loader.width; control: fc.control; state: fc.state; options: fc.options
            reveal: fc.reveal; bound: fc.bound; keyOverride: fc.keyOverride; boundValue: fc.boundValue
            onEdited: (v) => fc.edited(v) } }
    Component { id: choiceC
        ChoiceRow { width: loader.width; control: fc.control; state: fc.state; options: fc.options
            reveal: fc.reveal; bound: fc.bound; keyOverride: fc.keyOverride; boundValue: fc.boundValue
            onEdited: (v) => fc.edited(v) } }
    Component { id: resolutionC
        ResolutionRow { width: loader.width; control: fc.control; state: fc.state; options: fc.options
            reveal: fc.reveal; bound: fc.bound; keyOverride: fc.keyOverride; boundValue: fc.boundValue
            onEdited: (v) => fc.edited(v) } }
    Component { id: keybindC
        KeybindRow { width: loader.width; control: fc.control; state: fc.state; options: fc.options
            host: fc.host; reveal: fc.reveal } }
    Component { id: actionC
        ActionRow { width: loader.width; control: fc.control; state: fc.state; options: fc.options
            host: fc.host; reveal: fc.reveal; actionArgs: fc.actionArgs } }
    Component { id: motionC
        MotionRow { width: loader.width; control: fc.control; state: fc.state; options: fc.options
            reveal: fc.reveal } }
    Component { id: presetsC
        PresetsRow { width: loader.width; control: fc.control; state: fc.state; options: fc.options
            reveal: fc.reveal; paramKeys: fc.paramKeys } }
    Component { id: staticC
        StaticRow { width: loader.width; control: fc.control; state: fc.state; reveal: fc.reveal
            values: fc.staticValues; valueText: fc.staticText } }
    Component { id: previewC
        PreviewRow { width: loader.width; control: fc.control; state: fc.state; options: fc.options
            reveal: fc.reveal } }
    Component { id: appThemeC
        AppThemeRow { width: loader.width; control: fc.control; state: fc.state; options: fc.options
            reveal: fc.reveal } }
    Component { id: segmentC
        Column {
            width: loader.width
            spacing: 6 * Theme.scale
            Segment { width: parent.width; label: fc.control.label ? fc.control.label : ""; reveal: fc.reveal }
            Text {
                width: parent.width
                visible: text.length > 0
                text: fc.control.help ? fc.control.help : ""
                font.family: Theme.sans
                font.weight: Font.Medium
                font.pixelSize: Theme.fontBase
                color: Theme.withAlpha(Theme.surfaceText, 0.56 * fc.reveal)
                lineHeight: 1.38
                wrapMode: Text.WordWrap
                renderType: Text.NativeRendering
            }
        } }
    Component { id: lightingC
        LightingRow { width: loader.width; control: fc.control; host: fc.host; reveal: fc.reveal } }
}
