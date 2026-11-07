pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Shapes
import Ryoku.Ui
import "../../kit" as K
import ".." as S
import "." as P

Item {
    id: root

    property var scene: null
    property bool shown: false
    property var args: ({})

    implicitHeight: content.implicitHeight
    enabled: shown

    function commitPreview(key, value) {
        S.Prefs.preview(key, value);
        S.Prefs.commit(key);
    }

    component SpanPreview: Item {
        id: preview
        required property string span
        readonly property var drops: span === "islands"
            ? [{ x: 0.04, w: 0.27 }, { x: 0.37, w: 0.26 }, { x: 0.71, w: 0.25 }]
            : span === "fused" ? [{ x: 0.2, w: 0.6 }] : [{ x: 0, w: 1 }]

        anchors.fill: parent

        Rectangle {
            id: screen
            anchors.fill: parent
            anchors.margins: K.Style.s2
            radius: K.Style.radius
            color: K.Style.tint5
            border.width: 1
            border.color: K.Style.lineSoft

            Repeater {
                model: preview.drops

                delegate: Rectangle {
                    required property var modelData
                    readonly property bool band: preview.span === "full"
                    x: Math.round(screen.width * modelData.x)
                    width: Math.round(screen.width * modelData.w)
                    height: 10
                    topLeftRadius: band ? K.Style.radius : 0
                    topRightRadius: band ? K.Style.radius : 0
                    bottomLeftRadius: S.Prefs.form === "square" ? 2 : K.Style.radius
                    bottomRightRadius: S.Prefs.form === "square" ? 2 : K.Style.radius
                    color: K.Style.ink
                }
            }
        }
    }

    component FormPreview: Item {
        id: formPreview
        required property string form
        readonly property bool square: form === "square"
        readonly property var levels: [0.35, 0.6, 0.9, 1, 0.7, 0.45, 0.8, 0.55, 0.3]

        anchors.fill: parent

        Rectangle {
            id: formScreen
            anchors.fill: parent
            anchors.margins: K.Style.s2
            radius: K.Style.radius
            color: K.Style.tint5
            border.width: 1
            border.color: K.Style.lineSoft

            Rectangle {
                id: formDrop
                visible: formPreview.square
                x: Math.round(formScreen.width * 0.2)
                width: Math.round(formScreen.width * 0.6)
                height: 10
                bottomLeftRadius: 2
                bottomRightRadius: 2
                color: K.Style.ink
            }

            // The liquid drop swells at its centre, the way the mirrored wave does.
            Shape {
                visible: !formPreview.square
                x: formDrop.x
                width: formDrop.width
                height: 16
                preferredRendererType: Shape.CurveRenderer

                ShapePath {
                    id: swell
                    readonly property real w: formDrop.width
                    fillColor: K.Style.ink
                    strokeWidth: -1
                    PathSvg {
                        path: {
                            const w = swell.w;
                            return "M 0 0 H " + w + " V 5"
                                + " C " + w + " 8.5 " + (w - 3) + " 10 " + (w * 0.84) + " 10"
                                + " C " + (w * 0.68) + " 10 " + (w * 0.64) + " 15 " + (w * 0.5) + " 15"
                                + " C " + (w * 0.36) + " 15 " + (w * 0.32) + " 10 " + (w * 0.16) + " 10"
                                + " C 3 10 0 8.5 0 5 Z";
                        }
                    }
                }
            }

            Repeater {
                model: formPreview.square ? formPreview.levels : []

                delegate: Rectangle {
                    required property var modelData
                    required property int index
                    readonly property real pitch: formDrop.width * 0.84 / formPreview.levels.length
                    x: Math.round(formDrop.x + formDrop.width * 0.08 + index * pitch)
                    y: formDrop.height - 1
                    width: Math.max(2, Math.round(pitch) - 2)
                    height: Math.round(1 + modelData * 8)
                    bottomLeftRadius: 1
                    bottomRightRadius: 1
                    color: K.Style.ink
                }
            }
        }
    }

    Column {
        id: content
        width: root.width
        spacing: K.Style.s5

        P.PageHead {
            width: parent.width
            title: "Bar"
            description: "Shape the bar and choose when it stays in view."
        }

        SettingCard {
            width: parent.width
            title: "BAR SHAPE"
            collapsible: false

            SettingRow {
                width: parent.width
                label: "Shape"
                desc: "Choose how the bar spans the top edge."
                block: true

                Flow {
                    id: spanFlow
                    readonly property real tileWidth: Math.floor((width - 2 * spacing) / 3)
                    width: parent.width
                    spacing: K.Style.s3

                    Repeater {
                        model: [
                            { value: "islands", label: "Islands" },
                            { value: "fused", label: "Fused" },
                            { value: "full", label: "Full" }
                        ]

                        delegate: K.Tile {
                            id: spanTile
                            required property var modelData
                            width: spanFlow.tileWidth
                            label: modelData.label
                            selected: S.Prefs.span === modelData.value
                            onClicked: S.Prefs.set("span", modelData.value)

                            SpanPreview { span: spanTile.modelData.value }
                        }
                    }
                }
            }

            SettingRow {
                width: parent.width
                label: "Form"
                desc: "Choose a liquid edge, or crisp corners with a block wave."
                block: true
                divider: true

                Flow {
                    id: formFlow
                    readonly property real tileWidth: Math.floor((width - 2 * spacing) / 3)
                    width: parent.width
                    spacing: K.Style.s3

                    Repeater {
                        model: [
                            { value: "liquid", label: "Liquid" },
                            { value: "square", label: "Square" }
                        ]

                        delegate: K.Tile {
                            id: formTile
                            required property var modelData
                            width: formFlow.tileWidth
                            label: modelData.label
                            selected: S.Prefs.form === modelData.value
                            onClicked: S.Prefs.set("form", modelData.value)

                            FormPreview { form: formTile.modelData.value }
                        }
                    }
                }
            }
        }

        SettingCard {
            width: parent.width
            title: "APPEARANCE"
            collapsible: false

            SettingRow {
                width: parent.width
                label: "Size"
                desc: "Scale the bar and its modules together."
                value: String(Math.round(S.Prefs.scale * 100))
                unit: "%"
                controlWidth: Math.min(250, width * 0.42)

                K.Slider {
                    anchors.fill: parent
                    from: 0.85
                    to: 1.3
                    stepSize: 0.01
                    value: S.Prefs.scale
                    onMoved: value => S.Prefs.preview("scale", value)
                    onCommitted: value => root.commitPreview("scale", value)
                }
            }

            SettingRow {
                width: parent.width
                label: "Opacity"
                desc: "Let the wallpaper show through the bar."
                value: String(Math.round(S.Prefs.opacity * 100))
                unit: "%"
                controlWidth: Math.min(250, width * 0.42)
                divider: true

                K.Slider {
                    anchors.fill: parent
                    from: 0.6
                    to: 1
                    stepSize: 0.01
                    value: S.Prefs.opacity
                    onMoved: value => S.Prefs.preview("opacity", value)
                    onCommitted: value => root.commitPreview("opacity", value)
                }
            }

            SettingRow {
                width: parent.width
                label: "Outline"
                desc: "Trace the bar's edge with a subtle rim."
                controlWidth: 54
                divider: true

                K.Switch {
                    anchors.right: parent.right
                    checked: S.Prefs.rim
                    onToggled: value => S.Prefs.set("rim", value)
                }
            }
        }

        SettingCard {
            width: parent.width
            title: "VISIBILITY"
            collapsible: false

            SettingRow {
                width: parent.width
                label: "Auto-hide"
                desc: "Choose when the bar leaves the screen."
                block: true

                K.Segmented {
                    anchors.fill: parent
                    model: [
                        { value: "off", label: "Off", icon: "visibility" },
                        { value: "smart", label: "With windows", icon: "web_asset" },
                        { value: "always", label: "Always", icon: "visibility_off" }
                    ]
                    value: S.Prefs.autoHide
                    onPicked: value => S.Prefs.set("autoHide", value)
                }
            }

            SettingRow {
                width: parent.width
                label: "Peek on hover"
                desc: "Open the voice page after a short pause."
                controlWidth: 54
                divider: true

                K.Switch {
                    anchors.right: parent.right
                    checked: S.Prefs.hoverOpen
                    onToggled: value => S.Prefs.set("hoverOpen", value)
                }
            }

            SettingRow {
                width: parent.width
                label: "Switch pages on hover"
                desc: "Move to another module while a page is open."
                controlWidth: 54
                divider: true

                K.Switch {
                    anchors.right: parent.right
                    checked: S.Prefs.hoverSwitch
                    onToggled: value => S.Prefs.set("hoverSwitch", value)
                }
            }
        }
    }
}
