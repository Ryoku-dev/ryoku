import QtQuick
import QtQuick.Templates as T
import Lithic
import Ryoku.Ui.Singletons

// a setting's value chosen from a long list: a field showing the choice that
// opens a searchable list; set as a row's control: "custom"
T.Control {
    id: root

    // filled in by the row
    property var row: null
    property var value
    property var commit: null

    // [{ value, label }]
    property var options: []
    // draw each choice in its own typeface
    property bool fontPreview: false
    property string dialogTitle: ""
    property string emptyText: I18n.tr("Nothing to choose from yet")
    property bool busy: false
    property Item overlayParent: null
    // why the choices could not be read; the field still opens, to say so and offer Retry
    property string failure: ""

    signal retry()

    readonly property string shown: {
        for (var i = 0; i < root.options.length; i++)
            if (root.options[i].value === root.value)
                return root.options[i].label;
        return root.value === undefined || root.value === null ? "" : String(root.value);
    }

    function activate() {
        if (!root.enabled)
            return;
        dialog.query = "";
        dialog.chosen = root.value;
        dialog.open();
    }

    function choose(v) {
        dialog.close();
        if (v !== root.value && typeof root.commit === "function")
            root.commit(v);
    }

    enabled: (root.options.length > 0 || root.failure !== "") && !root.busy
    implicitWidth: LithicTokens.size.settingsControlMax
    implicitHeight: LithicTokens.size.pickerHeight
    focusPolicy: Qt.StrongFocus
    hoverEnabled: true
    Accessible.role: Accessible.ComboBox
    Accessible.name: root.row ? I18n.tr(root.row.label) : ""
    Accessible.description: root.shown

    Keys.onSpacePressed: root.activate()
    Keys.onReturnPressed: root.activate()
    Keys.onEnterPressed: root.activate()

    // past a second of applying, say so
    property bool working: false
    onBusyChanged: if (!root.busy) root.working = false
    Timer {
        interval: 1000
        running: root.busy
        onTriggered: root.working = true
    }

    contentItem: Item {
        CutRect {
            anchors.fill: parent
            fill: root.hovered && root.enabled ? Lithic.mix(Lithic.field, Lithic.fg, LithicTokens.alpha.hoverPlane) : Lithic.field
            strokeColor: root.activeFocus ? Lithic.fg2 : Lithic.edge
            strokeWidth: LithicTokens.size.hairline
            cut: LithicTokens.cut.mid
            opacity: root.enabled || root.busy ? 1 : Lithic.disabledOpacity

            ChiselColorBehavior on fill {
                duration: root.hovered ? LithicTokens.motion.hoverIn : LithicTokens.motion.hoverOut
            }

            FocusRing {
                x: -outset
                y: -outset
                width: parent.width + outset * 2
                height: parent.height + outset * 2
                keyboardFocusVisible: root.visualFocus
                cut: LithicTokens.cut.mid
            }

            Label {
                anchors.left: parent.left
                anchors.leftMargin: LithicTokens.size.pickerInset
                anchors.right: mark.left
                anchors.rightMargin: LithicTokens.size.pickerArrowGap
                anchors.verticalCenter: parent.verticalCenter
                text: root.options.length === 0 && root.shown === "" ? root.emptyText : root.shown
                typeRole: "body"
                inkRole: root.shown === "" ? "fg3" : "fg"
                font.family: root.fontPreview && root.shown !== "" ? root.value : Lithic.type.body.family
            }

            Item {
                id: mark
                anchors.right: parent.right
                anchors.rightMargin: LithicTokens.size.pickerInset
                anchors.verticalCenter: parent.verticalCenter
                width: LithicTokens.size.pickerArrow
                height: LithicTokens.size.pickerArrow

                Working {
                    anchors.centerIn: parent
                    size: parent.width
                    active: root.working
                    visible: active
                }
                Glyph {
                    anchors.centerIn: parent
                    visible: !root.working
                    name: "unfold_more"
                    size: parent.width
                    ink: Lithic.fg2
                }
            }
        }
    }

    TapHandler {
        enabled: root.enabled
        onTapped: {
            root.forceActiveFocus();
            root.activate();
        }
    }

    ChoiceDialog {
        id: dialog
        parent: root.overlayParent ? root.overlayParent : root.parent
        opener: root
        title: root.dialogTitle
        options: root.options
        fontPreview: root.fontPreview
        failure: root.failure
        onPicked: v => root.choose(v)
        onRetry: root.retry()
    }
}
