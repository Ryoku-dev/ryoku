pragma ComponentBehavior: Bound

import QtQuick
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

    Column {
        id: content
        width: root.width
        spacing: K.Style.s5

        P.PageHead {
            width: parent.width
            title: "Sound"
            description: "Tune how the bar's edge answers music and settles again."
        }

        SettingCard {
            width: parent.width
            title: "WAVE RESPONSE"
            collapsible: false

            SettingRow {
                width: parent.width
                label: "Gain"
                desc: "Set how much the edge reacts to sound."
                value: String(Math.round(S.Prefs.gain * 100))
                unit: "%"
                controlWidth: Math.min(250, width * 0.42)

                K.Slider {
                    anchors.fill: parent
                    from: 0
                    to: 3
                    stepSize: 0.01
                    value: S.Prefs.gain
                    onMoved: value => S.Prefs.preview("gain", value)
                    onCommitted: value => root.commitPreview("gain", value)
                }
            }
        }

        SettingCard {
            width: parent.width
            title: "MOVEMENT"
            collapsible: false

            SettingRow {
                width: parent.width
                label: "Depth"
                desc: "Set how far sound can pull the lower edge."
                value: String(Math.round(S.Prefs.depth))
                unit: "px"
                controlWidth: 58

                Step {
                    anchors.right: parent.right
                    value: Math.round(S.Prefs.depth)
                    from: 8
                    to: 40
                    stepBy: 1
                    onModified: value => S.Prefs.set("depth", value)
                }
            }

            SettingRow {
                width: parent.width
                label: "Mirror"
                desc: "Balance the wave around its centre."
                controlWidth: 54
                divider: true

                K.Switch {
                    anchors.right: parent.right
                    checked: S.Prefs.mirror
                    onToggled: value => S.Prefs.set("mirror", value)
                }
            }

            SettingRow {
                width: parent.width
                label: "Feel"
                desc: "Choose how quickly the edge settles."
                controlWidth: Math.min(190, width * 0.38)
                divider: true

                K.Segmented {
                    anchors.fill: parent
                    model: [
                        { value: "fluid", label: "Fluid", icon: "water" },
                        { value: "tight", label: "Tight", icon: "speed" }
                    ]
                    value: S.Prefs.feel
                    onPicked: value => S.Prefs.set("feel", value)
                }
            }
        }
    }
}
