import QtQuick
import QtQuick.Templates as T

T.TextField {
    id: root

    property string glyph: "search"
    property string shortcutHint: ""
    property string previewState: ""
    property string accessibleName: Lithic.tr("Search")

    readonly property bool _hoverActive: enabled && (hovered || previewState === "hover")
    readonly property bool _pressActive: enabled && previewState === "pressed"
    readonly property bool _focusActive: enabled && (activeFocus || previewState === "focus")

    hoverEnabled: true
    focusPolicy: Qt.StrongFocus
    implicitWidth: LithicTokens.size.searchWidth
    implicitHeight: LithicTokens.size.searchHeight
    leftPadding: LithicTokens.size.searchLeading
    rightPadding: text.length > 0
                  ? LithicTokens.size.searchClear + LithicTokens.size.searchClearInset * 2
                  : (shortcutHint.length > 0 ? hintLabel.implicitWidth + LithicTokens.size.searchIconInset : LithicTokens.size.textFieldInset)
    verticalAlignment: TextInput.AlignVCenter
    color: Lithic.fg
    placeholderTextColor: Lithic.fg3
    selectionColor: Lithic.sel
    selectedTextColor: Lithic.onSel
    font.family: Lithic.type.body.family
    font.pixelSize: Lithic.type.body.pixelSize
    font.weight: Lithic.type.body.weight
    opacity: enabled ? 1 : Lithic.disabledOpacity

    Accessible.role: Accessible.EditableText
    Accessible.name: accessibleName

    // Escape clears the search; an empty search lets it go on to close what holds it, as on the web
    Keys.onEscapePressed: function(event) {
        if (root.text.length === 0) {
            event.accepted = false;
            return;
        }
        root.clear();
        root.forceActiveFocus();
        event.accepted = true;
    }

    background: FieldSurface {
        hovered: root._hoverActive
        pressed: root._pressActive
        focused: root._focusActive
    }

    Glyph {
        x: LithicTokens.size.searchIconInset
        anchors.verticalCenter: parent.verticalCenter
        width: LithicTokens.size.searchIcon
        height: width
        name: root.glyph
        ink: Lithic.fg2
        size: LithicTokens.size.searchIcon
    }

    // the template draws no placeholder of its own
    Label {
        x: root.leftPadding
        width: Math.max(0, root.width - root.leftPadding - root.rightPadding)
        anchors.verticalCenter: parent.verticalCenter
        visible: root.text.length === 0 && root.preeditText.length === 0 && root.placeholderText.length > 0
        text: root.placeholderText
        typeRole: "body"
        ink: root.placeholderTextColor
        Accessible.ignored: true
    }

    Label {
        id: hintLabel
        anchors.right: parent.right
        anchors.rightMargin: LithicTokens.size.textFieldInset
        anchors.verticalCenter: parent.verticalCenter
        visible: root.text.length === 0 && root.shortcutHint.length > 0
        text: root.shortcutHint
        typeRole: "monoSm"
        inkRole: "fg3"
    }

    IconPlaque {
        anchors.right: parent.right
        anchors.rightMargin: LithicTokens.size.searchClearInset
        anchors.verticalCenter: parent.verticalCenter
        visible: root.text.length > 0
        small: true
        kind: "quiet"
        glyph: "close"
        accessibleName: Lithic.tr("Clear search")
        onClicked: {
            root.clear();
            root.forceActiveFocus();
        }
    }
}
