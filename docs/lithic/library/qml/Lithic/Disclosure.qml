import QtQuick
import QtQuick.Templates as T

T.Control {
    id: root

    property bool open: false
    property string text: ""
    property string glyph: open ? "expand_less" : "expand_more"
    property string previewState: ""
    property string accessibleName: text
    default property alias disclosureContent: contentHost.data

    signal toggled(bool open)

    implicitWidth: trigger.implicitWidth
    implicitHeight: trigger.implicitHeight
                    + (contentHost.height > 0 ? LithicTokens.space.s2 + contentHost.height : 0)
    opacity: enabled ? 1 : Lithic.disabledOpacity

    Accessible.role: Accessible.Grouping
    Accessible.name: accessibleName
    Accessible.expanded: open

    contentItem: Item {
        implicitWidth: trigger.implicitWidth
        implicitHeight: root.implicitHeight

        Plaque {
            id: trigger
            kind: "quiet"
            text: root.text
            glyph: root.glyph
            previewState: root.previewState
            accessibleName: root.accessibleName
            enabled: root.enabled
            onClicked: {
                root.open = !root.open;
                root.toggled(root.open);
            }
        }

        Column {
            id: contentHost
            x: 0
            y: trigger.height + LithicTokens.space.s2
            width: root.width
            height: root.open ? implicitHeight : 0
            opacity: root.open ? 1 : 0
            clip: true
            enabled: root.open
            Accessible.ignored: !root.open

            ChiselNumberBehavior on height { duration: LithicTokens.motion.unfold }
            ChiselNumberBehavior on opacity { duration: LithicTokens.motion.unfold }
        }
    }
}
