import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import "buttons.js" as Buttons

// the extra buttons of each connected mouse and what they send
Column {
    id: root

    // the page: findButton(opener), setKeys(device, code, opener), editSequence(device, code, opener), bindOf(device, code), labelOf(device, code)
    required property var controller
    required property var store
    property string highlightId: ""

    readonly property var devices: root.store ? root.store.devices : []
    readonly property bool seen: root.store ? root.store.seen : false

    spacing: LithicTokens.space.s3

    Column {
        visible: !root.seen || root.devices.length === 0
        width: parent.width
        spacing: LithicTokens.space.s3

        Label {
            width: parent.width
            text: !root.seen
                ? I18n.tr("The desktop service is not answering, so mouse buttons cannot be set here. Run ryoku doctor in a terminal, then look again.")
                : I18n.tr("No mouse with extra buttons is connected. A laptop's touchpad is never listed here.")
            typeRole: "body"
            ink: !root.seen ? Lithic.jasper : Lithic.fg2
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Plaque {
            kind: "line"
            small: true
            glyph: "refresh"
            text: I18n.tr("Look again")
            onClicked: root.store.refresh()
        }
    }

    Slab {
        visible: root.seen && root.devices.length > 0
        width: parent.width

        Column {
            width: parent.width

            SettingRow {
                id: findRow
                objectName: "find-button"
                width: parent.width
                first: true
                activatable: true
                highlighted: root.highlightId === "find-button"
                title: I18n.tr("Find a button")
                detail: I18n.tr("Press a button on your mouse, then the keys it should send")
                onActivated: root.controller.findButton(findRow)

                Plaque {
                    id: findPlaque
                    kind: "line"
                    small: true
                    glyph: "ads_click"
                    text: I18n.tr("Find")
                    accessibleName: I18n.tr("Find a button")
                    onClicked: root.controller.findButton(findPlaque)
                }
            }
        }
    }

    Repeater {
        model: root.seen ? root.devices : []
        delegate: Column {
            id: deviceItem
            required property var modelData
            readonly property var codes: Buttons.buttonsOf(deviceItem.modelData)
            width: root.width
            spacing: LithicTokens.space.s2

            Item { width: 1; height: LithicTokens.space.s2 }

            Label {
                width: parent.width
                text: Buttons.deviceName(deviceItem.modelData)
                typeRole: "row"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Label {
                width: parent.width
                text: (deviceItem.modelData.model ? I18n.tr("A known model, so its buttons have names") : I18n.tr("Not a model Ryoku knows, so its buttons are numbered"))
                    + (deviceItem.modelData.bus ? ", " + I18n.tr("connected by %1").arg(String(deviceItem.modelData.bus).toUpperCase()) : "")
                typeRole: "detail"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }

            Slab {
                width: parent.width

                Column {
                    width: parent.width

                    SettingRow {
                        visible: deviceItem.codes.length === 0
                        width: parent.width
                        first: true
                        title: I18n.tr("No extra buttons")
                        detail: I18n.tr("Left, right and the wheel only, so there is nothing to set")
                    }

                    Repeater {
                        model: deviceItem.codes
                        delegate: Column {
                            id: buttonItem
                            required property var modelData
                            required property int index
                            readonly property string device: deviceItem.modelData.id
                            readonly property int code: Number(buttonItem.modelData)
                            readonly property var bind: root.controller.bindOf(buttonItem.device, buttonItem.code)
                            readonly property var target: Changes.value(buttonItem.bind)
                            readonly property string error: Changes.error(buttonItem.bind)
                            readonly property bool slow: root.store !== null && root.store.slowKeys[buttonItem.bind.key] === true
                            width: parent.width
                            spacing: LithicTokens.space.s1

                            SettingRow {
                                width: parent.width
                                first: buttonItem.index === 0
                                title: Buttons.buttonLabel(buttonItem.code, deviceItem.modelData, I18n.tr)
                                detail: buttonItem.slow ? I18n.tr("Working…") : Buttons.bindingText(buttonItem.target, buttonItem.code, I18n.tr)
                                changed: !!buttonItem.target
                                onResetRequested: Changes.set(buttonItem.bind, undefined, root.controller.labelOf(buttonItem.device, buttonItem.code), "mouse-touchpad")

                                Plaque {
                                    id: keysPlaque
                                    kind: "quiet"
                                    small: true
                                    glyph: "keyboard"
                                    text: I18n.tr("Keys")
                                    accessibleName: I18n.tr("Set the keys %1 sends").arg(Buttons.buttonLabel(buttonItem.code, deviceItem.modelData, I18n.tr))
                                    onClicked: root.controller.setKeys(buttonItem.device, buttonItem.code, keysPlaque)
                                }
                                Plaque {
                                    id: sequencePlaque
                                    kind: "quiet"
                                    small: true
                                    glyph: "format_list_numbered"
                                    text: I18n.tr("Sequence")
                                    accessibleName: I18n.tr("Set a sequence for %1").arg(Buttons.buttonLabel(buttonItem.code, deviceItem.modelData, I18n.tr))
                                    onClicked: root.controller.editSequence(buttonItem.device, buttonItem.code, sequencePlaque)
                                }
                            }

                            Row {
                                visible: buttonItem.error.length > 0
                                width: parent.width
                                spacing: LithicTokens.space.s3

                                Label {
                                    anchors.verticalCenter: parent.verticalCenter
                                    width: Math.min(implicitWidth, parent.width - retry.width - LithicTokens.space.s3)
                                    text: I18n.tr(buttonItem.error)
                                    typeRole: "detail"
                                    ink: Lithic.jasper
                                    wrapMode: Text.Wrap
                                    elide: Text.ElideNone
                                }
                                Plaque {
                                    id: retry
                                    kind: "quiet"
                                    small: true
                                    glyph: "refresh"
                                    text: I18n.tr("Retry")
                                    onClicked: Changes.retry(buttonItem.bind)
                                }
                            }
                        }
                    }
                }
            }
        }
    }

    Label {
        visible: root.seen && root.devices.length > 0
        width: parent.width
        topPadding: LithicTokens.space.s1
        text: I18n.tr("Settings follow the physical button and stay with each mouse. They work in every app, under any window manager and on the lock screen, and Left-handed buttons does not move them.")
        typeRole: "detail"
        inkRole: "fg3"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
}
