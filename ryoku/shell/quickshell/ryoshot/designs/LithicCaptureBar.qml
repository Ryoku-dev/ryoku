pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import QtQuick.Shapes
import Lithic
import Ryoku.Ui.Singletons
import ".."
import "../look"

// Lithic's capture bar is a rule laid along the output's bottom edge: flat where
// it meets the edge, its top corners cut, a hairline over the top and down the
// sides. Fine ticks are engraved along its top, and the ones over the chosen
// action are lit. Mono slugs name the actions with their keys; the target, the
// record options and the tools follow in small cells. It rises from under the
// edge, its ticks light left to right, one shine crosses it, and it drops back.
Item {
    id: root

    required property Item bar
    readonly property real u: root.bar.u
    readonly property real reveal: root.rise
    readonly property rect plane: Qt.rect(rule.x, root.height - rule.height, rule.width, rule.height)

    readonly property real cut: 8 * root.u
    readonly property real hairline: Math.max(1, Math.round(root.u))
    readonly property real pad: 10 * root.u
    readonly property real cell: 26
    readonly property real cellH: 24

    property real rise: 0
    Behavior on rise {
        id: riseMotion
        NumberAnimation {
            duration: Look.dur(riseMotion.targetValue > 0.5 ? 200 : 150)
            easing.type: Easing.BezierSpline
            easing.bezierCurve: riseMotion.targetValue > 0.5 ? Look.decelerate : Look.accelerate
        }
    }

    // the ticks light once per entry, then the shine crosses
    property real sweep: 1
    property real shine: -0.3
    SequentialAnimation {
        id: landing
        PropertyAction { target: root; property: "sweep"; value: 0 }
        PropertyAction { target: root; property: "shine"; value: -0.3 }
        PauseAnimation { duration: 90 }
        NumberAnimation { target: root; property: "sweep"; from: 0; to: 1; duration: 120; easing.type: Easing.Linear }
        NumberAnimation { target: root; property: "shine"; from: -0.3; to: 1.3; duration: 720; easing.type: Easing.OutCubic }
    }
    function land() {
        if (!root.bar.shown)
            return;
        if (Look.reduceMotion) {
            landing.stop();
            root.sweep = 1;
            root.shine = -0.3;
        } else {
            landing.restart();
        }
    }
    Connections {
        target: root.bar
        function onShownChanged() {
            root.land();
        }
    }
    Component.onCompleted: {
        root.rise = Qt.binding(() => root.bar.shown ? 1 : 0);
        root.land();
    }

    // the ticks over the chosen slug
    property real litX: 0
    property real litW: 0
    Behavior on litX {
        NumberAnimation { duration: Look.dur(160); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.curve }
    }
    Behavior on litW {
        NumberAnimation { duration: Look.dur(160); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.curve }
    }

    Item {
        id: rule

        width: Math.ceil(content.implicitWidth + root.pad * 2)
        height: Math.round(36 * root.u)
        x: Math.round((root.width - width) / 2)
        y: Math.round(root.height - height * root.rise)
        enabled: root.bar.shown

        // nothing under the rule hears a click meant for it
        MouseArea {
            anchors.fill: parent
            hoverEnabled: true
        }

        RectangularShadow {
            anchors.fill: parent
            anchors.leftMargin: root.cut
            anchors.rightMargin: root.cut
            offset.y: -2 * root.u
            blur: 12 * root.u
            radius: root.cut
            color: Look.shadow
            opacity: 0.6
        }

        CutRect {
            anchors.fill: parent
            design: "lithic"
            cutTopLeft: root.cut
            cutTopRight: root.cut
            cutBottomLeft: 0
            cutBottomRight: 0
            fill: Look.planes.pane
        }

        Shape {
            anchors.fill: parent
            preferredRendererType: Shape.CurveRenderer
            ShapePath {
                strokeColor: Look.planes.paneStroke
                strokeWidth: root.hairline
                fillColor: "transparent"
                PathPolyline {
                    path: {
                        const w = rule.width, t = rule.height, c = root.cut, h = root.hairline / 2;
                        return [Qt.point(h, t), Qt.point(h, c), Qt.point(c, h),
                            Qt.point(w - c, h), Qt.point(w - h, c), Qt.point(w - h, t)];
                    }
                }
            }
        }

        // ticks every 8 px along the top, every fifth taller
        Repeater {
            model: Math.max(0, Math.floor((rule.width - root.cut * 2) / (8 * root.u)) + 1)
            delegate: Rectangle {
                id: tick
                required property int index
                readonly property real at: root.cut + tick.index * 8 * root.u
                readonly property bool lit: tick.at >= root.litX && tick.at <= root.litX + root.litW
                x: Math.round(tick.at)
                y: root.hairline * 2
                width: root.hairline
                height: Math.round((tick.index % 5 === 0 ? 5 : 3) * root.u)
                color: tick.lit ? Look.sel : Look.vein
                opacity: tick.at / rule.width <= root.sweep ? 1 : 0
                Behavior on color {
                    ColorAnimation { duration: Look.dur(120) }
                }
            }
        }

        // one engraved shine, inside the rule's outline
        Shape {
            anchors.fill: parent
            visible: root.shine > -0.25 && root.shine < 1.25
            preferredRendererType: Shape.CurveRenderer
            ShapePath {
                strokeColor: "transparent"
                fillGradient: LinearGradient {
                    x1: root.shine * rule.width - 26 * root.u
                    y1: 0
                    x2: root.shine * rule.width + 26 * root.u
                    y2: rule.height
                    GradientStop { position: 0; color: "transparent" }
                    GradientStop { position: 0.5; color: Look.dark ? Lithic.alpha(Look.fg, 0.08) : Lithic.alpha(Look.stone4, 0.9) }
                    GradientStop { position: 1; color: "transparent" }
                }
                PathPolyline {
                    path: [Qt.point(0, rule.height), Qt.point(0, root.cut), Qt.point(root.cut, 0),
                        Qt.point(rule.width - root.cut, 0), Qt.point(rule.width, root.cut),
                        Qt.point(rule.width, rule.height), Qt.point(0, rule.height)]
                }
            }
        }

        Row {
            id: content
            x: root.pad
            anchors.verticalCenter: parent.verticalCenter
            anchors.verticalCenterOffset: 2 * root.u
            spacing: 4 * root.u

            Row {
                id: slugs
                anchors.verticalCenter: parent.verticalCenter
                spacing: 2 * root.u

                Repeater {
                    model: root.bar.actions
                    delegate: Item {
                        id: slug
                        required property var modelData
                        required property int index
                        readonly property bool current: root.bar.activeAction === slug.modelData.id
                        readonly property bool rec: slug.modelData.id === "record"

                        width: Math.ceil(slugText.implicitWidth + 14 * root.u)
                        height: Math.round(root.cellH * root.u)

                        Binding {
                            when: slug.current
                            target: root
                            property: "litX"
                            value: content.x + slugs.x + slug.x
                        }
                        Binding {
                            when: slug.current
                            target: root
                            property: "litW"
                            value: slug.width
                        }

                        CutRect {
                            anchors.fill: parent
                            design: "lithic"
                            cut: (Look.cutSm + (slugArea.pressed ? Lithic.pressCutGrow : 0)) * root.u
                            fill: slug.current ? Look.sel : slugArea.containsMouse ? Look.hover : Lithic.alpha(Look.sel, 0)
                            Behavior on fill {
                                ColorAnimation { duration: Look.dur(Look.stateMs) }
                            }
                        }

                        Row {
                            id: slugText
                            anchors.centerIn: parent
                            anchors.verticalCenterOffset: slugArea.pressed ? Lithic.pressSink * root.u : 0
                            spacing: 4 * root.u

                            Text {
                                anchors.baseline: label.baseline
                                text: slug.index + 1
                                color: slug.current ? Lithic.alpha(Look.onSel, 0.62) : Look.fg3
                                font.family: Look.mono
                                font.pixelSize: Math.round(8 * root.u)
                                renderType: Text.CurveRendering
                            }
                            Rectangle {
                                visible: slug.current && slug.rec
                                anchors.verticalCenter: parent.verticalCenter
                                width: Math.round(6 * root.u)
                                height: width
                                radius: width / 2
                                color: Look.jasper
                                border.width: Math.max(1, Math.round(root.u))
                                border.color: Look.onSel
                            }
                            Text {
                                id: label
                                anchors.verticalCenter: parent.verticalCenter
                                text: String(slug.modelData.short || slug.modelData.label).toUpperCase()
                                color: slug.current ? Look.onSel : slugArea.containsMouse ? Look.fg : Look.fg2
                                font.family: Look.mono
                                font.pixelSize: Math.round(10.5 * root.u)
                                font.weight: Font.Bold
                                font.letterSpacing: Look.tracking * root.u
                                renderType: Text.CurveRendering
                                Behavior on color {
                                    ColorAnimation { duration: Look.dur(Look.stateMs) }
                                }
                            }
                        }

                        MouseArea {
                            id: slugArea
                            anchors.fill: parent
                            hoverEnabled: true
                            cursorShape: Qt.PointingHandCursor
                            onClicked: root.bar.actionPicked(slug.modelData.id)
                        }
                    }
                }
            }

            Divider { u: root.u; length: 16; pad: 3; anchors.verticalCenter: parent.verticalCenter }

            Row {
                anchors.verticalCenter: parent.verticalCenter
                spacing: 2 * root.u

                GlyphButton {
                    u: root.u; cellWidth: root.cell; cellHeight: root.cellH; iconSize: 15
                    icon: "region"
                    checked: root.bar.target === "region"
                    tip: I18n.tr("Region"); keyHint: I18n.tr("Space")
                    onClicked: root.bar.targetPicked("region")
                }
                GlyphButton {
                    visible: root.bar.windowTarget
                    u: root.u; cellWidth: root.cell; cellHeight: root.cellH; iconSize: 15
                    icon: "window"
                    checked: root.bar.target === "window"
                    tip: I18n.tr("Window"); keyHint: I18n.tr("Space")
                    onClicked: root.bar.targetPicked("window")
                }
                GlyphButton {
                    u: root.u; cellWidth: root.cell; cellHeight: root.cellH; iconSize: 15
                    icon: "screen"
                    checked: root.bar.target === "monitor"
                    tip: I18n.tr("Screen"); keyHint: I18n.tr("Space")
                    onClicked: root.bar.targetPicked("monitor")
                }
            }

            // the record options slide in beside the targets while Record is chosen
            Item {
                id: recordDrawer
                anchors.verticalCenter: parent.verticalCenter
                height: recordRow.implicitHeight
                width: root.bar.recording ? recordRow.implicitWidth : 0
                clip: width < recordRow.implicitWidth
                opacity: root.bar.recording ? 1 : 0
                visible: width > 0
                Behavior on width {
                    NumberAnimation { duration: Look.dur(180); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.decelerate }
                }
                Behavior on opacity {
                    NumberAnimation { duration: Look.dur(140) }
                }

                Row {
                    id: recordRow
                    spacing: 2 * root.u

                    Divider { u: root.u; length: 16; pad: 3; anchors.verticalCenter: parent.verticalCenter }
                    GlyphButton {
                        u: root.u; cellWidth: root.cell; cellHeight: root.cellH; iconSize: 15
                        icon: "sound"; checked: root.bar.audioOn
                        checkedFill: Look.container; checkedInk: Look.containerInk
                        tip: I18n.tr("Record system audio")
                        onClicked: root.bar.audioToggled()
                    }
                    GlyphButton {
                        u: root.u; cellWidth: root.cell; cellHeight: root.cellH; iconSize: 15
                        icon: "mic"; checked: root.bar.micOn
                        checkedFill: Look.container; checkedInk: Look.containerInk
                        tip: I18n.tr("Record microphone audio")
                        onClicked: root.bar.micToggled()
                    }
                    GlyphButton {
                        u: root.u; cellWidth: root.cell; cellHeight: root.cellH; iconSize: 15
                        icon: "keyboard"; checked: root.bar.keypressesOn
                        checkedFill: Look.container; checkedInk: Look.containerInk
                        tip: root.bar.keypressesOn ? I18n.tr("Hide key presses") : I18n.tr("Show key presses")
                        onClicked: root.bar.keypressesToggled()
                    }
                    GlyphButton {
                        u: root.u; cellWidth: root.cell; cellHeight: root.cellH; iconSize: 15
                        icon: "webcam"; checked: root.bar.webcamOn
                        checkedFill: Look.container; checkedInk: Look.containerInk
                        tip: root.bar.webcamOn ? I18n.tr("Hide webcam") : I18n.tr("Show webcam")
                        onClicked: root.bar.webcamToggled()
                    }
                }
            }

            Divider { u: root.u; length: 16; pad: 3; anchors.verticalCenter: parent.verticalCenter }

            Row {
                anchors.verticalCenter: parent.verticalCenter
                spacing: 2 * root.u

                // the timer reads its delay in mono beside the glyph
                Item {
                    id: timer
                    readonly property bool set: root.bar.delaySeconds > 0
                    width: Math.round(root.cell * root.u) + (timer.set ? Math.ceil(delayText.implicitWidth + 4 * root.u) : 0)
                    height: Math.round(root.cellH * root.u)
                    Behavior on width {
                        NumberAnimation { duration: Look.dur(140); easing.type: Easing.OutCubic }
                    }

                    CutRect {
                        anchors.fill: parent
                        design: "lithic"
                        cut: Look.cutSm * root.u
                        fill: timerArea.pressed ? Look.press : timerArea.containsMouse ? Look.hover : Lithic.alpha(Look.hover, 0)
                    }
                    Icon {
                        x: Math.round((root.cell * root.u - width) / 2)
                        anchors.verticalCenter: parent.verticalCenter
                        name: "timer"
                        size: 15 * root.u
                        tint: timer.set ? Look.sel : timerArea.containsMouse ? Look.fg : Look.fg2
                    }
                    Text {
                        id: delayText
                        visible: timer.set
                        x: Math.round(root.cell * root.u - 3 * root.u)
                        anchors.verticalCenter: parent.verticalCenter
                        text: root.bar.delaySeconds + "s"
                        color: Look.sel
                        font.family: Look.mono
                        font.pixelSize: Math.round(10 * root.u)
                        font.weight: Font.Bold
                        renderType: Text.CurveRendering
                    }
                    MouseArea {
                        id: timerArea
                        anchors.fill: parent
                        hoverEnabled: true
                        cursorShape: Qt.PointingHandCursor
                        onClicked: root.bar.delayCycled()
                    }
                    Tip {
                        z: 100
                        u: root.u
                        anchors.horizontalCenter: parent.horizontalCenter
                        y: -height - 8 * root.u
                        text: root.bar.delaySeconds === 0 ? I18n.tr("No delay") : I18n.tr("%1-second delay").arg(root.bar.delaySeconds)
                        shown: timerArea.containsMouse
                    }
                }
                GlyphButton {
                    u: root.u; cellWidth: root.cell; cellHeight: root.cellH; iconSize: 15
                    icon: "eyedropper"
                    tip: I18n.tr("Pick a colour"); keyHint: "C"
                    onClicked: root.bar.colorPickRequested()
                }
                GlyphButton {
                    u: root.u; cellWidth: root.cell; cellHeight: root.cellH; iconSize: 15
                    icon: "folder"
                    tip: I18n.tr("Open screenshots folder")
                    onClicked: root.bar.folderRequested()
                }
            }

            Item { width: 2 * root.u; height: 1 }

            // a quiet key plaque: Enter takes the whole screen
            Item {
                id: plaque
                anchors.verticalCenter: parent.verticalCenter
                width: Math.ceil(plaqueRow.implicitWidth + 12 * root.u)
                height: Math.round(20 * root.u)

                CutRect {
                    anchors.fill: parent
                    design: "lithic"
                    cut: Look.cutSm * root.u
                    fill: plaqueArea.containsMouse ? Look.hover : Lithic.alpha(Look.hover, 0)
                    strokeColor: plaqueArea.containsMouse ? Look.edge : Look.vein
                    strokeWidth: root.hairline
                }
                Row {
                    id: plaqueRow
                    anchors.centerIn: parent
                    anchors.verticalCenterOffset: plaqueArea.pressed ? Lithic.pressSink * root.u : 0
                    spacing: 5 * root.u
                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        text: "\u23ce"
                        color: plaqueArea.containsMouse ? Look.fg : Look.fg3
                        font.family: Look.mono
                        font.pixelSize: Math.round(10 * root.u)
                        renderType: Text.CurveRendering
                    }
                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("Screen").toUpperCase()
                        color: plaqueArea.containsMouse ? Look.fg : Look.fg3
                        font.family: Look.mono
                        font.pixelSize: Math.round(9 * root.u)
                        font.weight: Font.Bold
                        font.letterSpacing: Look.tracking * root.u
                        renderType: Text.CurveRendering
                    }
                }
                MouseArea {
                    id: plaqueArea
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: Qt.PointingHandCursor
                    onClicked: root.bar.screenRequested()
                }
                Tip {
                    z: 100
                    u: root.u
                    anchors.horizontalCenter: parent.horizontalCenter
                    y: -height - 8 * root.u
                    text: I18n.tr("Capture this monitor")
                    shown: plaqueArea.containsMouse
                }
            }

            GlyphButton {
                anchors.verticalCenter: parent.verticalCenter
                u: root.u; cellWidth: root.cell; cellHeight: root.cellH; iconSize: 14
                icon: "cancel"
                danger: true
                tip: I18n.tr("Close"); keyHint: "Esc"
                onClicked: root.bar.closeRequested()
            }
        }
    }
}
