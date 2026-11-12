import QtQuick
import QtQuick.Templates as T

T.TextField {
    id: root

    property string label: ""
    property string helpText: ""
    property string errorText: ""
    property bool secret: false
    property bool revealed: false
    property bool mono: false
    property string previewState: ""

    readonly property string _supportText: errorText.length > 0 ? errorText : helpText
    readonly property real _labelHeight: label.length > 0 ? fieldLabel.implicitHeight + LithicTokens.size.textFieldLabelGap : 0
    readonly property real _helpHeight: _supportText.length > 0 ? LithicTokens.size.textFieldHelpGap + helpLabel.implicitHeight : 0
    readonly property bool _hoverActive: enabled && (hovered || previewState === "hover")
    readonly property bool _pressActive: enabled && previewState === "pressed"
    readonly property bool _focusActive: enabled && (activeFocus || previewState === "focus")

    hoverEnabled: true
    focusPolicy: Qt.StrongFocus
    implicitWidth: LithicTokens.size.textFieldDefaultWidth
    implicitHeight: _labelHeight + LithicTokens.size.textFieldHeight + _helpHeight
    leftPadding: LithicTokens.size.textFieldInset
    rightPadding: secret
                  ? LithicTokens.size.textFieldSecretAction + LithicTokens.size.textFieldSecretInset * 2
                  : LithicTokens.size.textFieldInset
    topPadding: _labelHeight
    bottomPadding: _helpHeight
    verticalAlignment: TextInput.AlignVCenter
    color: Lithic.fg
    placeholderTextColor: Lithic.fg3
    selectionColor: Lithic.sel
    selectedTextColor: Lithic.onSel
    font.family: mono ? Lithic.type.mono.family : Lithic.type.body.family
    font.pixelSize: mono ? Lithic.type.mono.pixelSize : Lithic.type.body.pixelSize
    font.weight: mono ? Lithic.type.mono.weight : Lithic.type.body.weight
    echoMode: secret && !revealed ? TextInput.Password : TextInput.Normal
    passwordCharacter: "\u2022"
    opacity: enabled ? 1 : Lithic.disabledOpacity

    Accessible.role: Accessible.EditableText
    Accessible.name: label.length > 0 ? label : placeholderText
    Accessible.description: _supportText

    background: Item {
        FieldSurface {
            y: root._labelHeight
            width: parent.width
            height: LithicTokens.size.textFieldHeight
            hovered: root._hoverActive
            pressed: root._pressActive
            focused: root._focusActive
            invalid: root.errorText.length > 0
        }
    }

    // the template draws no placeholder of its own
    Label {
        x: root.leftPadding
        y: root._labelHeight + (LithicTokens.size.textFieldHeight - height) / 2
        width: Math.max(0, root.width - root.leftPadding - root.rightPadding)
        visible: root.text.length === 0 && root.preeditText.length === 0 && root.placeholderText.length > 0
        text: root.placeholderText
        // a hint is words, even in a field that holds literal data
        typeRole: "body"
        ink: root.placeholderTextColor
        Accessible.ignored: true
    }

    Label {
        id: fieldLabel
        visible: root.label.length > 0
        x: 0
        y: 0
        text: root.label
        typeRole: "detail"
    }

    Label {
        id: helpLabel
        visible: root._supportText.length > 0
        x: 0
        y: root._labelHeight + LithicTokens.size.textFieldHeight + LithicTokens.size.textFieldHelpGap
        width: root.width
        text: root._supportText
        typeRole: "caption"
        ink: root.errorText.length > 0 ? Lithic.jasper : Lithic.fg3
    }

    IconPlaque {
        anchors.right: parent.right
        anchors.rightMargin: LithicTokens.size.textFieldSecretInset
        y: root._labelHeight + LithicTokens.size.textFieldSecretInset
        visible: root.secret
        small: true
        kind: "quiet"
        glyph: root.revealed ? "visibility_off" : "visibility"
        accessibleName: root.revealed ? Lithic.tr("Hide value") : Lithic.tr("Show value")
        onClicked: {
            root.revealed = !root.revealed;
            root.forceActiveFocus();
        }
    }
}
