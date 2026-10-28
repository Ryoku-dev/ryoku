pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Controls as QQC
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../visualizer/Singletons" as VizCfg
import "Singletons" as StageCfg
import stage.modules.common as StageIsland

Item {
    id: page

    readonly property var cfg: VizCfg.Config
    readonly property var inst: VizCfg.Config.instance
    readonly property var provider: StageIsland.Config.widgetProvider
    readonly property string sid: page.cfg.styleId
    readonly property bool aura: page.cfg.isAura
    readonly property bool shapeSet: ["bars", "split", "dots", "segments", "frame", "radial", "spiral"].indexOf(page.sid) >= 0
    readonly property bool growSet: ["bars", "split", "dots", "segments", "wave", "ribbon", "curtain", "line"].indexOf(page.sid) >= 0
    readonly property bool polarSet: ["radial", "orb", "spiral"].indexOf(page.sid) >= 0
    readonly property bool bloomSet: page.sid !== "aura"
    readonly property bool segSet: page.sid === "segments"
    readonly property bool reflSet: page.growSet && page.inst.grow === "up"
    readonly property real aspect: page.provider ? page.provider.aspect() : 1
    property string tab: "Look"
    onAuraChanged: if (!page.aura && page.tab === "Field") page.tab = "Look"
    property var placeBefore: null
    property string placeId: ""

    function cap(s) {
        const text = "" + (s || "");
        return text.length ? text.charAt(0).toUpperCase() + text.slice(1) : "";
    }
    function hexOf(c) {
        return "#" + [c.r, c.g, c.b].map(function (x) {
            const s = Math.round(x * 255).toString(16);
            return s.length === 1 ? "0" + s : s;
        }).join("").toUpperCase();
    }
    function selectInstance(instanceId) {
        page.finishPlace();
        if (!page.cfg.setActive(instanceId))
            return;
        StageCfg.StageSession.select("visualizer:" + instanceId);
    }
    function addInstance() {
        if (page.cfg.count >= page.cfg.maxVisualizers)
            return;
        const instanceId = page.provider ? page.provider.addVisualizer() : page.cfg.addVisualizer();
        if (!instanceId)
            return;
        page.cfg.setEnabled(true);
        page.selectInstance(instanceId);
    }
    function toggleShown() {
        const id = "visualizer:" + page.cfg.activeId;
        if (page.cfg.enabled) {
            StageIsland.Config.removeWidgetInstance("visualizer");
            StageCfg.StageSession.remove(id);
        } else {
            StageIsland.Config.addWidgetToDesktop("visualizer");
            page.selectInstance(page.cfg.activeId);
        }
    }
    function placeChange(change) {
        const id = "visualizer:" + page.cfg.activeId;
        if (page.placeBefore === null || page.placeId !== id) {
            page.finishPlace();
            page.placeId = id;
            page.placeBefore = page.provider ? page.provider.snapshot(id) : null;
        }
        change();
        placeSettle.restart();
    }
    function finishPlace() {
        placeSettle.stop();
        if (page.provider && page.placeBefore)
            page.provider.recordVisualizer(page.placeId, page.placeBefore);
        page.placeBefore = null;
        page.placeId = "";
    }
    function placeOnce(change) {
        page.finishPlace();
        const id = "visualizer:" + page.cfg.activeId;
        const before = page.provider ? page.provider.snapshot(id) : null;
        change();
        if (page.provider)
            page.provider.recordVisualizer(id, before);
    }
    function ensureStops() {
        if (!page.cfg.hasCustomColor)
            page.cfg.setColor(page.hexOf(Tokens.sun));
        if (!page.cfg.hasColor2)
            page.cfg.setColor2(page.hexOf(Qt.lighter(Tokens.sun, 1.5)));
    }

    Timer {
        id: placeSettle
        interval: Tokens.swap * 2
        onTriggered: page.finishPlace()
    }

    Connections {
        target: page.cfg
        function onActiveChanged() {
            if (page.placeBefore)
                page.finishPlace();
            StageCfg.StageSession.select("visualizer:" + page.cfg.activeId);
        }
    }

    component SliderRow: SettingRow {
        id: row
        property real amount: 0
        property real minimum: 0
        property real maximum: 1
        property string shown: Math.round(amount * 100) + "%"
        property var writeValue: null
        width: parent ? parent.width : 0
        value: shown
        controlWidth: Tokens.s7 * 2 + Tokens.s5
        Slid {
            anchors.fill: parent
            value: row.amount
            from: row.minimum
            to: row.maximum
            onModified: v => { if (row.writeValue) row.writeValue(v); }
        }
    }

    component StepRow: SettingRow {
        id: row
        property int amount: 0
        property int minimum: 0
        property int maximum: 100
        property int stride: 1
        property var writeValue: null
        width: parent ? parent.width : 0
        value: String(amount)
        controlWidth: Tokens.s6 + Tokens.s5
        Step {
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            value: row.amount
            from: row.minimum
            to: row.maximum
            stepBy: row.stride
            onModified: v => { if (row.writeValue) row.writeValue(v); }
        }
    }

    component SwitchRow: SettingRow {
        id: row
        property bool onState: false
        property var writeValue: null
        width: parent ? parent.width : 0
        controlWidth: Tokens.s7 + Tokens.s2
        Sw {
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            on: row.onState
            onToggled: v => { if (row.writeValue) row.writeValue(v); }
        }
    }

    component SegmentRow: SettingRow {
        id: row
        property var choices: []
        property var choiceLabels: ({})
        property string chosen: ""
        property var writeValue: null
        width: parent ? parent.width : 0
        block: row.choices.length >= 4
        controlWidth: Tokens.s7 * 4 + Tokens.s5
        Seg {
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            options: row.choices
            labels: row.choiceLabels
            current: row.chosen
            onChose: key => { if (row.writeValue) row.writeValue(key); }
        }
    }

    component ChipsRow: SettingRow {
        id: row
        property var choices: []
        property var choiceLabels: ({})
        property string chosen: ""
        property var writeValue: null
        width: parent ? parent.width : 0
        block: true
        Chips {
            width: parent.width
            options: row.choices
            labels: row.choiceLabels
            current: row.chosen
            onChose: key => { if (row.writeValue) row.writeValue(key); }
        }
    }

    component ColourRow: SettingRow {
        id: row
        property string current: ""
        property var writeValue: null
        width: parent ? parent.width : 0
        block: true
        Flow {
            width: parent.width
            spacing: Tokens.s2
            Repeater {
                model: [Tokens.sun, Tokens.ink, Tokens.inkDim, Tokens.bone, Tokens.paperLift]
                delegate: Rectangle {
                    required property var modelData
                    readonly property string toneHex: page.hexOf(modelData)
                    width: Tokens.s6 + Tokens.s3
                    height: Tokens.s5 + Tokens.s1
                    radius: Tokens.radius
                    color: modelData
                    border.width: toneHex.toUpperCase() === row.current.toUpperCase()
                        ? Tokens.border * 3 : Tokens.border
                    border.color: toneHex.toUpperCase() === row.current.toUpperCase() ? Tokens.ink : Tokens.lineStrong
                    HoverHandler { cursorShape: Qt.PointingHandCursor }
                    TapHandler {
                        onTapped: if (row.writeValue) row.writeValue(parent.toneHex)
                    }
                }
            }
            Field {
                width: Tokens.s7 * 2
                text: row.current
                placeholder: "#RRGGBB"
                tabular: true
                onCommitted: value => {
                    let next = String(value).trim();
                    if (next.length > 0 && next[0] !== "#")
                        next = "#" + next;
                    if (/^#[0-9a-fA-F]{6}$/.test(next) && row.writeValue)
                        row.writeValue(next);
                }
            }
        }
    }

    Flickable {
        id: scroll
        anchors.fill: parent
        clip: true
        contentWidth: width
        contentHeight: content.implicitHeight + Tokens.s4
        boundsBehavior: Flickable.StopAtBounds
        QQC.ScrollBar.vertical: QQC.ScrollBar {}

        WheelScroll {}

        Column {
            id: content
            width: scroll.width
            spacing: Tokens.s2

            SettingRow {
                width: parent.width
                label: I18n.tr("Show")
                controlWidth: Tokens.s7 + Tokens.s2
                Sw {
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    on: page.cfg.enabled
                    onToggled: page.toggleShown()
                }
            }

            SettingRow {
                width: parent.width
                label: I18n.tr("Instances")
                divider: true
                controlWidth: Tokens.s7 * 4 + Tokens.s6
                Row {
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Tokens.s2
                    Chips {
                        options: {
                            const out = [];
                            for (let i = 0; i < page.cfg.count; ++i)
                                out.push(page.cfg.labelAt(i));
                            return out;
                        }
                        current: page.cfg.labelAt(page.cfg.active)
                        onChose: label => {
                            for (let i = 0; i < page.cfg.count; ++i)
                                if (page.cfg.labelAt(i) === label) {
                                    page.selectInstance(page.cfg.idAt(i));
                                    return;
                                }
                        }
                    }
                    Btn {
                        text: I18n.tr("Add")
                        compact: true
                        armed: page.cfg.count < page.cfg.maxVisualizers
                        onAct: page.addInstance()
                    }
                }
            }

            Chips {
                width: parent.width
                options: page.aura
                    ? ["Look", "Place", "Colour", "Motion", "Shape", "Field"]
                    : ["Look", "Place", "Colour", "Motion", "Shape"]
                current: page.tab
                onChose: value => page.tab = value
            }

            SettingCard {
                visible: page.tab === "Look"
                width: parent.width
                title: I18n.tr("LOOK GALLERY")
                kana: "姿"
                collapsible: false
                SettingRow {
                    width: parent.width
                    label: I18n.tr("Style")
                    desc: VizStyles.whatOf(page.sid)
                    block: true
                    Gallery {
                        width: parent.width
                        options: VizStyles.styles.map(function (s) {
                            return { key: s.key, origin: s.kind, draw: s.key };
                        })
                        painter: VizStyles
                        current: page.sid
                        onChose: key => page.cfg.setStyle(key)
                    }
                }
            }

            SettingCard {
                visible: page.tab === "Place"
                width: parent.width
                title: I18n.tr("PLACE")
                kana: "配置"
                collapsible: false
                SliderRow {
                    visible: !page.aura
                    label: I18n.tr("Width")
                    desc: I18n.tr("Horizontal span of this instance")
                    amount: page.cfg.w
                    minimum: 0.04
                    maximum: 1
                    writeValue: v => page.placeChange(() => page.cfg.sizeBox(v, page.cfg.h, page.aspect))
                }
                SliderRow {
                    visible: !page.aura
                    divider: true
                    label: I18n.tr("Height")
                    desc: I18n.tr("Vertical span of this instance")
                    amount: page.cfg.h
                    minimum: 0.03
                    maximum: 1
                    writeValue: v => page.placeChange(() => page.cfg.sizeBox(page.cfg.w, v, page.aspect))
                }
                SliderRow {
                    visible: !page.aura
                    divider: true
                    label: I18n.tr("Across")
                    desc: I18n.tr("Centre from left to right")
                    amount: page.cfg.x + page.cfg.w / 2
                    writeValue: v => page.placeChange(() => page.cfg.moveBox(v - page.cfg.w / 2, page.cfg.y, page.aspect))
                }
                SliderRow {
                    visible: !page.aura
                    divider: true
                    label: I18n.tr("Down")
                    desc: I18n.tr("Centre from top to bottom")
                    amount: page.cfg.y + page.cfg.h / 2
                    writeValue: v => page.placeChange(() => page.cfg.moveBox(page.cfg.x, v - page.cfg.h / 2, page.aspect))
                }
                SliderRow {
                    visible: !page.aura
                    divider: true
                    label: I18n.tr("Turn")
                    desc: I18n.tr("Rotate around the box centre")
                    amount: page.cfg.angle > 180 ? page.cfg.angle - 360 : page.cfg.angle
                    minimum: -180
                    maximum: 180
                    shown: Math.round(amount) + "\u00b0"
                    writeValue: v => page.placeChange(() => page.cfg.rotate(v))
                }
                SettingRow {
                    visible: !page.aura
                    width: parent.width
                    divider: true
                    label: I18n.tr("Quick place")
                    desc: I18n.tr("Useful starting points for direct manipulation")
                    block: true
                    Flow {
                        width: parent.width
                        spacing: Tokens.s2
                        Btn {
                            text: I18n.tr("Centre")
                            compact: true
                            onAct: page.placeOnce(() => page.cfg.moveBox(
                                0.5 - page.cfg.w / 2, 0.5 - page.cfg.h / 2, page.aspect))
                        }
                        Btn {
                            text: I18n.tr("Square")
                            compact: true
                            onAct: page.placeOnce(() => {
                                page.cfg.rotate(0);
                                page.cfg.moveBox(page.cfg.x, page.cfg.y, page.aspect);
                            })
                        }
                        Btn {
                            text: I18n.tr("Full width")
                            compact: true
                            onAct: page.placeOnce(() => page.cfg.setBox(
                                0, page.cfg.y, 1, page.cfg.h, page.aspect))
                        }
                        Btn {
                            text: I18n.tr("Top")
                            compact: true
                            onAct: page.placeOnce(() => page.cfg.moveBox(page.cfg.x, 0, page.aspect))
                        }
                        Btn {
                            text: I18n.tr("Middle")
                            compact: true
                            onAct: page.placeOnce(() => page.cfg.moveBox(
                                page.cfg.x, 0.5 - page.cfg.h / 2, page.aspect))
                        }
                        Btn {
                            text: I18n.tr("Bottom")
                            compact: true
                            onAct: page.placeOnce(() => page.cfg.moveBox(
                                page.cfg.x, 1 - page.cfg.h, page.aspect))
                        }
                    }
                }
                SettingRow {
                    visible: page.aura
                    width: parent.width
                    label: I18n.tr("Full-screen field")
                    desc: I18n.tr("The aura follows the screen edges and has no placement box")
                }
            }

            SettingCard {
                visible: page.tab === "Colour"
                width: parent.width
                title: I18n.tr("COLOUR")
                kana: "彩"
                collapsible: false
                SwitchRow {
                    label: I18n.tr("Gradient")
                    desc: I18n.tr("Blend between the base and second stop")
                    onState: page.cfg.gradient
                    writeValue: value => {
                        if (value)
                            page.ensureStops();
                        page.cfg.setGradient(value);
                    }
                }
                SettingRow {
                    width: parent.width
                    divider: true
                    label: I18n.tr("Wallpaper colour")
                    desc: I18n.tr("Clear pinned colours and follow the desktop palette")
                    controlWidth: Tokens.s7 + Tokens.s5
                    Btn {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("Auto")
                        compact: true
                        onAct: {
                            page.cfg.setGradient(false);
                            page.cfg.clearColor();
                        }
                    }
                }
                ColourRow {
                    divider: true
                    label: I18n.tr("Base")
                    desc: I18n.tr("Primary ink for this instance")
                    current: page.cfg.colorHex
                    writeValue: value => page.cfg.setColor(value)
                }
                ColourRow {
                    divider: true
                    label: I18n.tr("Second")
                    desc: I18n.tr("The far end of the gradient")
                    current: page.cfg.color2Hex
                    writeValue: value => page.cfg.setColor2(value)
                }
                ColourRow {
                    visible: page.aura
                    divider: true
                    label: I18n.tr("Field 2")
                    desc: I18n.tr("Middle colour in the field ramp")
                    current: String(page.inst.auraColor2)
                    writeValue: value => page.cfg.poke("auraColor2", value)
                }
                ColourRow {
                    visible: page.aura
                    divider: true
                    label: I18n.tr("Field 3")
                    desc: I18n.tr("Final colour in the field ramp")
                    current: String(page.inst.auraColor3)
                    writeValue: value => page.cfg.poke("auraColor3", value)
                }
            }

            SettingCard {
                visible: page.tab === "Motion"
                width: parent.width
                title: I18n.tr("MOTION")
                kana: "動"
                collapsible: false
                SwitchRow {
                    label: I18n.tr("Idle wave")
                    desc: I18n.tr("Keep gentle motion when audio is quiet")
                    onState: page.inst.idleWave
                    writeValue: value => page.cfg.poke("idleWave", value)
                }
                SegmentRow {
                    divider: true
                    label: I18n.tr("Frame rate")
                    desc: I18n.tr("Shared render rate for every instance")
                    choices: ["30", "45", "60"]
                    chosen: String(page.cfg.fps)
                    writeValue: value => page.cfg.setFps(parseInt(value))
                }
                SwitchRow {
                    divider: true
                    label: I18n.tr("Adaptive")
                    desc: I18n.tr("Ease the render rate under load")
                    onState: page.cfg.adaptive
                    writeValue: value => page.cfg.setAdaptive(value)
                }
                SliderRow {
                    divider: true
                    label: I18n.tr("Gain")
                    desc: I18n.tr("Audio response strength")
                    amount: page.inst.gain
                    minimum: 0.5
                    maximum: 2
                    writeValue: value => page.cfg.setGain(value)
                }
                SliderRow {
                    divider: true
                    label: I18n.tr("Smoothing")
                    desc: I18n.tr("Settle abrupt level changes")
                    amount: page.inst.smoothing
                    writeValue: value => page.cfg.setSmoothing(value)
                }
                SliderRow {
                    divider: true
                    label: I18n.tr("Lean X")
                    desc: I18n.tr("Horizontal perspective")
                    amount: page.inst.tiltX
                    minimum: -page.cfg.tiltMax
                    maximum: page.cfg.tiltMax
                    shown: Math.round(amount) + "\u00b0"
                    writeValue: value => page.cfg.setTiltX(value)
                }
                SliderRow {
                    divider: true
                    label: I18n.tr("Lean Y")
                    desc: I18n.tr("Vertical perspective")
                    amount: page.inst.tiltY
                    minimum: -page.cfg.tiltMax
                    maximum: page.cfg.tiltMax
                    shown: Math.round(amount) + "\u00b0"
                    writeValue: value => page.cfg.setTiltY(value)
                }
                SettingRow {
                    width: parent.width
                    divider: true
                    label: I18n.tr("Level lean")
                    desc: I18n.tr("Return both perspective axes to zero")
                    controlWidth: Tokens.s7 + Tokens.s5
                    Btn {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("Level")
                        compact: true
                        onAct: page.cfg.levelTilt()
                    }
                }
                SettingRow {
                    visible: page.cfg.count >= 2
                    width: parent.width
                    divider: true
                    label: I18n.tr("Render memory")
                    desc: I18n.tr("Each instance is one full-screen pass")
                    value: "~" + page.cfg.ramEstimateMB + " MB"
                }
            }

            SettingCard {
                visible: page.tab === "Shape"
                width: parent.width
                title: I18n.tr("SHAPE")
                kana: "形"
                collapsible: false
                StepRow {
                    label: I18n.tr("Bands")
                    desc: I18n.tr("Frequency detail")
                    amount: page.inst.bars
                    minimum: 16
                    maximum: 128
                    stride: 4
                    writeValue: value => page.cfg.setBars(value)
                }
                SegmentRow {
                    visible: page.shapeSet
                    divider: true
                    label: I18n.tr("Corners")
                    desc: I18n.tr("Band edge treatment")
                    choices: ["rounded", "flat"]
                    chosen: page.inst.shape
                    writeValue: value => page.cfg.poke("shape", value)
                }
                StepRow {
                    visible: page.segSet
                    divider: true
                    label: I18n.tr("Segments")
                    desc: I18n.tr("Cells in each frequency band")
                    amount: page.inst.segments
                    minimum: 3
                    maximum: 24
                    writeValue: value => page.cfg.poke("segments", value)
                }
                SliderRow {
                    visible: page.shapeSet
                    divider: true
                    label: I18n.tr("Bar width")
                    desc: I18n.tr("Weight of each band")
                    amount: page.inst.thickness
                    minimum: 0.2
                    maximum: 1
                    writeValue: value => page.cfg.poke("thickness", value)
                }
                ChipsRow {
                    visible: page.growSet
                    divider: true
                    label: I18n.tr("Grows")
                    desc: I18n.tr("Direction the signal expands")
                    choices: ["up", "down", "center", "left", "right"]
                    chosen: page.inst.grow
                    writeValue: value => page.cfg.poke("grow", value)
                }
                StepRow {
                    visible: page.polarSet
                    divider: true
                    label: I18n.tr("Rotation")
                    desc: I18n.tr("Continuous polar motion")
                    amount: Math.round(page.inst.spin)
                    minimum: -30
                    maximum: 30
                    writeValue: value => page.cfg.poke("spin", value)
                }
                SliderRow {
                    visible: page.reflSet
                    divider: true
                    label: I18n.tr("Reflection")
                    desc: I18n.tr("Faint signal below the baseline")
                    amount: page.inst.reflection
                    maximum: 0.3
                    writeValue: value => page.cfg.poke("reflection", value)
                }
                SliderRow {
                    visible: page.bloomSet
                    divider: true
                    label: I18n.tr("Bloom")
                    desc: I18n.tr("Soft light around the signal")
                    amount: page.inst.bloom
                    writeValue: value => page.cfg.poke("bloom", value)
                }
                SwitchRow {
                    visible: page.cfg.mirrorApplies
                    divider: true
                    label: I18n.tr("Mirror")
                    desc: I18n.tr("Reflect the signal across its axis")
                    onState: page.inst.mirror
                    writeValue: value => page.cfg.poke("mirror", value)
                }
                SwitchRow {
                    visible: page.cfg.peaksApply
                    divider: true
                    label: I18n.tr("Peaks")
                    desc: I18n.tr("Hold the highest recent level")
                    onState: page.inst.peaks
                    writeValue: value => page.cfg.poke("peaks", value)
                }
                SettingRow {
                    width: parent.width
                    divider: true
                    label: I18n.tr("Flip")
                    desc: I18n.tr("Reverse the direction for this look")
                    controlWidth: Tokens.s7 + Tokens.s4
                    Btn {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("Flip")
                        compact: true
                        onAct: page.cfg.flip()
                    }
                }
            }

            SettingCard {
                visible: page.aura && page.tab === "Field"
                width: parent.width
                title: I18n.tr("FIELD")
                kana: "光"
                collapsible: false
                SettingRow {
                    width: parent.width
                    label: I18n.tr("Edges")
                    desc: I18n.tr("Screen rails that carry the field")
                    block: true
                    Flow {
                        width: parent.width
                        spacing: Tokens.s2
                        Repeater {
                            model: ["top", "right", "bottom", "left"]
                            delegate: Btn {
                                required property string modelData
                                text: page.cap(modelData)
                                compact: true
                                primary: (page.inst.auraEdges || []).indexOf(modelData) >= 0
                                onAct: page.cfg.toggleAuraEdge(modelData)
                            }
                        }
                    }
                }
                ChipsRow {
                    divider: true
                    label: I18n.tr("Movement")
                    desc: I18n.tr("Structure of the flowing body")
                    choices: ["flow", "ribbon", "cells", "filament"]
                    chosen: page.inst.auraShape
                    writeValue: value => page.cfg.poke("auraShape", value)
                }
                ChipsRow {
                    divider: true
                    label: I18n.tr("Effect")
                    desc: I18n.tr("Surface treatment over the field")
                    choices: ["clean", "shimmer", "echo", "prism", "bloom", "caustic", "afterglow"]
                    chosen: page.inst.auraEffect
                    writeValue: value => page.cfg.poke("auraEffect", value)
                }
                SliderRow {
                    divider: true
                    label: I18n.tr("Effect strength")
                    desc: I18n.tr("Amount of surface treatment")
                    amount: page.inst.auraEffectStrength
                    writeValue: value => page.cfg.poke("auraEffectStrength", value)
                }
                SegmentRow {
                    divider: true
                    label: I18n.tr("Colour mode")
                    desc: I18n.tr("How colours travel through the field")
                    choices: ["flow", "spectrum", "pulse", "static"]
                    chosen: page.inst.auraColorMode
                    writeValue: value => page.cfg.poke("auraColorMode", value)
                }
                SliderRow {
                    divider: true
                    label: I18n.tr("Colour drift")
                    desc: I18n.tr("Speed of colour travel")
                    amount: page.inst.auraColorSpeed
                    writeValue: value => page.cfg.poke("auraColorSpeed", value)
                }
                SegmentRow {
                    divider: true
                    label: I18n.tr("Corners")
                    desc: I18n.tr("Join or separate adjacent rails")
                    choices: ["auto", "separate"]
                    chosen: page.inst.auraJoin
                    writeValue: value => page.cfg.poke("auraJoin", value)
                }
                SegmentRow {
                    divider: true
                    label: I18n.tr("Flow")
                    desc: I18n.tr("Direction around the screen")
                    choices: ["clockwise", "counterclockwise"]
                    choiceLabels: ({ clockwise: I18n.tr("Clockwise"), counterclockwise: I18n.tr("Counter") })
                    chosen: page.inst.auraFlow
                    writeValue: value => page.cfg.poke("auraFlow", value)
                }
                SliderRow {
                    divider: true
                    label: I18n.tr("Span")
                    desc: I18n.tr("Length of each lit rail")
                    amount: page.inst.auraSpan
                    minimum: 0.2
                    writeValue: value => page.cfg.poke("auraSpan", value)
                }
                SliderRow {
                    divider: true
                    label: I18n.tr("Taper")
                    desc: I18n.tr("Fade at each rail end")
                    amount: page.inst.auraTaper
                    maximum: 0.5
                    writeValue: value => page.cfg.poke("auraTaper", value)
                }
                StepRow {
                    divider: true
                    label: I18n.tr("Corner radius")
                    desc: I18n.tr("Roundness at joined edges")
                    amount: Math.round(page.inst.auraCornerRadius)
                    minimum: 0
                    maximum: 64
                    writeValue: value => page.cfg.poke("auraCornerRadius", value)
                }
                SliderRow {
                    divider: true
                    label: I18n.tr("Corner blend")
                    desc: I18n.tr("Smoothness through each join")
                    amount: page.inst.auraCornerBlend
                    writeValue: value => page.cfg.poke("auraCornerBlend", value)
                }
                ChipsRow {
                    divider: true
                    label: I18n.tr("Frequency profile")
                    desc: I18n.tr("Bias the field toward a part of the mix")
                    choices: ["flat", "bass", "warm", "vocal", "treble", "smile"]
                    chosen: page.inst.auraProfile
                    writeValue: value => page.cfg.poke("auraProfile", value)
                }
                SliderRow {
                    divider: true
                    label: I18n.tr("Profile accent")
                    desc: I18n.tr("Strength of the frequency bias")
                    amount: page.inst.auraAccent
                    writeValue: value => page.cfg.poke("auraAccent", value)
                }
                SliderRow {
                    divider: true
                    label: I18n.tr("Field opacity")
                    desc: I18n.tr("Overall visibility")
                    amount: page.inst.auraOpacity
                    minimum: 0.2
                    writeValue: value => page.cfg.poke("auraOpacity", value)
                }
                SliderRow {
                    divider: true
                    label: I18n.tr("Body opacity")
                    desc: I18n.tr("Fill behind the crest")
                    amount: page.inst.auraBodyOpacity
                    writeValue: value => page.cfg.poke("auraBodyOpacity", value)
                }
                SliderRow {
                    divider: true
                    label: I18n.tr("Crest strength")
                    desc: I18n.tr("Brightness along the leading edge")
                    amount: page.inst.auraCrestStrength
                    writeValue: value => page.cfg.poke("auraCrestStrength", value)
                }
                SliderRow {
                    divider: true
                    label: I18n.tr("Glow")
                    desc: I18n.tr("Light around the body")
                    amount: page.inst.auraGlow
                    writeValue: value => page.cfg.poke("auraGlow", value)
                }
                SliderRow {
                    divider: true
                    label: I18n.tr("Glow spread")
                    desc: I18n.tr("Reach of the soft light")
                    amount: page.inst.auraGlowSpread
                    writeValue: value => page.cfg.poke("auraGlowSpread", value)
                }
                SliderRow {
                    divider: true
                    label: I18n.tr("Audio range")
                    desc: I18n.tr("Share of the spectrum that drives the field")
                    amount: page.inst.auraAudioRange
                    writeValue: value => page.cfg.poke("auraAudioRange", value)
                }
                SliderRow {
                    divider: true
                    label: I18n.tr("Body width")
                    desc: I18n.tr("Thickness of the current")
                    amount: page.inst.auraThickness
                    minimum: 0.05
                    maximum: 0.6
                    writeValue: value => page.cfg.poke("auraThickness", value)
                }
                SliderRow {
                    divider: true
                    label: I18n.tr("Detail")
                    desc: I18n.tr("Fine structure in the current")
                    amount: page.inst.auraDetail
                    writeValue: value => page.cfg.poke("auraDetail", value)
                }
                SliderRow {
                    divider: true
                    label: I18n.tr("Bass drive")
                    desc: I18n.tr("Low-frequency response")
                    amount: page.inst.auraBassDrive
                    maximum: 1.5
                    writeValue: value => page.cfg.poke("auraBassDrive", value)
                }
                SliderRow {
                    divider: true
                    label: I18n.tr("Treble drive")
                    desc: I18n.tr("High-frequency response")
                    amount: page.inst.auraTrebleDrive
                    maximum: 1.5
                    writeValue: value => page.cfg.poke("auraTrebleDrive", value)
                }
                SliderRow {
                    divider: true
                    label: I18n.tr("Transient kick")
                    desc: I18n.tr("Response to sharp attacks")
                    amount: page.inst.auraTransient
                    writeValue: value => page.cfg.poke("auraTransient", value)
                }
                SliderRow {
                    divider: true
                    label: I18n.tr("Beat glow")
                    desc: I18n.tr("Light added on detected beats")
                    amount: page.inst.auraBeatGlow
                    writeValue: value => page.cfg.poke("auraBeatGlow", value)
                }
                SliderRow {
                    divider: true
                    label: I18n.tr("Compression")
                    desc: I18n.tr("Even out loud and quiet passages")
                    amount: page.inst.auraCompression
                    writeValue: value => page.cfg.poke("auraCompression", value)
                }
                SliderRow {
                    divider: true
                    label: I18n.tr("Sensitivity")
                    desc: I18n.tr("Input level for the field")
                    amount: page.inst.auraSensitivity
                    maximum: 2
                    writeValue: value => page.cfg.poke("auraSensitivity", value)
                }
                SliderRow {
                    divider: true
                    label: I18n.tr("Motion speed")
                    desc: I18n.tr("Travel speed of the current")
                    amount: page.inst.auraMotionSpeed
                    maximum: 3
                    writeValue: value => page.cfg.poke("auraMotionSpeed", value)
                }
                SliderRow {
                    divider: true
                    label: I18n.tr("Idle drift")
                    desc: I18n.tr("Motion while audio is quiet")
                    amount: page.inst.auraIdleMotion
                    writeValue: value => page.cfg.poke("auraIdleMotion", value)
                }
                SliderRow {
                    divider: true
                    label: I18n.tr("Attack")
                    desc: I18n.tr("How quickly the field rises")
                    amount: page.inst.auraAttack
                    minimum: 0.2
                    maximum: 3
                    shown: amount.toFixed(2)
                    writeValue: value => page.cfg.poke("auraAttack", value)
                }
                SliderRow {
                    divider: true
                    label: I18n.tr("Release")
                    desc: I18n.tr("How slowly the field settles")
                    amount: page.inst.auraRelease
                    minimum: 0.2
                    maximum: 3
                    shown: amount.toFixed(2)
                    writeValue: value => page.cfg.poke("auraRelease", value)
                }
                SegmentRow {
                    divider: true
                    label: I18n.tr("Material")
                    desc: I18n.tr("Surface character of the field")
                    choices: page.cfg.auraMaterials
                    chosen: page.inst.auraMaterial
                    writeValue: value => page.cfg.poke("auraMaterial", value)
                }
                SliderRow {
                    divider: true
                    label: I18n.tr("Reach")
                    desc: I18n.tr("Depth of the field into the desktop")
                    amount: page.inst.auraDepth
                    minimum: 24
                    maximum: 600
                    shown: Math.round(amount) + "px"
                    writeValue: value => page.cfg.setAuraDepth(value)
                }
            }
        }
    }

    Component.onCompleted: {
        if (page.cfg.enabled)
            page.selectInstance(page.cfg.active);
    }
    Component.onDestruction: page.finishPlace()
}
