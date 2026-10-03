pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Dialogs
import shell.services
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../../stage/Singletons" as StageCfg
import "../../visualizer/Singletons" as VizCfg
import ".."

Item {
    id: root

    required property real s
    required property bool open
    required property real reveal
    required property bool tabActive
    property int index: 0
    signal requestClose()

    readonly property var backend: StageCfg.StageBackend
    readonly property var stage: StageCfg.Config
    readonly property var visualizer: VizCfg.Config
    readonly property string effect: root.backend.effect
    readonly property string wallpaper: root.backend.current
    readonly property int layerCount: root.backend.layerCountFor(root.wallpaper)
    readonly property real gap: Tokens.s3 * root.s
    readonly property real pad: Tokens.s4 * root.s
    property string pendingQuality: ""
    property string pickedCut: ""
    property bool clearArmed: false

    readonly property string shownQuality: root.pendingQuality !== "" ? root.pendingQuality : root.stage.quality
    readonly property var shownModel: root.backend.modelForQuality(root.shownQuality)
    readonly property bool shownInstalled: !!(root.shownModel && root.shownModel.installed === true)

    implicitHeight: shell.implicitHeight

    onOpenChanged: if (!root.open) {
        root.pendingQuality = "";
        root.pickedCut = "";
        root.clearArmed = false;
    }

    function chooseQuality(value): void {
        root.pendingQuality = value === root.stage.quality ? "" : value;
    }

    function applyQuality(): void {
        if (root.pendingQuality === "")
            return;
        root.stage.setQuality(root.pendingQuality);
        root.pendingQuality = "";
        root.backend.refresh();
    }

    function pathFromUrl(url): string {
        const text = String(url || "");
        return text.indexOf("file://") === 0 ? decodeURIComponent(text.slice(7)) : text;
    }

    function effectLabel(): string {
        if (root.effect === "parallax")
            return I18n.tr("Parallax scene");
        if (root.effect === "depth")
            return I18n.tr("Depth scene");
        return I18n.tr("Plain wallpaper");
    }

    component SettingRow: Rectangle {
        id: setting
        required property string glyph
        required property string label
        property string detail: ""
        default property alias control: controlSlot.data

        implicitHeight: 54 * root.s
        radius: Tokens.radius * root.s
        color: Tokens.tint5
        border.width: Tokens.border
        border.color: Tokens.line
        Row {
            anchors.left: parent.left
            anchors.right: controlSlot.left
            anchors.leftMargin: root.pad
            anchors.rightMargin: Tokens.s3 * root.s
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s2 * root.s
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: setting.glyph
                color: Tokens.inkDim
                font.family: "Material Symbols Rounded"
                font.pixelSize: 20 * root.s
            }
            Column {
                anchors.verticalCenter: parent.verticalCenter
                width: parent.width - 30 * root.s
                spacing: Tokens.s1 * root.s
                Text {
                    width: parent.width
                    text: setting.label
                    color: Tokens.ink
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall * root.s
                    font.weight: Font.DemiBold
                    elide: Text.ElideRight
                }
                Text {
                    visible: setting.detail.length > 0
                    width: parent.width
                    text: setting.detail
                    color: Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fTiny * root.s
                    elide: Text.ElideRight
                }
            }
        }
        Item {
            id: controlSlot
            anchors.right: parent.right
            anchors.rightMargin: root.pad
            anchors.verticalCenter: parent.verticalCenter
            implicitWidth: childrenRect.width
            implicitHeight: childrenRect.height
        }
    }

    component LayerRow: Rectangle {
        id: layerRow
        required property int layerIndex
        readonly property bool front: root.backend.layerFront(root.wallpaper, layerRow.layerIndex)

        implicitHeight: layerColumn.implicitHeight + Tokens.s3 * root.s * 2
        radius: Tokens.radius * root.s
        color: Tokens.tint5
        border.width: Tokens.border
        border.color: Tokens.line

        Column {
            id: layerColumn
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.margins: Tokens.s3 * root.s
            spacing: Tokens.s2 * root.s

            Item {
                width: parent.width
                implicitHeight: Math.max(layerName.implicitHeight, layerActions.implicitHeight)
                Text {
                    id: layerName
                    anchors.left: parent.left
                    anchors.right: layerActions.left
                    anchors.rightMargin: Tokens.s2 * root.s
                    anchors.verticalCenter: parent.verticalCenter
                    text: root.backend.layerLabel(root.wallpaper, layerRow.layerIndex)
                    color: Tokens.ink
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall * root.s
                    font.weight: Font.DemiBold
                    elide: Text.ElideRight
                }
                Row {
                    id: layerActions
                    anchors.right: parent.right
                    spacing: Tokens.s2 * root.s
                    Seg {
                        width: 140 * root.s
                        options: ["behind", "front"]
                        labels: ({ behind: I18n.tr("Behind"), front: I18n.tr("In front") })
                        current: layerRow.front ? "front" : "behind"
                        onChose: value => root.backend.setLayerFront(layerRow.layerIndex, value === "front")
                    }
                    Btn {
                        visible: layerRow.layerIndex > 0
                        text: I18n.tr("Remove")
                        compact: true
                        onAct: root.backend.removeLayer(layerRow.layerIndex)
                    }
                }
            }
            Row {
                visible: root.effect === "parallax"
                width: parent.width
                spacing: Tokens.s3 * root.s
                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("Drift")
                    color: Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fMicro * root.s
                }
                Slid {
                    anchors.verticalCenter: parent.verticalCenter
                    width: parent.width - driftValue.width - Tokens.s7 * root.s
                    value: root.backend.layerDepth(root.wallpaper, layerRow.layerIndex)
                    from: 0
                    to: 1
                    onModified: value => root.backend.setLayerDepth(layerRow.layerIndex, value)
                }
                Text {
                    id: driftValue
                    anchors.verticalCenter: parent.verticalCenter
                    text: root.backend.layerDepth(root.wallpaper, layerRow.layerIndex).toFixed(2)
                    color: Tokens.inkDim
                    font.family: Tokens.mono
                    font.pixelSize: Tokens.fMicro * root.s
                }
            }
        }
    }

    SidebarCardShell {
        id: shell
        width: root.width
        index: root.index
        open: root.open
        reveal: root.reveal
        tabActive: root.tabActive
        title: I18n.tr("Stage")
        glyph: "graphic_eq"
        eyebrow: root.effect === "off" && !root.visualizer.enabled ? I18n.tr("QUIET DESKTOP") : I18n.tr("LIVE CANVAS")

        Column {
            width: parent.width
            spacing: root.gap

            Rectangle {
                width: parent.width
                implicitHeight: 210 * root.s
                radius: Tokens.radius * root.s
                clip: true
                border.width: Tokens.border
                border.color: Qt.rgba(Tokens.sun.r, Tokens.sun.g, Tokens.sun.b, 0.42)
                gradient: Gradient {
                    orientation: Gradient.Vertical
                    GradientStop { position: 0; color: Qt.rgba(Tokens.sun.r, Tokens.sun.g, Tokens.sun.b, 0.20) }
                    GradientStop { position: 1; color: Tokens.paperLift }
                }

                Image {
                    id: wallPreview
                    anchors.fill: parent
                    source: root.wallpaper !== "" ? "file://" + root.wallpaper : ""
                    fillMode: Image.PreserveAspectCrop
                    asynchronous: true
                    opacity: status === Image.Ready ? 0.58 : 0
                }
                Rectangle {
                    anchors.fill: parent
                    gradient: Gradient {
                        orientation: Gradient.Vertical
                        GradientStop { position: 0; color: Qt.rgba(Tokens.paper.r, Tokens.paper.g, Tokens.paper.b, 0.08) }
                        GradientStop { position: 1; color: Qt.rgba(Tokens.paper.r, Tokens.paper.g, Tokens.paper.b, 0.88) }
                    }
                }
                Image {
                    visible: root.effect !== "off" && root.layerCount > 0
                    anchors.fill: parent
                    source: root.backend.layerUrlFor(root.wallpaper, 0)
                    fillMode: Image.PreserveAspectCrop
                    asynchronous: true
                    opacity: status === Image.Ready ? 0.8 : 0
                }
                Text {
                    visible: wallPreview.status !== Image.Ready
                    anchors.centerIn: parent
                    text: root.effect === "off" ? "wallpaper" : "layers"
                    color: Qt.rgba(Tokens.sun.r, Tokens.sun.g, Tokens.sun.b, 0.42)
                    font.family: "Material Symbols Rounded"
                    font.pixelSize: 76 * root.s
                }

                Column {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.bottom: parent.bottom
                    anchors.margins: root.pad
                    spacing: Tokens.s2 * root.s
                    Text {
                        width: parent.width
                        text: root.effectLabel()
                        color: Tokens.ink
                        font.family: Tokens.display
                        font.pixelSize: Tokens.fHero * root.s
                        font.weight: Font.Medium
                        elide: Text.ElideRight
                    }
                    Text {
                        width: parent.width
                        text: root.backend.busy ? I18n.tr("Building layers · %1%").arg(root.backend.percent)
                            : root.layerCount > 0 ? I18n.tr("%1 layers ready").arg(root.layerCount)
                            : I18n.tr("Turn a wallpaper into a layered scene")
                        color: Tokens.inkMuted
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall * root.s
                        elide: Text.ElideRight
                    }
                }
            }

            Seg {
                width: parent.width
                options: ["off", "depth", "parallax"]
                labels: ({ off: I18n.tr("Off"), depth: I18n.tr("Depth"), parallax: I18n.tr("Parallax") })
                current: root.effect
                onChose: value => root.backend.setEffect(value)
            }

            Rectangle {
                visible: root.backend.busy
                width: parent.width
                implicitHeight: busyRow.implicitHeight + root.pad * 2
                radius: Tokens.radius * root.s
                color: Qt.rgba(Tokens.sun.r, Tokens.sun.g, Tokens.sun.b, 0.11)
                border.width: Tokens.border
                border.color: Qt.rgba(Tokens.sun.r, Tokens.sun.g, Tokens.sun.b, 0.46)
                Row {
                    id: busyRow
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    anchors.margins: root.pad
                    spacing: Tokens.s3 * root.s
                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        text: "progress_activity"
                        color: Tokens.sun
                        font.family: "Material Symbols Rounded"
                        font.pixelSize: 22 * root.s
                        RotationAnimation on rotation {
                            running: root.backend.busy && !Tokens.reduceMotion
                            loops: Animation.Infinite
                            from: 0
                            to: 360
                            duration: Tokens.dur(900)
                        }
                    }
                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        width: parent.width - cancelCut.width - 42 * root.s
                        text: I18n.tr("Cutting · %1%").arg(root.backend.percent)
                        color: Tokens.ink
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall * root.s
                        font.weight: Font.DemiBold
                    }
                    Btn {
                        id: cancelCut
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("Stop")
                        compact: true
                        onAct: root.backend.cancel()
                    }
                }
            }

            Column {
                visible: root.effect !== "off"
                width: parent.width
                spacing: root.gap

                Text {
                    text: I18n.tr("Cut quality")
                    color: Tokens.ink
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fRow * root.s
                    font.weight: Font.DemiBold
                }
                Seg {
                    width: parent.width
                    options: ["draft", "standard", "fine"]
                    labels: ({ draft: I18n.tr("Draft"), standard: I18n.tr("Standard"), fine: I18n.tr("Fine") })
                    current: root.shownQuality
                    onChose: value => root.chooseQuality(value)
                }
                Rectangle {
                    visible: root.pendingQuality !== ""
                    width: parent.width
                    implicitHeight: qualityPrompt.implicitHeight + root.pad * 2
                    radius: Tokens.radius * root.s
                    color: Tokens.tint5
                    border.width: Tokens.border
                    border.color: Tokens.line
                    Row {
                        id: qualityPrompt
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        anchors.margins: root.pad
                        spacing: Tokens.s3 * root.s
                        Text {
                            anchors.verticalCenter: parent.verticalCenter
                            width: parent.width - qualityAction.width - Tokens.s3 * root.s
                            text: !root.shownModel ? I18n.tr("Checking available cut model…")
                                : !root.shownInstalled ? I18n.tr("This quality needs a %1 download").arg(root.shownModel.size || "")
                                : I18n.tr("Re-cut this wallpaper at the new quality")
                            color: Tokens.inkMuted
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall * root.s
                            wrapMode: Text.Wrap
                        }
                        Btn {
                            id: qualityAction
                            anchors.verticalCenter: parent.verticalCenter
                            text: root.backend.installing ? I18n.tr("Downloading…")
                                : !root.shownModel ? I18n.tr("Checking…")
                                : !root.shownInstalled ? I18n.tr("Download") : I18n.tr("Re-cut")
                            primary: true
                            armed: !root.backend.installing && root.shownModel !== null
                            onAct: {
                                if (root.shownInstalled)
                                    root.applyQuality();
                                else if (root.shownModel)
                                    root.backend.install(root.shownModel.id);
                            }
                        }
                    }
                }

                Text {
                    visible: root.layerCount > 0
                    text: I18n.tr("Layers")
                    color: Tokens.ink
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fRow * root.s
                    font.weight: Font.DemiBold
                }
                Repeater {
                    model: root.layerCount
                    delegate: LayerRow {
                        required property int index
                        width: parent.width
                        layerIndex: index
                    }
                }
                Flow {
                    width: parent.width
                    spacing: Tokens.s2 * root.s
                    Btn { text: I18n.tr("Cut a picture…"); onAct: cutDialog.open() }
                    Btn { text: I18n.tr("Add a PNG…"); onAct: pngDialog.open() }
                    Btn {
                        visible: root.layerCount > 0 && !root.clearArmed
                        text: I18n.tr("Clear cut-outs")
                        onAct: root.clearArmed = true
                    }
                }
                Rectangle {
                    visible: root.pickedCut !== ""
                    width: parent.width
                    implicitHeight: pickedRow.implicitHeight + root.pad * 2
                    radius: Tokens.radius * root.s
                    color: Tokens.tint5
                    border.width: Tokens.border
                    border.color: Tokens.line
                    Row {
                        id: pickedRow
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        anchors.margins: root.pad
                        spacing: Tokens.s2 * root.s
                        Text {
                            anchors.verticalCenter: parent.verticalCenter
                            width: parent.width - cutButton.width - Tokens.s2 * root.s
                            text: root.pathFromUrl(root.pickedCut).split("/").pop()
                            color: Tokens.inkMuted
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall * root.s
                            elide: Text.ElideMiddle
                        }
                        Btn {
                            id: cutButton
                            text: I18n.tr("Cut")
                            primary: true
                            onAct: {
                                root.backend.cutLayer(root.pathFromUrl(root.pickedCut));
                                root.pickedCut = "";
                            }
                        }
                    }
                }
                Rectangle {
                    visible: root.clearArmed
                    width: parent.width
                    implicitHeight: clearRow.implicitHeight + root.pad * 2
                    radius: Tokens.radius * root.s
                    color: Qt.rgba(Tokens.alert.r, Tokens.alert.g, Tokens.alert.b, 0.08)
                    border.width: Tokens.border
                    border.color: Qt.rgba(Tokens.alert.r, Tokens.alert.g, Tokens.alert.b, 0.42)
                    Row {
                        id: clearRow
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        anchors.margins: root.pad
                        spacing: Tokens.s2 * root.s
                        Text {
                            anchors.verticalCenter: parent.verticalCenter
                            width: parent.width - clearButtons.width - Tokens.s2 * root.s
                            text: I18n.tr("Remove every cut-out for this wallpaper?")
                            color: Tokens.ink
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall * root.s
                            wrapMode: Text.Wrap
                        }
                        Row {
                            id: clearButtons
                            spacing: Tokens.s2 * root.s
                            Btn { text: I18n.tr("Cancel"); compact: true; onAct: root.clearArmed = false }
                            Btn {
                                text: I18n.tr("Clear")
                                compact: true
                                primary: true
                                onAct: {
                                    root.backend.clear();
                                    root.clearArmed = false;
                                }
                            }
                        }
                    }
                }

                Text {
                    text: I18n.tr("Look")
                    color: Tokens.ink
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fRow * root.s
                    font.weight: Font.DemiBold
                }
                SettingRow {
                    width: parent.width
                    glyph: "blur_on"
                    label: I18n.tr("Edge softness")
                    detail: Math.round(root.stage.edge * 100) + "%"
                    Slid {
                        width: 104 * root.s
                        value: root.stage.edge
                        from: 0
                        to: 1
                        onModified: value => root.stage.setEdge(value)
                    }
                }
                SettingRow {
                    width: parent.width
                    glyph: "shadow"
                    label: I18n.tr("Shadow")
                    detail: Math.round(root.stage.shadow * 100) + "%"
                    Slid {
                        width: 104 * root.s
                        value: root.stage.shadow
                        from: 0
                        to: 1
                        onModified: value => root.stage.setShadow(value)
                    }
                }

                Text {
                    visible: root.effect === "parallax"
                    text: I18n.tr("Motion")
                    color: Tokens.ink
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fRow * root.s
                    font.weight: Font.DemiBold
                }
                SettingRow {
                    visible: root.effect === "parallax"
                    width: parent.width
                    glyph: "open_in_full"
                    label: I18n.tr("Amount")
                    Seg {
                        width: 194 * root.s
                        options: ["subtle", "normal", "strong"]
                        labels: ({ subtle: I18n.tr("Subtle"), normal: I18n.tr("Normal"), strong: I18n.tr("Strong") })
                        current: root.stage.amount
                        onChose: value => root.stage.setAmount(value)
                    }
                }
                SettingRow {
                    visible: root.effect === "parallax"
                    width: parent.width
                    glyph: "animation"
                    label: I18n.tr("Idle motion")
                    Seg {
                        options: ["none", "float", "breathe", "sway"]
                        width: 220 * root.s
                        labels: ({ none: I18n.tr("Still"), float: I18n.tr("Float"), breathe: I18n.tr("Breathe"), sway: I18n.tr("Sway") })
                        current: root.stage.idle
                        onChose: value => root.stage.setIdle(value)
                    }
                }
                SettingRow {
                    visible: root.effect === "parallax"
                    width: parent.width
                    glyph: "music_note"
                    label: I18n.tr("React to music")
                    detail: I18n.tr("Near layers pulse with the beat")
                    Sw { on: root.stage.music; onToggled: root.stage.setMusic(!root.stage.music) }
                }
                SettingRow {
                    visible: root.effect === "parallax"
                    width: parent.width
                    glyph: "mouse"
                    label: I18n.tr("Follow mouse")
                    detail: I18n.tr("Layers drift with the pointer")
                    Sw { on: root.stage.followMouse; onToggled: root.stage.setMouse(!root.stage.followMouse) }
                }
            }

            Rectangle { width: parent.width; height: Tokens.border; color: Tokens.lineSoft }

            Rectangle {
                width: parent.width
                implicitHeight: vizHero.implicitHeight + root.pad * 2
                radius: Tokens.radius * root.s
                color: root.visualizer.enabled ? Qt.rgba(Tokens.sun.r, Tokens.sun.g, Tokens.sun.b, 0.12) : Tokens.tint5
                border.width: Tokens.border
                border.color: root.visualizer.enabled ? Qt.rgba(Tokens.sun.r, Tokens.sun.g, Tokens.sun.b, 0.45) : Tokens.line
                Row {
                    id: vizHero
                    anchors.left: parent.left
                    anchors.right: vizSwitch.left
                    anchors.leftMargin: root.pad
                    anchors.rightMargin: Tokens.s3 * root.s
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Tokens.s3 * root.s
                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        text: "graphic_eq"
                        color: root.visualizer.enabled ? Tokens.sun : Tokens.inkDim
                        font.family: "Material Symbols Rounded"
                        font.pixelSize: 30 * root.s
                    }
                    Column {
                        anchors.verticalCenter: parent.verticalCenter
                        width: parent.width - 42 * root.s
                        spacing: Tokens.s1 * root.s
                        Text {
                            width: parent.width
                            text: I18n.tr("Visualizer")
                            color: Tokens.ink
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fValue * root.s
                            font.weight: Font.Medium
                        }
                        Text {
                            width: parent.width
                            text: root.visualizer.styleId + " · " + root.visualizer.fps + " fps"
                            color: Tokens.inkMuted
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fMicro * root.s
                            elide: Text.ElideRight
                        }
                    }
                }
                Sw {
                    id: vizSwitch
                    anchors.right: parent.right
                    anchors.rightMargin: root.pad
                    anchors.verticalCenter: parent.verticalCenter
                    on: root.visualizer.enabled
                    onToggled: root.visualizer.setEnabled(!root.visualizer.enabled)
                }
            }

            Column {
                visible: root.visualizer.enabled
                width: parent.width
                spacing: root.gap

                SettingRow {
                    width: parent.width
                    glyph: "style"
                    label: I18n.tr("Look")
                    detail: root.visualizer.styleId
                    Row {
                        spacing: Tokens.s2 * root.s
                        Btn { text: I18n.tr("Previous"); compact: true; onAct: root.visualizer.cycleStyle(-1) }
                        Btn { text: I18n.tr("Next"); compact: true; onAct: root.visualizer.cycleStyle(1) }
                    }
                }
                SettingRow {
                    width: parent.width
                    glyph: "speed"
                    label: I18n.tr("Frame rate")
                    Seg {
                        options: ["30", "45", "60"]
                        labels: ({ "30": "30", "45": "45", "60": "60" })
                        width: 164 * root.s
                        current: String(root.visualizer.fps)
                        onChose: value => root.visualizer.setFps(Number(value))
                    }
                }
                SettingRow {
                    width: parent.width
                    glyph: "auto_mode"
                    label: I18n.tr("Adaptive quality")
                    detail: I18n.tr("Lower load when the desktop is busy")
                    Sw { on: root.visualizer.adaptive; onToggled: root.visualizer.setAdaptive(!root.visualizer.adaptive) }
                }
                SettingRow {
                    width: parent.width
                    glyph: "equalizer"
                    label: I18n.tr("Gain")
                    detail: Math.round(root.visualizer.gain * 100) + "%"
                    Slid {
                        width: 104 * root.s
                        value: root.visualizer.gain
                        from: 0.5
                        to: 2
                        onModified: value => root.visualizer.setGain(value)
                    }
                }
                SettingRow {
                    width: parent.width
                    glyph: "waves"
                    label: I18n.tr("Smoothing")
                    detail: Math.round(root.visualizer.smoothing * 100) + "%"
                    Slid {
                        width: 104 * root.s
                        value: root.visualizer.smoothing
                        from: 0
                        to: 1
                        onModified: value => root.visualizer.setSmoothing(value)
                    }
                }
                SettingRow {
                    visible: root.effect !== "off"
                    width: parent.width
                    glyph: "layers"
                    label: I18n.tr("Depth")
                    Seg {
                        options: ["behind", "front"]
                        labels: ({ behind: I18n.tr("Behind"), front: I18n.tr("In front") })
                        current: root.stage.isFront("visualizer") ? "front" : "behind"
                        width: 140 * root.s
                        onChose: value => root.stage.setFront("visualizer", value === "front")
                    }
                }
            }
        }
    }

    FileDialog {
        id: cutDialog
        title: I18n.tr("Cut a layer from a picture")
        nameFilters: [I18n.tr("Pictures (*.png *.jpg *.jpeg *.webp *.bmp)"), I18n.tr("All files (*)")]
        onAccepted: root.pickedCut = String(selectedFile)
    }
    FileDialog {
        id: pngDialog
        title: I18n.tr("Add a PNG layer")
        nameFilters: [I18n.tr("PNG images (*.png)"), I18n.tr("All files (*)")]
        onAccepted: root.backend.addLayer(root.pathFromUrl(selectedFile))
    }
}
