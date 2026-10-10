pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Dialogs
import Quickshell
import Ryoku.Ui
import Ryoku.Ui.Singletons
import stage.modules.common as StageIsland
import "Singletons" as StageCfg

Column {
    id: opts

    property string section: "Scene"
    property string pickedCut: ""
    property bool clearArmed: false
    property bool modelPickerOpen: false
    property string pendingModelRecut: ""

    width: parent ? parent.width : 0
    spacing: Tokens.s4

    readonly property var backend: StageCfg.StageBackend
    readonly property var cfg: StageCfg.Config
    readonly property var provider: StageIsland.Config.widgetProvider
    readonly property string wall: opts.provider && opts.provider.wallpaperPath
        ? opts.provider.wallpaperPath : opts.backend.current
    readonly property string effect: opts.backend.effectFor(opts.wall)
    readonly property bool parallax: opts.effect === "parallax"
    readonly property int layerCount: opts.backend.layerCountFor(opts.wall)
    readonly property var qualityModel: opts.backend.modelForQuality(opts.cfg.quality, opts.cfg.models)
    readonly property bool qualityReady: opts.backend.qualityInstalled(opts.cfg.quality, opts.cfg.models)
    readonly property string stateDir: Quickshell.env("XDG_STATE_HOME")
        || (Quickshell.env("HOME") + "/.local/state")
    readonly property string previewPath: opts.isVideo(opts.wall)
        ? opts.stateDir + "/ryoku-live-frame.png" : opts.wall
    readonly property var visibleRows: opts.provider
        ? opts.provider.rows.filter(row => row.enabled) : []
    readonly property var motionPresets: ({
        soft: { amount: "subtle", idle: "float", speed: 0.6, music: false },
        cinematic: { amount: "normal", idle: "sway", speed: 0.5, music: false },
        beat: { amount: "strong", idle: "none", speed: 1.0, music: true }
    })

    function isVideo(path) {
        return /\.(mp4|webm|mkv|mov)$/i.test(String(path || ""));
    }

    function pathFromUrl(url) {
        const text = String(url || "");
        return text.indexOf("file://") === 0 ? decodeURIComponent(text.slice(7)) : text;
    }

    function fileUrl(path) {
        const text = String(path || "");
        return text === "" || text.indexOf("file://") === 0 ? text : "file://" + text;
    }

    function leaf(path) {
        const parts = opts.pathFromUrl(path).split("/");
        return parts.length > 0 ? parts[parts.length - 1] : "";
    }

    function pct(value) {
        return Math.round(value * 100) + "%";
    }

    function effectLabel() {
        if (opts.effect === "parallax")
            return I18n.tr("Parallax scene");
        if (opts.effect === "depth")
            return I18n.tr("Depth scene");
        return I18n.tr("Plain wallpaper");
    }

    function sceneState() {
        if (opts.backend.busy)
            return I18n.tr("Building layers · %1%").arg(opts.backend.percent);
        if (opts.backend.notice !== "" && opts.effect !== "off")
            return I18n.tr("Cut blocked · %1").arg(opts.backend.notice);
        if (opts.layerCount === 1)
            return I18n.tr("1 layer ready");
        if (opts.layerCount > 1)
            return I18n.tr("%1 layers ready").arg(opts.layerCount);
        if (opts.effect === "off")
            return I18n.tr("The wallpaper is drawn flat behind the desktop.");
        return I18n.tr("Choose a cut quality, then build the scene.");
    }

    function motionPreset() {
        for (const name in opts.motionPresets) {
            const preset = opts.motionPresets[name];
            if (opts.cfg.amount === preset.amount
                    && opts.cfg.idle === preset.idle
                    && Math.abs(opts.cfg.speed - preset.speed) < 0.001
                    && opts.cfg.music === preset.music)
                return name;
        }
        return "";
    }

    function applyMotionPreset(name) {
        const preset = opts.motionPresets[name];
        if (!preset)
            return;
        opts.cfg.setAmount(preset.amount);
        opts.cfg.setIdle(preset.idle);
        opts.cfg.setSpeed(preset.speed);
        opts.cfg.setMusic(preset.music);
    }

    function liftId(row) {
        const id = String(row && row.id || "");
        return id.indexOf("plugin:") === 0 ? id.slice(7) : id;
    }
    function chooseModel(model) {
        if (!model || !model.id || opts.backend.selectedModelId(opts.cfg.quality, opts.cfg.models) === model.id) {
            opts.modelPickerOpen = false;
            return;
        }
        opts.cfg.setModel(opts.cfg.quality, model.id);
        opts.pendingModelRecut = model.id;
        opts.modelPickerOpen = model.installed !== true;
        modelRecut.restart();
    }

    function recutSelectedModel() {
        if (opts.pendingModelRecut === "")
            return;
        const model = opts.backend.modelById(opts.pendingModelRecut);
        if (!model || model.installed !== true)
            return;
        if (opts.effect !== "off")
            opts.backend.refresh();
        opts.pendingModelRecut = "";
    }

    onQualityReadyChanged: if (qualityReady && opts.pendingModelRecut !== "")
        modelRecut.restart()

    Timer {
        id: modelRecut
        interval: 120
        onTriggered: opts.recutSelectedModel()
    }

    Component.onCompleted: if (!opts.backend.checked)
        opts.backend.recheck()

    component NoticePlate: Rectangle {
        id: plate

        property string title: ""
        property string detail: ""
        property string actionText: ""
        property bool danger: false
        property real progress: -1
        signal act()

        width: parent ? parent.width : 0
        implicitHeight: noticeBody.implicitHeight + Tokens.s3 * 2
        radius: Tokens.radius
        color: plate.danger
            ? Qt.rgba(Tokens.alert.r, Tokens.alert.g, Tokens.alert.b, 0.08)
            : Tokens.tint5
        border.width: Tokens.border
        border.color: plate.danger ? Tokens.alert : Tokens.line
        clip: true

        Row {
            id: noticeBody
            anchors {
                left: parent.left
                right: parent.right
                verticalCenter: parent.verticalCenter
                margins: Tokens.s3
            }
            spacing: Tokens.s3

            Column {
                width: Math.max(0, noticeBody.width
                    - (noticeAction.visible ? noticeAction.implicitWidth + noticeBody.spacing : 0))
                spacing: Tokens.s1

                Text {
                    width: parent.width
                    text: plate.title
                    color: Tokens.ink
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                    font.weight: Font.DemiBold
                    elide: Text.ElideRight
                }
                Text {
                    visible: plate.detail !== ""
                    width: parent.width
                    text: plate.detail
                    color: Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                    wrapMode: Text.WordWrap
                    maximumLineCount: 3
                    elide: Text.ElideRight
                }
            }

            Btn {
                id: noticeAction
                visible: plate.actionText !== ""
                text: plate.actionText
                compact: true
                primary: plate.danger
                anchors.verticalCenter: parent.verticalCenter
                onAct: plate.act()
            }
        }

        Rectangle {
            visible: plate.progress >= 0
            anchors {
                left: parent.left
                bottom: parent.bottom
            }
            width: parent.width * Math.max(0, Math.min(1, plate.progress))
            height: Tokens.border * 2
            color: plate.danger ? Tokens.alert : Tokens.ink
            Behavior on width {
                NumberAnimation { duration: Tokens.move; easing.type: Tokens.ease }
            }
        }
    }

    component LayerCard: SettingCard {
        id: layerCard

        required property int layerIndex
        readonly property string layerName: opts.backend.layerLabel(opts.wall, layerCard.layerIndex)

        width: parent ? parent.width : 0
        title: layerCard.layerName.toUpperCase()
        kana: layerCard.layerIndex === 0 ? "主" : "層"
        collapsible: false

        SettingRow {
            width: parent.width
            label: I18n.tr("Shown")
            desc: I18n.tr("Draw this cut-out in the scene")
            controlWidth: Tokens.s7 + Tokens.s2
            Sw {
                anchors.right: parent.right
                on: opts.backend.layerEnabled(opts.wall, layerCard.layerIndex)
                onToggled: value => opts.backend.setLayer(layerCard.layerIndex, { enabled: value })
            }
        }
        SettingRow {
            width: parent.width
            divider: true
            label: I18n.tr("Plane")
            desc: I18n.tr("Place the cut-out behind or over the desktop")
            controlWidth: width * 0.5
            Seg {
                width: parent.width
                options: ["behind", "front"]
                labels: ({
                    behind: I18n.tr("Behind"),
                    front: I18n.tr("In front")
                })
                current: opts.backend.layerFront(opts.wall, layerCard.layerIndex)
                    ? "front" : "behind"
                onChose: value => opts.backend.setLayerFront(layerCard.layerIndex, value === "front")
            }
        }
        SettingRow {
            visible: opts.parallax
            width: parent.width
            divider: true
            label: I18n.tr("Distance")
            desc: I18n.tr("How far this layer travels in Parallax") + " · "
                + opts.pct(opts.backend.layerDepth(opts.wall, layerCard.layerIndex))
            value: opts.pct(opts.backend.layerDepth(opts.wall, layerCard.layerIndex))
            block: true
            Slid {
                width: parent.width
                from: 0
                to: 1
                value: opts.backend.layerDepth(opts.wall, layerCard.layerIndex)
                onModified: value => opts.backend.setLayerDepth(layerCard.layerIndex, value)
            }
        }
        SettingRow {
            visible: layerCard.layerIndex > 0
            width: parent.width
            divider: true
            label: I18n.tr("Remove layer")
            desc: I18n.tr("The subject is kept as the scene's anchor")
            controlWidth: removeLayer.implicitWidth
            Btn {
                id: removeLayer
                text: I18n.tr("Remove")
                compact: true
                onAct: opts.backend.removeLayer(layerCard.layerIndex)
            }
        }
    }

    Column {
        visible: opts.section === "Scene"
        width: parent.width
        spacing: Tokens.s4

        StageDepthHero {
            width: parent.width
            wallpaperUrl: opts.fileUrl(opts.previewPath)
            subjectUrl: opts.effect !== "off" && opts.layerCount > 0
                ? opts.backend.layerUrlFor(opts.wall, 0) : ""
            mode: opts.effectLabel()
            state: opts.sceneState()
            busy: opts.backend.busy
        }

        SettingCard {
            width: parent.width
            title: I18n.tr("SCENE")
            kana: "景"
            collapsible: false

            SettingRow {
                width: parent.width
                label: I18n.tr("Mode")
                desc: I18n.tr("Plain stays flat; Depth cuts layers; Parallax adds motion")
                block: true
                Seg {
                    width: parent.width
                    options: ["off", "depth", "parallax"]
                    labels: ({
                        off: I18n.tr("Plain"),
                        depth: I18n.tr("Depth"),
                        parallax: I18n.tr("Parallax")
                    })
                    current: opts.effect
                    onChose: value => opts.backend.setEffect(value)
                }
            }
        }

        NoticePlate {
            visible: opts.backend.busy
            title: I18n.tr("Cutting the wallpaper")
            detail: opts.backend.stage !== ""
                ? opts.backend.stage : I18n.tr("Building the scene layers")
            actionText: I18n.tr("Stop")
            progress: opts.backend.percent / 100
            onAct: opts.backend.cancel()
        }

        NoticePlate {
            visible: !opts.backend.busy
                && opts.backend.notice !== ""
                && opts.effect !== "off"
            title: I18n.tr("The scene could not be cut")
            detail: opts.backend.notice
            actionText: I18n.tr("Retry")
            danger: true
            onAct: opts.backend.refresh()
        }

        SettingCard {
            width: parent.width
            title: I18n.tr("CUT QUALITY")
            kana: "切"
            collapsible: false

            SettingRow {
                width: parent.width
                label: I18n.tr("Quality")
                desc: I18n.tr("Each tier remembers its model and edge treatment")
                block: true
                Seg {
                    width: parent.width
                    options: ["draft", "standard", "fine"]
                    labels: ({
                        draft: I18n.tr("Draft"),
                        standard: I18n.tr("Standard"),
                        fine: I18n.tr("Fine")
                    })
                    current: opts.cfg.quality
                    onChose: value => {
                        if (value === opts.cfg.quality)
                            return;
                        opts.cfg.setQuality(value);
                        opts.pendingModelRecut = opts.backend.selectedModelId(value, opts.cfg.models);
                        modelRecut.restart();
                    }
                }
            }

            SettingRow {
                width: parent.width
                divider: true
                label: I18n.tr("Model for %1").arg(
                    opts.cfg.quality === "fine" ? I18n.tr("Fine")
                    : opts.cfg.quality === "standard" ? I18n.tr("Standard")
                    : I18n.tr("Draft"))
                desc: opts.qualityModel
                    ? I18n.tr("%1 · %2 · %3").arg(opts.qualityModel.label || opts.qualityModel.id)
                        .arg(opts.qualityModel.size || I18n.tr("size unknown"))
                        .arg(opts.qualityModel.licence || I18n.tr("licence unknown"))
                    : I18n.tr("Checking the model catalogue")
                block: true

                Column {
                    width: parent.width
                    spacing: Tokens.s2

                    Flow {
                        width: parent.width
                        spacing: Tokens.s2

                        Btn {
                            text: opts.modelPickerOpen ? I18n.tr("Close models") : I18n.tr("Choose model")
                            compact: true
                            onAct: opts.modelPickerOpen = !opts.modelPickerOpen
                        }
                        Btn {
                            text: I18n.tr("Re-cut")
                            primary: opts.qualityReady && opts.effect !== "off"
                            armed: opts.qualityReady && opts.effect !== "off" && !opts.backend.busy
                            compact: true
                            onAct: opts.backend.refresh()
                        }
                    }

                    Column {
                        id: modelList
                        visible: opts.modelPickerOpen
                        width: parent.width
                        spacing: Tokens.s1

                        Repeater {
                            model: opts.backend.models || []
                            delegate: Rectangle {
                                id: modelRow

                                required property var modelData
                                readonly property bool selected: opts.qualityModel
                                    && opts.qualityModel.id === modelRow.modelData.id

                                width: modelList.width
                                implicitHeight: modelBody.implicitHeight + Tokens.s2 * 2
                                radius: Tokens.radius
                                color: modelRow.selected ? Tokens.tint10 : Tokens.tint5
                                border.width: Tokens.border
                                border.color: modelRow.selected ? Tokens.lineStrong : Tokens.line

                                Row {
                                    id: modelBody
                                    anchors {
                                        left: parent.left
                                        right: parent.right
                                        verticalCenter: parent.verticalCenter
                                        margins: Tokens.s2
                                    }
                                    spacing: Tokens.s2

                                    Column {
                                        width: Math.max(0, modelBody.width - modelActions.implicitWidth - modelBody.spacing)
                                        spacing: Tokens.s1

                                        Text {
                                            width: parent.width
                                            text: modelRow.modelData.label || modelRow.modelData.id
                                            color: Tokens.ink
                                            font.family: Tokens.ui
                                            font.pixelSize: Tokens.fSmall
                                            font.weight: modelRow.selected ? Font.DemiBold : Font.Normal
                                            elide: Text.ElideRight
                                        }
                                        Text {
                                            width: parent.width
                                            text: I18n.tr("%1 · %2 · %3")
                                                .arg(modelRow.modelData.size || I18n.tr("size unknown"))
                                                .arg(modelRow.modelData.licence || I18n.tr("licence unknown"))
                                                .arg(modelRow.modelData.installed === true
                                                    ? I18n.tr("Downloaded") : I18n.tr("Not downloaded"))
                                            color: Tokens.inkMuted
                                            font.family: Tokens.ui
                                            font.pixelSize: Tokens.fTiny
                                            elide: Text.ElideRight
                                        }
                                    }

                                    Row {
                                        id: modelActions
                                        spacing: Tokens.s1
                                        anchors.verticalCenter: parent.verticalCenter

                                        Btn {
                                            text: modelRow.selected ? I18n.tr("Selected") : I18n.tr("Use")
                                            compact: true
                                            primary: modelRow.selected
                                            armed: !modelRow.selected
                                                && !opts.backend.installing && !opts.backend.removing
                                            onAct: opts.chooseModel(modelRow.modelData)
                                        }
                                        Btn {
                                            text: modelRow.modelData.installed === true
                                                ? (opts.backend.removingModel === modelRow.modelData.id
                                                    ? I18n.tr("Removing…") : I18n.tr("Remove"))
                                                : (opts.backend.installingModel === modelRow.modelData.id
                                                    ? I18n.tr("Downloading…") : I18n.tr("Download"))
                                            compact: true
                                            armed: !opts.backend.installing && !opts.backend.removing
                                            onAct: {
                                                if (modelRow.modelData.installed === true)
                                                    opts.backend.remove(modelRow.modelData.id);
                                                else
                                                    opts.backend.install(modelRow.modelData.id);
                                            }
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }

        NoticePlate {
            visible: (opts.backend.installing || opts.backend.removing)
                && opts.backend.progress !== ""
            title: opts.backend.installing
                ? I18n.tr("Preparing the cut model")
                : I18n.tr("Removing the cut model")
            detail: opts.backend.progress
        }
    }

    Column {
        visible: opts.section === "Layers"
        width: parent.width
        spacing: Tokens.s4

        SettingCard {
            width: parent.width
            title: I18n.tr("ADD LAYERS")
            kana: "層"
            collapsible: false

            SettingRow {
                width: parent.width
                label: I18n.tr("Cut a layer from a picture")
                desc: I18n.tr("Choose a picture, then confirm the cut")
                controlWidth: chooseCut.implicitWidth
                Btn {
                    id: chooseCut
                    text: I18n.tr("Choose")
                    compact: true
                    onAct: cutDialog.open()
                }
            }
            SettingRow {
                width: parent.width
                divider: true
                label: I18n.tr("Add a PNG")
                desc: I18n.tr("Use an existing transparent cut-out")
                controlWidth: addPng.implicitWidth
                Btn {
                    id: addPng
                    text: I18n.tr("Choose")
                    compact: true
                    onAct: pngDialog.open()
                }
            }
        }

        NoticePlate {
            visible: opts.pickedCut !== ""
            title: opts.leaf(opts.pickedCut)
            detail: I18n.tr("Cut this picture into a new layer?")
            actionText: I18n.tr("Cut")
            onAct: {
                opts.backend.cutLayer(opts.pathFromUrl(opts.pickedCut));
                opts.pickedCut = "";
            }
        }

        Empty {
            visible: opts.layerCount === 0
            width: parent.width
            caption: I18n.tr("Choose Depth or Parallax, or add a PNG, to build the layer stack.")
        }

        Repeater {
            model: opts.layerCount
            delegate: LayerCard {
                required property int index
                layerIndex: index
            }
        }

        SettingCard {
            visible: opts.layerCount > 0
            width: parent.width
            title: I18n.tr("CUT-OUTS")
            kana: "消"
            collapsible: false

            SettingRow {
                width: parent.width
                label: opts.clearArmed
                    ? I18n.tr("Remove every cut-out?")
                    : I18n.tr("Clear cut-outs")
                desc: opts.clearArmed
                    ? I18n.tr("The wallpaper stays; every generated and added layer is removed")
                    : I18n.tr("Start this wallpaper's layer stack again")
                block: opts.clearArmed
                controlWidth: clearButton.implicitWidth

                Btn {
                    id: clearButton
                    visible: !opts.clearArmed
                    text: I18n.tr("Clear")
                    compact: true
                    onAct: opts.clearArmed = true
                }
                Flow {
                    visible: opts.clearArmed
                    width: parent.width
                    spacing: Tokens.s2
                    Btn {
                        text: I18n.tr("Cancel")
                        compact: true
                        onAct: opts.clearArmed = false
                    }
                    Btn {
                        text: I18n.tr("Clear cut-outs")
                        compact: true
                        primary: true
                        onAct: {
                            opts.backend.clear();
                            opts.clearArmed = false;
                        }
                    }
                }
            }
        }
    }

    Column {
        visible: opts.section === "Look"
        width: parent.width
        spacing: Tokens.s4

        SettingCard {
            width: parent.width
            title: I18n.tr("LOOK")
            kana: "質感"
            collapsible: false

            SettingRow {
                width: parent.width
                label: I18n.tr("Edge softness")
                desc: I18n.tr("Feather the boundary between a cut-out and its backdrop")
                    + " · " + opts.pct(opts.cfg.edge)
                value: opts.pct(opts.cfg.edge)
                block: true
                Slid {
                    width: parent.width
                    from: 0
                    to: 1
                    value: opts.cfg.edge
                    onModified: value => opts.cfg.setEdge(value)
                }
            }
            SettingRow {
                width: parent.width
                divider: true
                label: I18n.tr("Shadow strength")
                desc: I18n.tr("Separate near layers from the wallpaper")
                    + " · " + opts.pct(opts.cfg.shadow)
                value: opts.pct(opts.cfg.shadow)
                block: true
                Slid {
                    width: parent.width
                    from: 0
                    to: 1
                    value: opts.cfg.shadow
                    onModified: value => opts.cfg.setShadow(value)
                }
            }
            SettingRow {
                width: parent.width
                divider: true
                label: I18n.tr("Shadow direction")
                desc: I18n.tr("Drag around the dial to place the light")
                    + " · " + Math.round(opts.cfg.shadowAngle) + "°"
                value: Math.round(opts.cfg.shadowAngle) + "°"
                block: true
                StageAngleDial {
                    anchors.horizontalCenter: parent.horizontalCenter
                    dim: Math.min(parent.width, Tokens.s7 * 2)
                    implicitHeight: dim
                    angle: opts.cfg.shadowAngle
                    onChanged: degrees => opts.cfg.setShadowAngle(degrees)
                }
            }
            SettingRow {
                width: parent.width
                divider: true
                label: I18n.tr("Reset look and motion")
                desc: I18n.tr("Restore edge, shadow and Parallax motion defaults")
                controlWidth: resetLook.implicitWidth
                Btn {
                    id: resetLook
                    text: I18n.tr("Reset")
                    compact: true
                    onAct: opts.cfg.resetLook()
                }
            }
        }
    }

    Column {
        visible: opts.section === "Motion"
        width: parent.width
        spacing: Tokens.s4

        NoticePlate {
            visible: !opts.parallax
            title: I18n.tr("Motion needs Parallax")
            detail: I18n.tr("Depth keeps the cut-outs still. Switch modes to tune scene motion.")
            actionText: I18n.tr("Use Parallax")
            onAct: opts.backend.setEffect("parallax")
        }

        SettingCard {
            width: parent.width
            title: I18n.tr("PRESET")
            kana: "型"
            collapsible: false

            SettingRow {
                enabled: opts.parallax
                width: parent.width
                label: I18n.tr("Motion character")
                desc: I18n.tr("A preset stays selected only while all of its values still match")
                block: true
                Seg {
                    width: parent.width
                    options: ["soft", "cinematic", "beat"]
                    labels: ({
                        soft: I18n.tr("Soft"),
                        cinematic: I18n.tr("Cinematic"),
                        beat: I18n.tr("Beat")
                    })
                    current: opts.motionPreset()
                    onChose: value => opts.applyMotionPreset(value)
                }
            }
        }

        SettingCard {
            width: parent.width
            title: I18n.tr("MOTION")
            kana: "動"
            collapsible: false

            SettingRow {
                enabled: opts.parallax
                width: parent.width
                label: I18n.tr("Amount")
                desc: I18n.tr("How far the whole composition travels")
                block: true
                Seg {
                    width: parent.width
                    options: ["subtle", "normal", "strong"]
                    labels: ({
                        subtle: I18n.tr("Subtle"),
                        normal: I18n.tr("Normal"),
                        strong: I18n.tr("Strong")
                    })
                    current: opts.cfg.amount
                    onChose: value => opts.cfg.setAmount(value)
                }
            }
            SettingRow {
                enabled: opts.parallax
                width: parent.width
                divider: true
                label: I18n.tr("Idle style")
                desc: I18n.tr("Movement while the pointer rests")
                block: true
                Seg {
                    width: parent.width
                    options: ["none", "float", "breathe", "sway"]
                    labels: ({
                        none: I18n.tr("Still"),
                        float: I18n.tr("Float"),
                        breathe: I18n.tr("Breathe"),
                        sway: I18n.tr("Sway")
                    })
                    current: opts.cfg.idle
                    onChose: value => opts.cfg.setIdle(value)
                }
            }
            SettingRow {
                enabled: opts.parallax && opts.cfg.idle !== "none"
                width: parent.width
                divider: true
                label: I18n.tr("Idle speed")
                desc: I18n.tr("How quickly the idle movement loops")
                    + " · " + opts.cfg.speed.toFixed(2) + "×"
                value: opts.cfg.speed.toFixed(2) + "×"
                block: true
                Slid {
                    width: parent.width
                    from: 0.25
                    to: 2
                    value: opts.cfg.speed
                    onModified: value => opts.cfg.setSpeed(value)
                }
            }
        }

        SettingCard {
            width: parent.width
            title: I18n.tr("MUSIC")
            kana: "音"
            collapsible: false

            SettingRow {
                enabled: opts.parallax
                width: parent.width
                label: I18n.tr("React to music")
                desc: I18n.tr("Near layers pulse with the beat")
                controlWidth: Tokens.s7 + Tokens.s2
                Sw {
                    anchors.right: parent.right
                    on: opts.cfg.music
                    onToggled: value => opts.cfg.setMusic(value)
                }
            }
            SettingRow {
                enabled: opts.parallax && opts.cfg.music
                width: parent.width
                divider: true
                label: I18n.tr("Music intensity")
                desc: I18n.tr("How strongly the scene answers the beat")
                    + " · " + opts.pct(opts.cfg.musicLevel)
                value: opts.pct(opts.cfg.musicLevel)
                block: true
                Slid {
                    width: parent.width
                    from: 0
                    to: 1
                    value: opts.cfg.musicLevel
                    onModified: value => opts.cfg.setMusicLevel(value)
                }
            }
        }

        SettingCard {
            width: parent.width
            title: I18n.tr("POINTER")
            kana: "指"
            collapsible: false

            SettingRow {
                enabled: opts.parallax
                width: parent.width
                label: I18n.tr("Follow pointer")
                desc: I18n.tr("Layers drift with the cursor")
                controlWidth: Tokens.s7 + Tokens.s2
                Sw {
                    anchors.right: parent.right
                    on: opts.cfg.followMouse
                    onToggled: value => opts.cfg.setMouse(value)
                }
            }
            SettingRow {
                enabled: opts.parallax && opts.cfg.followMouse
                width: parent.width
                divider: true
                label: I18n.tr("Sensitivity")
                desc: I18n.tr("How readily the scene follows small movements")
                    + " · " + opts.pct(opts.cfg.sensitivity)
                value: opts.pct(opts.cfg.sensitivity)
                block: true
                Slid {
                    width: parent.width
                    from: 0
                    to: 2
                    value: opts.cfg.sensitivity
                    onModified: value => opts.cfg.setSensitivity(value)
                }
            }
            SettingRow {
                enabled: opts.parallax && opts.cfg.followMouse
                width: parent.width
                divider: true
                label: I18n.tr("Range")
                desc: I18n.tr("The furthest the cut-outs can travel")
                    + " · " + opts.pct(opts.cfg.range)
                value: opts.pct(opts.cfg.range)
                block: true
                Slid {
                    width: parent.width
                    from: 0
                    to: 2
                    value: opts.cfg.range
                    onModified: value => opts.cfg.setRange(value)
                }
            }
            SettingRow {
                enabled: opts.parallax && opts.cfg.followMouse
                width: parent.width
                divider: true
                label: I18n.tr("Backdrop drift")
                desc: I18n.tr("Let the wallpaper move against the cut-outs")
                    + " · " + opts.pct(opts.cfg.backdrop)
                value: opts.pct(opts.cfg.backdrop)
                block: true
                Slid {
                    width: parent.width
                    from: 0
                    to: 1
                    value: opts.cfg.backdrop
                    onModified: value => opts.cfg.setBackdrop(value)
                }
            }
        }
    }

    Column {
        visible: opts.section === "In front"
        width: parent.width
        spacing: Tokens.s4

        NoticePlate {
            visible: opts.layerCount === 0
            title: I18n.tr("No in-front layers yet")
            detail: I18n.tr("Cut the wallpaper first, then choose which desktop pieces rise above its foreground.")
            actionText: I18n.tr("Use Depth")
            onAct: opts.backend.setEffect("depth")
        }

        Empty {
            visible: opts.layerCount > 0 && opts.visibleRows.length === 0
            width: parent.width
            caption: I18n.tr("Place a widget or visualizer, then return here to choose its depth.")
        }

        SettingCard {
            visible: opts.layerCount > 0 && opts.visibleRows.length > 0
            width: parent.width
            title: I18n.tr("DESKTOP LAYERS")
            kana: "前面"
            summary: I18n.tr("%1 placed").arg(opts.visibleRows.length)
            collapsible: false

            Repeater {
                model: opts.visibleRows
                delegate: SettingRow {
                    id: liftRow

                    required property var modelData
                    required property int index
                    readonly property string key: opts.liftId(liftRow.modelData)

                    width: parent.width
                    divider: index > 0
                    label: modelData.label
                    desc: modelData.id.indexOf("plugin:") === 0
                        ? I18n.tr("Plugin tile · keyed by %1").arg(liftRow.key)
                        : I18n.tr("Placed desktop item")
                    controlWidth: Tokens.s7 + Tokens.s2

                    Sw {
                        anchors.right: parent.right
                        on: opts.cfg.isFront(liftRow.key)
                        onToggled: value => opts.cfg.setFront(liftRow.key, value)
                    }
                }
            }
        }
    }

    FileDialog {
        id: cutDialog
        title: I18n.tr("Cut a layer from a picture")
        nameFilters: [
            I18n.tr("Pictures (*.png *.jpg *.jpeg *.webp *.bmp)"),
            I18n.tr("All files (*)")
        ]
        onAccepted: opts.pickedCut = String(selectedFile)
    }

    FileDialog {
        id: pngDialog
        title: I18n.tr("Add a PNG layer")
        nameFilters: [
            I18n.tr("PNG images (*.png)"),
            I18n.tr("All files (*)")
        ]
        onAccepted: opts.backend.addLayer(opts.pathFromUrl(selectedFile))
    }
}
