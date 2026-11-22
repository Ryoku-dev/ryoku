pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import shell.services
import "../ask/look"

Item {
    id: root

    required property real s
    required property var screen
    required property bool active
    // the panel's window can reach past its card, so the card bounds the settings popup
    property Item card: null
    property bool terminal: false
    property real bleed: 0
    signal session(string action)
    signal openExtensions()
    signal requestClose()

    readonly property var allActions: [
        { id: "lock", action: "lock", glyph: "lock", label: I18n.tr("Lock"), tip: I18n.tr("Hold to lock") },
        { id: "sleep", action: "suspend", glyph: "dark_mode", label: I18n.tr("Sleep"), tip: I18n.tr("Hold to sleep") },
        { id: "logout", action: "logout", glyph: "logout", label: I18n.tr("Log out"), tip: I18n.tr("Hold to log out") },
        { id: "restart", action: "reboot", glyph: "restart_alt", label: I18n.tr("Restart"), tip: I18n.tr("Hold to restart") },
        { id: "powerOff", action: "shutdown", glyph: "power_settings_new", label: I18n.tr("Power off"), tip: I18n.tr("Hold to power off") }
    ]
    readonly property var actions: root.allActions.filter(entry => SidebarState.elementVisible(entry.id))
    readonly property int actionCount: root.actions.length
    readonly property real rowHeight: Math.round(QsLook.holdHeight * root.s)
    readonly property real spacing: Math.round((QsLook.lithic ? 6 : QsLook.fluent ? 4 : 8) * root.s)
    readonly property bool gearShown: SidebarState.elementVisible("panelSettings")
    readonly property real separation: root.gearShown && root.actionCount > 0
        ? Math.round((QsLook.lithic ? 13 : QsLook.fluent ? 8 : 12) * root.s) : 0
    readonly property real minActionsWidth: root.actionCount > 0
        ? root.rowHeight * root.actionCount + root.spacing * (root.actionCount - 1) : 0
    readonly property real actionsWidth: root.actionCount > 0
        ? Math.max(root.minActionsWidth, root.width - root.separation - (root.gearShown ? gear.implicitWidth : 0)) : 0
    readonly property real cell: root.actionCount > 0
        ? (root.actionsWidth - root.spacing * (root.actionCount - 1)) / root.actionCount : 0
    // labels are all or nothing since a row mixing labelled and icon-only actions reads as five different controls
    readonly property bool labelled: root.actionCount > 0
        && Math.ceil(widest.advanceWidth) + Math.round(52 * root.s) <= root.cell

    implicitHeight: root.rowHeight
    // store designs size their panel from this
    implicitWidth: root.minActionsWidth + root.separation + (root.gearShown ? gear.implicitWidth : 0)

    TextMetrics {
        id: widest
        font.family: QsLook.sans
        font.pixelSize: Math.round(12 * root.s)
        font.weight: Font.Medium
        text: root.actions.map(entry => entry.label).reduce((a, b) => a.length >= b.length ? a : b, "")
    }

    Rectangle {
        readonly property real above: Math.round(QsLook.gap * root.s / 2)
        visible: QsLook.fluent && root.terminal
        x: -root.bleed
        y: -above
        width: root.width + root.bleed * 2
        height: root.height + above + root.bleed
        readonly property real corner: Look.cornerAt(QsLook.frameCut, width, height, root.s)
        bottomLeftRadius: corner
        bottomRightRadius: corner
        color: Look.footer

        Rectangle {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            height: Math.max(1, Math.round(root.s))
            color: Lithic.alpha(Lithic.vein, 0.7)
        }
    }

    Row {
        id: actionRow
        anchors.left: parent.left
        anchors.verticalCenter: parent.verticalCenter
        width: root.actionsWidth
        height: root.rowHeight
        spacing: root.spacing

        Repeater {
            model: root.actions

            delegate: Item {
                id: slot
                required property var modelData
                width: root.cell
                height: root.rowHeight

                HoldButton {
                    id: button
                    width: button.round ? button.implicitWidth : slot.width
                    anchors.centerIn: parent
                    s: root.s
                    glyph: slot.modelData.glyph
                    text: slot.modelData.label
                    tip: slot.modelData.tip
                    subtle: true
                    danger: slot.modelData.id === "powerOff"
                    labelled: root.labelled
                    onHeld: root.session(slot.modelData.action)
                }
            }
        }
    }

    Rectangle {
        visible: QsLook.lithic && root.separation > 0
        x: actionRow.width + Math.round((root.separation - width) / 2)
        anchors.verticalCenter: parent.verticalCenter
        width: Math.max(1, Math.round(root.s))
        height: Math.round(root.rowHeight * 0.6)
        color: Lithic.vein
    }

    CornerButton {
        id: gear
        visible: root.gearShown
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        s: root.s
        glyph: "settings"
        checked: settings.opened
        subtle: true
        turn: settings.opened ? 90 : 0
        Accessible.name: I18n.tr("Panel settings")
        onClicked: settings.opened ? settings.close() : settings.open()
    }

    ControlsSettingsPopup {
        id: settings
        readonly property Item bounds: root.card ?? settings.host
        readonly property real clearance: Math.round(8 * root.s)
        parent: root
        x: {
            if (!settings.bounds)
                return 0;
            const origin = root.mapToItem(settings.bounds, 0, 0);
            return Math.max(0, Math.min(settings.bounds.width - settings.width, origin.x + root.width - settings.width)) - origin.x;
        }
        y: {
            if (!settings.bounds)
                return 0;
            const origin = root.mapToItem(settings.bounds, 0, 0);
            const preferred = origin.y - settings.height - settings.clearance;
            return Math.max(0, Math.min(settings.bounds.height - settings.height, preferred)) - origin.y;
        }
        s: root.s
        active: root.active
        // measured on every open because the row settles after the popup exists
        onAboutToShow: settings.maxHeight = settings.bounds
            ? Math.max(1, root.mapToItem(settings.bounds, 0, 0).y - settings.clearance) : settings.hostHeight
        onOpenExtensions: root.openExtensions()
        onRequestClose: root.requestClose()
    }
}
