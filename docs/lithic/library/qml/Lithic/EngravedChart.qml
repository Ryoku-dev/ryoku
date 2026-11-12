import QtQuick
import QtQuick.Templates as T

T.Control {
    id: root

    property var values: []
    property var ticks: []
    property real minimum: Number.NaN
    property real maximum: Number.NaN
    property string emptyText: Lithic.tr("Not enough data yet")
    property color lineInk: Lithic.fg
    property color baselineInk: Lithic.vein
    property string previewState: ""
    readonly property bool hasSeries: values && values.length >= 2

    implicitWidth: LithicTokens.size.dialogWidth
    implicitHeight: LithicTokens.space.s6 * 2 + Lithic.type.monoSm.lineHeight
    padding: 0
    Accessible.role: Accessible.Chart
    Accessible.name: Lithic.tr("Data chart")

    function rangeMinimum() {
        if (!Number.isNaN(minimum))
            return minimum
        let result = values.length > 0 ? Number(values[0]) : 0
        for (let index = 1; index < values.length; ++index)
            result = Math.min(result, Number(values[index]))
        return result
    }

    function rangeMaximum() {
        if (!Number.isNaN(maximum))
            return maximum
        let result = values.length > 0 ? Number(values[0]) : 1
        for (let index = 1; index < values.length; ++index)
            result = Math.max(result, Number(values[index]))
        return result
    }

    function requestChartPaint() {
        if (chart)
            chart.requestPaint()
    }

    onValuesChanged: requestChartPaint()
    onMinimumChanged: requestChartPaint()
    onMaximumChanged: requestChartPaint()
    onLineInkChanged: requestChartPaint()
    onBaselineInkChanged: requestChartPaint()

    background: null

    contentItem: Item {
        Canvas {
            id: chart
            visible: root.hasSeries
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.bottom: tickRow.top

            onWidthChanged: requestPaint()
            onHeightChanged: requestPaint()
            onPaint: {
                const context = getContext("2d")
                context.reset()
                if (!root.hasSeries || width <= 0 || height <= 0)
                    return

                const low = root.rangeMinimum()
                const highValue = root.rangeMaximum()
                const high = highValue <= low ? low + 1 : highValue
                const baselineY = height - LithicTokens.size.chartBaseline
                const points = []
                for (let index = 0; index < root.values.length; ++index) {
                    const fractionX = index / (root.values.length - 1)
                    const fractionY = (Number(root.values[index]) - low) / (high - low)
                    points.push({
                        x: fractionX * width,
                        y: baselineY - fractionY * Math.max(0, baselineY)
                    })
                }

                context.strokeStyle = root.baselineInk
                context.lineWidth = LithicTokens.size.chartBaseline
                context.beginPath()
                context.moveTo(0, baselineY)
                context.lineTo(width, baselineY)
                context.stroke()

                context.save()
                context.beginPath()
                context.moveTo(points[0].x, points[0].y)
                for (let index = 1; index < points.length; ++index)
                    context.lineTo(points[index].x, points[index].y)
                context.lineTo(points[points.length - 1].x, baselineY)
                context.lineTo(points[0].x, baselineY)
                context.closePath()
                context.clip()
                context.strokeStyle = Lithic.alpha(root.lineInk, LithicTokens.alpha.hatch)
                context.lineWidth = LithicTokens.size.chartHatchLine
                for (let offset = -height; offset < width + height;
                     offset += LithicTokens.size.chartHatchGap) {
                    context.beginPath()
                    context.moveTo(offset, height)
                    context.lineTo(offset + height, 0)
                    context.stroke()
                }
                context.restore()

                context.strokeStyle = root.lineInk
                context.lineWidth = LithicTokens.size.chartLine
                context.lineJoin = "miter"
                context.lineCap = "butt"
                context.beginPath()
                context.moveTo(points[0].x, points[0].y)
                for (let index = 1; index < points.length; ++index)
                    context.lineTo(points[index].x, points[index].y)
                context.stroke()
            }
        }

        Label {
            visible: !root.hasSeries
            anchors.centerIn: parent
            text: root.emptyText
            typeRole: "body"
            inkRole: "fg3"
        }

        Item {
            id: tickRow
            visible: root.hasSeries && root.ticks.length > 0
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            height: visible
                    ? LithicTokens.size.chartTick + Lithic.type.monoSm.lineHeight : 0

            Repeater {
                model: root.ticks
                delegate: Item {
                    required property var modelData
                    required property int index
                    readonly property real tickX: root.ticks.length > 1
                                                  ? index * tickRow.width / (root.ticks.length - 1)
                                                  : tickRow.width / 2
                    x: Math.max(0, Math.min(tickRow.width - width,
                                            tickX - width / 2))
                    width: tickLabel.implicitWidth
                    height: parent.height

                    CutRect {
                        anchors.horizontalCenter: parent.horizontalCenter
                        width: LithicTokens.size.hairline
                        height: LithicTokens.size.chartTick
                        fill: Lithic.vein
                        cut: 0
                    }
                    Label {
                        id: tickLabel
                        anchors.top: parent.top
                        anchors.topMargin: LithicTokens.size.chartTick
                        text: String(parent.modelData)
                        typeRole: "monoSm"
                        inkRole: "fg3"
                    }
                }
            }
        }
    }
}
