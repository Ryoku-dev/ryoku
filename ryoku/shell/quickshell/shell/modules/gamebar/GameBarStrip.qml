pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import Ryoku.Ui.Singletons
import Lithic
import "look"

Item {
    id: root

    property real s: 1
    property var cards: []
    property var activeIds: []
    property bool snapping: true

    signal picked(string id)
    signal snapToggled()

    readonly property real padding: 8 * root.s
    readonly property real button: 32 * root.s

    implicitWidth: bar.width
    implicitHeight: root.button + root.padding * 2

    property string clock: ""

    function tick() {
        root.clock = TimeFormat.format(new Date());
    }

    Component.onCompleted: root.tick()

    Timer {
        interval: 10000
        running: root.visible && root.enabled
        repeat: true
        onTriggered: root.tick()
    }

    onVisibleChanged: if (root.visible) root.tick()
    onEnabledChanged: if (root.enabled) root.tick()

    Connections {
        target: TimeFormat
        function onIs24hChanged() { root.tick(); }
        function onFormatLocaleChanged() { root.tick(); }
    }

    Item {
        id: bar
        width: row.implicitWidth + root.padding * 2
        height: root.button + root.padding * 2
        readonly property real lithicScale: root.s

        CutRect {
            anchors.fill: parent
            fill: GameBarTone.pane
            strokeColor: GameBarTone.paneStroke
            strokeWidth: LithicTokens.size.hairline
            cut: GameBarTone.paneCut
        }

        Row {
            id: row
            x: root.padding
            y: root.padding
            height: root.button
            spacing: 4 * root.s

            Repeater {
                model: root.cards

                delegate: IconPlaque {
                    id: widgetButton
                    required property var modelData

                    readonly property bool on: root.activeIds.indexOf(widgetButton.modelData.id) !== -1

                    small: true
                    kind: widgetButton.on ? "tonal" : "quiet"
                    glyph: widgetButton.modelData.symbol
                    Accessible.name: widgetButton.modelData.label
                    onClicked: root.picked(widgetButton.modelData.id)
                }
            }

            Item {
                width: LithicTokens.size.hairline
                height: row.height

                Rectangle {
                    anchors.verticalCenter: parent.verticalCenter
                    width: LithicTokens.size.hairline
                    height: 18 * root.s
                    color: GameBarTone.line
                }
            }

            IconPlaque {
                small: true
                kind: root.snapping ? "tonal" : "quiet"
                glyph: "grid_view"
                Accessible.name: I18n.tr("Snapping")
                onClicked: root.snapToggled()
            }

            Item {
                width: clockLabel.implicitWidth
                height: row.height

                Label {
                    id: clockLabel
                    anchors.verticalCenter: parent.verticalCenter
                    leftPadding: 8 * root.s
                    rightPadding: 4 * root.s
                    text: root.clock
                    typeRole: "proof"
                    font.pixelSize: Math.round(22 * root.s)
                    lineHeight: Math.round(26 * root.s)
                    ink: GameBarTone.ink
                    numeric: true
                }
            }

            Item {
                visible: Battery.present
                width: LithicTokens.size.hairline
                height: row.height

                Rectangle {
                    anchors.verticalCenter: parent.verticalCenter
                    width: LithicTokens.size.hairline
                    height: 18 * root.s
                    color: GameBarTone.line
                }
            }

            Item {
                visible: Battery.present
                width: batteryRow.width
                height: row.height

                Row {
                    id: batteryRow
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 6 * root.s

                    GameBarGlyph {
                        brand: "battery"
                        size: 20
                        ink: Battery.low && !Battery.charging ? GameBarTone.danger : GameBarTone.ink2
                    }

                    Item {
                        width: batteryPct.implicitWidth
                        height: batteryRow.height

                        Label {
                            id: batteryPct
                            anchors.verticalCenter: parent.verticalCenter
                            text: Battery.pct + "%"
                            typeRole: "mono"
                            ink: Battery.low && !Battery.charging ? GameBarTone.danger : GameBarTone.ink
                            numeric: true
                        }
                    }
                }
            }
        }
    }
}
