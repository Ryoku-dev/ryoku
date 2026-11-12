import QtQuick
import Ryoku.Ui.Singletons

Item {
    id: deviceCard

    required property var device
    property var controller
    property real reveal: 1

    readonly property bool online: device.online === true
    readonly property bool managed: device.managed === true
    readonly property bool following: device.following === true
        || (deviceCard.managed && device.source === "accent")
    readonly property bool painted: String(device.effect || "").length > 0
    readonly property var modes: Array.isArray(device.modes) ? device.modes : []
    readonly property var effects: Array.isArray(device.effects) ? device.effects : []
    readonly property var zones: Array.isArray(device.zones) ? device.zones : []
    readonly property var activeMode: {
        var wanted = String(device.mode || device.active || "")
        for (var i = 0; i < deviceCard.modes.length; ++i)
            if (String(deviceCard.modes[i].name) === wanted)
                return deviceCard.modes[i]
        return null
    }
    readonly property var activeEffect: {
        var wanted = String(device.effect || "")
        for (var i = 0; i < deviceCard.effects.length; ++i)
            if (String(deviceCard.effects[i].id) === wanted)
                return deviceCard.effects[i]
        return null
    }
    readonly property int colourSlots: deviceCard.painted ? 1
        : (!deviceCard.activeMode ? 0
           : (Number(deviceCard.activeMode.colors) > 0
              ? Math.min(4, Number(deviceCard.activeMode.colors))
              : (deviceCard.activeMode.perLed === true ? 1 : 0)))
    readonly property bool perZone: !deviceCard.painted && deviceCard.zones.length > 1
        && deviceCard.activeMode && deviceCard.activeMode.perLed === true
    readonly property bool hasBrightness: deviceCard.painted
        || (deviceCard.activeMode && deviceCard.activeMode.brightness === true)
    readonly property bool hasSpeed: deviceCard.painted
        ? (deviceCard.activeEffect && deviceCard.activeEffect.speed === true)
        : (deviceCard.activeMode && deviceCard.activeMode.speed === true)
    readonly property var directions: !deviceCard.painted && deviceCard.activeMode
        && Array.isArray(deviceCard.activeMode.directions) ? deviceCard.activeMode.directions : []
    readonly property bool canSave: !deviceCard.painted && deviceCard.activeMode
        && deviceCard.activeMode.canSave === true
    readonly property int brightness: Number(device.brightness) >= 0 ? Number(device.brightness)
        : (deviceCard.painted ? 100
           : (deviceCard.activeMode ? Number(deviceCard.activeMode.briPct) : 100))
    readonly property int speed: Number(device.speed) >= 0 ? Number(device.speed)
        : (deviceCard.painted ? 50
           : (deviceCard.activeMode ? Number(deviceCard.activeMode.speedPct) : 50))

    implicitHeight: card.implicitHeight

    function patch(values) { deviceCard.controller.setDevice(String(deviceCard.device.key), values) }
    function patchLater(values) { deviceCard.controller.setDeviceLater(String(deviceCard.device.key), values) }

    function modeChoices() {
        var result = []
        for (var i = 0; i < deviceCard.modes.length; ++i)
            result.push({ value: String(deviceCard.modes[i].name), label: I18n.tr(String(deviceCard.modes[i].name)) })
        return result
    }

    function effectChoices() {
        var result = []
        for (var i = 0; i < deviceCard.effects.length; ++i)
            result.push({ value: String(deviceCard.effects[i].id), label: I18n.tr(String(deviceCard.effects[i].label)) })
        return result
    }

    StackBar {
        id: card
        width: deviceCard.width
        expanded: true
        title: String(deviceCard.device.name || I18n.tr("Lighting device"))
            + (deviceCard.online ? "" : "  ·  " + I18n.tr("not connected"))

        Column {
            parent: card.body
            width: card.body.width
            spacing: 13 * Theme.scale
            topPadding: 4 * Theme.scale
            bottomPadding: 12 * Theme.scale

            Text {
                width: parent.width
                text: [deviceCard.device.type || "", deviceCard.device.vendor || "",
                       deviceCard.device.location || ""].filter(function(v) { return String(v).length > 0 }).join("  ·  ")
                visible: text.length > 0
                font.family: Theme.sans
                font.weight: Font.Medium
                font.pixelSize: Theme.fontFine
                color: Theme.withAlpha(Theme.surfaceText, 0.48 * deviceCard.reveal)
                elide: Text.ElideRight
                renderType: Text.NativeRendering
            }

            Item {
                width: parent.width
                height: Math.max(followText.implicitHeight, followSwitch.height)

                Column {
                    id: followText
                    anchors.left: parent.left
                    anchors.right: followSwitch.left
                    anchors.rightMargin: 12 * Theme.scale
                    spacing: 3 * Theme.scale
                    Text {
                        width: parent.width
                        text: I18n.tr("Follow wallpaper & theme")
                        font.family: Theme.sans
                        font.weight: Font.Medium
                        font.pixelSize: Theme.fontField
                        color: Theme.withAlpha(Theme.surfaceText, deviceCard.online ? deviceCard.reveal : 0.4)
                        wrapMode: Text.WordWrap
                        renderType: Text.NativeRendering
                    }
                    Text {
                        width: parent.width
                        text: deviceCard.following
                            ? I18n.tr("Your palette's live accent recolours this device whenever the wallpaper or fixed theme changes.")
                            : I18n.tr("One click adopts the device, picks a colour-capable mode and starts following the palette. Turning it off hands the device back.")
                        font.family: Theme.sans
                        font.weight: Font.Normal
                        font.pixelSize: Theme.fontBase
                        color: Theme.withAlpha(Theme.surfaceText, 0.55 * deviceCard.reveal)
                        lineHeight: 1.35
                        wrapMode: Text.WordWrap
                        renderType: Text.NativeRendering
                    }
                }
                Switch {
                    id: followSwitch
                    anchors.right: parent.right
                    anchors.top: parent.top
                    checked: deviceCard.following
                    enabled: deviceCard.online && !deviceCard.controller.busy
                    onToggled: (on) => deviceCard.controller.follow(String(deviceCard.device.key), on)
                }
            }

            FolioRule { width: parent.width; alpha: 0.3; reveal: deviceCard.reveal }

            Item {
                width: parent.width
                height: Math.max(controlText.implicitHeight, controlSwitch.height)

                Column {
                    id: controlText
                    anchors.left: parent.left
                    anchors.right: controlSwitch.left
                    anchors.rightMargin: 12 * Theme.scale
                    spacing: 3 * Theme.scale
                    Text {
                        width: parent.width
                        text: I18n.tr("Ryoku controls this device")
                        font.family: Theme.sans
                        font.weight: Font.Medium
                        font.pixelSize: Theme.fontField
                        color: Theme.withAlpha(Theme.surfaceText, deviceCard.online ? deviceCard.reveal : 0.4)
                        wrapMode: Text.WordWrap
                        renderType: Text.NativeRendering
                    }
                    Text {
                        width: parent.width
                        text: deviceCard.managed
                            ? I18n.tr("Handing it back restores %1.").arg(deviceCard.device.restore || I18n.tr("the effect Ryoku found"))
                            : I18n.tr("Off leaves its onboard profile, hardware switch or other lighting software in charge.")
                        font.family: Theme.sans
                        font.weight: Font.Normal
                        font.pixelSize: Theme.fontBase
                        color: Theme.withAlpha(Theme.surfaceText, 0.55 * deviceCard.reveal)
                        lineHeight: 1.35
                        wrapMode: Text.WordWrap
                        renderType: Text.NativeRendering
                    }
                }
                Switch {
                    id: controlSwitch
                    anchors.right: parent.right
                    anchors.top: parent.top
                    checked: deviceCard.managed
                    enabled: deviceCard.online && !deviceCard.controller.busy
                    onToggled: (on) => {
                        if (on)
                            deviceCard.patch({ managed: true })
                        else
                            deviceCard.controller.release(String(deviceCard.device.key))
                    }
                }
            }

            Column {
                width: parent.width
                visible: deviceCard.managed && deviceCard.effects.length > 0
                spacing: 6 * Theme.scale
                Text {
                    text: I18n.tr("Ryoku-painted effects")
                    font.family: Theme.sans
                    font.weight: Font.Medium
                    font.pixelSize: Theme.fontField
                    color: Theme.surfaceText
                    renderType: Text.NativeRendering
                }
                Text {
                    width: parent.width
                    text: I18n.tr("Drawn live by Ryoku in the device's per-key mode. These effects need Ryoku running.")
                    font.family: Theme.sans
                    font.pixelSize: Theme.fontBase
                    color: Theme.withAlpha(Theme.surfaceText, 0.55)
                    wrapMode: Text.WordWrap
                    renderType: Text.NativeRendering
                }
                ChoiceButtons {
                    width: parent.width
                    options: deviceCard.effectChoices()
                    value: String(deviceCard.device.effect || "")
                    enabled: deviceCard.online && !deviceCard.controller.busy
                    onSelected: (value) => deviceCard.patch({ effect: value })
                }
            }

            Column {
                width: parent.width
                visible: deviceCard.managed && deviceCard.modes.length > 0
                spacing: 6 * Theme.scale
                Text {
                    text: I18n.tr("Device effects")
                    font.family: Theme.sans
                    font.weight: Font.Medium
                    font.pixelSize: Theme.fontField
                    color: Theme.surfaceText
                    renderType: Text.NativeRendering
                }
                Text {
                    width: parent.width
                    text: I18n.tr("Run by the device itself, so they keep going when Ryoku closes.")
                    font.family: Theme.sans
                    font.pixelSize: Theme.fontBase
                    color: Theme.withAlpha(Theme.surfaceText, 0.55)
                    wrapMode: Text.WordWrap
                    renderType: Text.NativeRendering
                }
                ChoiceButtons {
                    width: parent.width
                    options: deviceCard.modeChoices()
                    value: deviceCard.painted ? "" : String(deviceCard.device.mode || deviceCard.device.active || "")
                    enabled: deviceCard.online && !deviceCard.controller.busy
                    onSelected: (value) => deviceCard.patch({ mode: value })
                }
            }

            Column {
                width: parent.width
                visible: deviceCard.managed && deviceCard.colourSlots > 0
                spacing: 6 * Theme.scale
                Text {
                    text: I18n.tr("Colour source")
                    font.family: Theme.sans
                    font.weight: Font.Medium
                    font.pixelSize: Theme.fontField
                    color: Theme.surfaceText
                    renderType: Text.NativeRendering
                }
                ChoiceButtons {
                    width: parent.width
                    options: [
                        { value: "accent", label: I18n.tr("Wallpaper & theme"),
                          strip: deviceCard.controller.report ? deviceCard.controller.report.accent : Theme.primary },
                        { value: "fixed", label: I18n.tr("Fixed colours"),
                          strip: (deviceCard.device.colors || [])[0] || Theme.surfaceVariant }
                    ]
                    value: String(deviceCard.device.source || "accent")
                    showStrip: true
                    enabled: deviceCard.online && !deviceCard.controller.busy
                    onSelected: (value) => deviceCard.patch({ source: value })
                }
            }

            Column {
                id: colourColumn
                width: parent.width
                visible: deviceCard.managed && deviceCard.device.source === "fixed"
                    && deviceCard.colourSlots > 0 && !deviceCard.perZone
                spacing: 9 * Theme.scale
                Text {
                    text: deviceCard.colourSlots > 1 ? I18n.tr("Colours") : I18n.tr("Fixed colour")
                    font.family: Theme.sans
                    font.weight: Font.Medium
                    font.pixelSize: Theme.fontField
                    color: Theme.surfaceText
                    renderType: Text.NativeRendering
                }
                Repeater {
                    model: deviceCard.colourSlots
                    delegate: LightingColorPicker {
                        required property int index
                        width: colourColumn.width
                        label: deviceCard.colourSlots > 1 ? I18n.tr("Colour %1").arg(index + 1) : I18n.tr("Choose")
                        value: (deviceCard.device.colors || [])[index]
                            || (deviceCard.controller.report ? deviceCard.controller.report.accent : "#ffffff")
                        onChosen: (hex) => {
                            var colours = Array.isArray(deviceCard.device.colors) ? deviceCard.device.colors.slice() : []
                            var accent = deviceCard.controller.report ? deviceCard.controller.report.accent : "#ffffff"
                            while (colours.length < deviceCard.colourSlots)
                                colours.push(accent)
                            colours[index] = hex
                            deviceCard.patch({ colors: colours })
                        }
                    }
                }
            }

            Column {
                id: zoneColumn
                width: parent.width
                visible: deviceCard.managed && deviceCard.device.source === "fixed" && deviceCard.perZone
                spacing: 10 * Theme.scale
                Text {
                    text: I18n.tr("Zone colours")
                    font.family: Theme.sans
                    font.weight: Font.Medium
                    font.pixelSize: Theme.fontField
                    color: Theme.surfaceText
                    renderType: Text.NativeRendering
                }
                Text {
                    width: parent.width
                    text: I18n.tr("Each part of this device can take its own colour.")
                    font.family: Theme.sans
                    font.pixelSize: Theme.fontBase
                    color: Theme.withAlpha(Theme.surfaceText, 0.55)
                    wrapMode: Text.WordWrap
                    renderType: Text.NativeRendering
                }
                Repeater {
                    model: deviceCard.zones
                    delegate: Column {
                        id: zone
                        required property var modelData
                        width: zoneColumn.width
                        spacing: 4 * Theme.scale
                        Text {
                            text: I18n.tr(String(zone.modelData.name))
                            font.family: Theme.sans
                            font.weight: Font.Medium
                            font.pixelSize: Theme.fontFine
                            color: Theme.withAlpha(Theme.surfaceText, 0.68)
                            renderType: Text.NativeRendering
                        }
                        LightingColorPicker {
                            width: zone.width
                            value: (deviceCard.device.zoneColors || ({}))[String(zone.modelData.name)]
                                || (deviceCard.device.colors || [])[0]
                                || (deviceCard.controller.report ? deviceCard.controller.report.accent : "#ffffff")
                            onChosen: (hex) => {
                                var zoneColours = Object.assign({}, deviceCard.device.zoneColors || ({}))
                                zoneColours[String(zone.modelData.name)] = hex
                                deviceCard.patch({ zoneColors: zoneColours })
                            }
                        }
                    }
                }
            }

            Column {
                width: parent.width
                visible: deviceCard.managed && deviceCard.hasBrightness
                spacing: 4 * Theme.scale
                Row {
                    width: parent.width
                    Text {
                        width: parent.width - brightnessValue.width
                        text: I18n.tr("Brightness")
                        font.family: Theme.sans
                        font.weight: Font.Medium
                        font.pixelSize: Theme.fontField
                        color: Theme.surfaceText
                        renderType: Text.NativeRendering
                    }
                    Text {
                        id: brightnessValue
                        text: I18n.tr("%1%").arg(deviceCard.brightness)
                        font.family: Theme.display
                        font.pixelSize: Theme.fontField
                        color: Theme.surfaceText
                        renderType: Text.NativeRendering
                    }
                }
                FolioSlider {
                    width: parent.width
                    from: 0
                    to: 100
                    step: 1
                    value: deviceCard.brightness
                    enabled: deviceCard.online && !deviceCard.controller.busy
                    onMoved: (value) => deviceCard.patchLater({ brightness: Math.round(value) })
                    onReleased: (value) => deviceCard.patchLater({ brightness: Math.round(value) })
                }
            }

            Column {
                width: parent.width
                visible: deviceCard.managed && deviceCard.hasSpeed
                spacing: 4 * Theme.scale
                Row {
                    width: parent.width
                    Text {
                        width: parent.width - speedValue.width
                        text: I18n.tr("Speed")
                        font.family: Theme.sans
                        font.weight: Font.Medium
                        font.pixelSize: Theme.fontField
                        color: Theme.surfaceText
                        renderType: Text.NativeRendering
                    }
                    Text {
                        id: speedValue
                        text: I18n.tr("%1%").arg(deviceCard.speed)
                        font.family: Theme.display
                        font.pixelSize: Theme.fontField
                        color: Theme.surfaceText
                        renderType: Text.NativeRendering
                    }
                }
                FolioSlider {
                    width: parent.width
                    from: 0
                    to: 100
                    step: 1
                    value: deviceCard.speed
                    enabled: deviceCard.online && !deviceCard.controller.busy
                    onMoved: (value) => deviceCard.patchLater({ speed: Math.round(value) })
                    onReleased: (value) => deviceCard.patchLater({ speed: Math.round(value) })
                }
            }

            Column {
                width: parent.width
                visible: deviceCard.managed && deviceCard.directions.length > 0
                spacing: 6 * Theme.scale
                Text {
                    text: I18n.tr("Direction")
                    font.family: Theme.sans
                    font.weight: Font.Medium
                    font.pixelSize: Theme.fontField
                    color: Theme.surfaceText
                    renderType: Text.NativeRendering
                }
                ChoiceButtons {
                    width: parent.width
                    options: {
                        var out = []
                        for (var i = 0; i < deviceCard.directions.length; ++i)
                            out.push({ value: String(deviceCard.directions[i]), label: I18n.tr(String(deviceCard.directions[i])) })
                        return out
                    }
                    value: String(deviceCard.device.direction || "")
                    enabled: deviceCard.online && !deviceCard.controller.busy
                    onSelected: (value) => deviceCard.patch({ direction: value })
                }
            }

            Flow {
                width: parent.width
                visible: deviceCard.managed
                spacing: 8 * Theme.scale

                FolioAction {
                    visible: deviceCard.canSave
                    label: I18n.tr("Save to device")
                    enabled: deviceCard.online && !deviceCard.controller.busy
                    confirm: true
                    onTriggered: deviceCard.controller.save(String(deviceCard.device.key))
                }
                FolioAction {
                    label: I18n.tr("Hand back")
                    enabled: deviceCard.online && !deviceCard.controller.busy
                    onTriggered: deviceCard.controller.release(String(deviceCard.device.key))
                }
            }

            Text {
                width: parent.width
                visible: deviceCard.managed && !deviceCard.online
                text: I18n.tr("This saved device is unplugged. Reconnect it to change its lighting or hand it back safely.")
                font.family: Theme.sans
                font.weight: Font.Medium
                font.pixelSize: Theme.fontBase
                color: Theme.withAlpha(Theme.tertiary, 0.82)
                wrapMode: Text.WordWrap
                renderType: Text.NativeRendering
            }

            Text {
                width: parent.width
                visible: deviceCard.managed && !deviceCard.painted && !deviceCard.canSave
                text: I18n.tr("This device cannot store the selected look in onboard memory. Ryoku reapplies it after login instead.")
                font.family: Theme.sans
                font.pixelSize: Theme.fontFine
                color: Theme.withAlpha(Theme.surfaceText, 0.48)
                wrapMode: Text.WordWrap
                renderType: Text.NativeRendering
            }
        }
    }
}
