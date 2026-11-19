import QtQuick
import "../../look"
import "../.."

// A studio value on a slider, as the design lays one out: Lithic's tick ruler
// and Material's Expressive slider under their own label and figure, an Adwaita
// row with the scale filling its end, a Fluent SettingsCard whose slider wraps
// under its title. key names the studio property it reads and writes.
Field {
    id: sr

    required property var studio
    required property string key
    property real from: 0
    property real to: 100
    property string suffix: ""
    property bool bipolar: false
    property int decimals: 0

    readonly property real value: sr.studio ? sr.studio[sr.key] : 0
    readonly property bool inline: Look.adwaita
    readonly property bool labelled: Look.lithic || Look.material
    function fmt(v) {
        const n = sr.decimals > 0 ? v.toFixed(sr.decimals) : String(Math.round(v));
        return (sr.bipolar && v > 0.5 ? "+" : "") + n + sr.suffix;
    }
    function set(v) {
        sr.studio[sr.key] = sr.decimals > 0 ? v : Math.round(v);
    }

    title: sr.labelled ? "" : sr.label
    property string label: ""
    fillTrailing: sr.inline
    titleWidth: sr.inline ? Math.round(86 * sr.u) : 0

    Item {
        visible: !sr.labelled
        width: sr.inline ? parent.width : valueText.implicitWidth
        height: sr.inline ? inlineSlider.height : valueText.implicitHeight

        Slider {
            id: inlineSlider
            visible: sr.inline
            width: parent.width - Math.round(40 * sr.u)
            u: sr.u
            labels: false
            detailed: true
            from: sr.from
            to: sr.to
            bipolar: sr.bipolar
            decimals: sr.decimals
            suffix: sr.suffix
            value: sr.value
            onMoved: (v) => sr.set(v)
        }
        Text {
            id: valueText
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            text: sr.fmt(sr.value)
            color: Look.fg2
            font.family: Look.sans
            font.pixelSize: Math.round(13 * sr.u)
            font.features: ({ "tnum": 1 })
            renderType: Text.CurveRendering
        }
    }

    block: Slider {
        visible: !sr.inline
        width: parent ? parent.width : 0
        u: sr.u
        label: sr.label
        labels: sr.labelled
        detailed: true
        from: sr.from
        to: sr.to
        bipolar: sr.bipolar
        decimals: sr.decimals
        suffix: sr.suffix
        value: sr.value
        onMoved: (v) => sr.set(v)
    }
}
