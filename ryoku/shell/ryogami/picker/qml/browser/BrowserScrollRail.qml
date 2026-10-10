import QtQuick
import Ryoku.Ui.Singletons

Item {
    id: rail

    property var field: null
    readonly property real extent: field ? Math.max(0, Number(field.scrollExtent)) : 0
    readonly property real position: field ? Math.max(0, Math.min(extent, Number(field.scrollPosition))) : 0
    readonly property real viewportRatio: extent <= 0 ? 1 : Math.max(0.12, height / (height + extent))
    readonly property real thumbHeight: Math.max(36 * Theme.scale, height * viewportRatio)
    readonly property real travel: Math.max(0, height - thumbHeight)
    readonly property real thumbY: extent <= 0 ? 0 : travel * position / extent

    visible: extent > 0
    width: 12 * Theme.scale

    Rectangle {
        anchors.horizontalCenter: parent.horizontalCenter
        width: 2 * Theme.scale
        height: parent.height
        radius: width
        color: Theme.withAlpha(Theme.outline, 0.24)
    }

    Rectangle {
        id: thumb
        anchors.horizontalCenter: parent.horizontalCenter
        y: rail.thumbY
        width: dragHandler.active || hover.hovered ? 7 * Theme.scale : 5 * Theme.scale
        height: rail.thumbHeight
        radius: width
        color: Theme.withAlpha(Theme.surfaceText, dragHandler.active ? 0.88 : (hover.hovered ? 0.68 : 0.46))

        Behavior on width { NumberAnimation { duration: Theme.fast; easing.type: Theme.revealEasing } }
        Behavior on color { ColorAnimation { duration: Theme.fast } }

        HoverHandler { id: hover; cursorShape: Qt.PointingHandCursor }
        DragHandler {
            id: dragHandler
            xAxis.enabled: false
            yAxis.minimum: 0
            yAxis.maximum: rail.travel
            onActiveChanged: if (!active) thumb.y = Qt.binding(function() { return rail.thumbY })
            onTranslationChanged: {
                if (!active || !rail.field || rail.travel <= 0)
                    return
                rail.field.setScrollPosition(rail.extent * thumb.y / rail.travel)
            }
        }
    }

    TapHandler {
        acceptedButtons: Qt.LeftButton
        onTapped: function(eventPoint) {
            if (!rail.field || rail.travel <= 0)
                return
            rail.field.setScrollPosition(rail.extent * Math.max(0, Math.min(rail.travel,
                eventPoint.position.y - rail.thumbHeight * 0.5)) / rail.travel)
        }
    }

    WheelHandler {
        onWheel: function(event) {
            if (!rail.field)
                return
            var amount = event.pixelDelta.y !== 0 ? -event.pixelDelta.y : -event.angleDelta.y / 120 * 150 * Theme.scale
            rail.field.scrollBy(amount)
            event.accepted = true
        }
    }
}
