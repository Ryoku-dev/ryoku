import QtQuick
import QtQuick.Controls
import ".."
import Ryoku.Ui.Singletons

Item {
    id: picker

    property var colors
    property string label: ""
    property string value: ""
    property var model: []
    property var families: []
    property int skew: 10
    property real popupWidth: 240 * Config.uiScale
    property real popupMaxHeight: 280 * Config.uiScale
    property bool enabled: true

    signal selected(string key)

    // Resolve a family key ("fade") to its display label ("Fade") from `families`;
    // falls back to a Title-cased key so an unmapped family still reads sanely.
    function familyLabel(key) {
        for (var i = 0; i < families.length; i++) {
            if (families[i].key === key) return I18n.tr(families[i].label)
        }
        return key ? key.charAt(0).toUpperCase() + key.slice(1) : ""
    }

    readonly property string displayValue: {
        for (var i = 0; i < model.length; i++) {
            if (model[i].key === value) return model[i].label
        }
        return value
    }

    width: _contentRow.implicitWidth + 24 * Config.uiScale + skew
    height: 28 * Config.uiScale
    z: _popupOpen ? 100 : (isHovered ? 5 : 1)
    opacity: enabled ? 1.0 : 0.4

    readonly property bool isHovered: _mouse.containsMouse
    property bool _popupOpen: false

    Canvas {
        id: _canvas
        anchors.fill: parent

        property color fillColor: picker._popupOpen
            ? (picker.colors ? Qt.rgba(picker.colors.primary.r, picker.colors.primary.g, picker.colors.primary.b, 0.30) : Qt.rgba(1, 1, 1, 0.22))
            : (picker.isHovered
                ? (picker.colors ? Qt.rgba(picker.colors.surfaceVariant.r, picker.colors.surfaceVariant.g, picker.colors.surfaceVariant.b, 0.6) : Qt.rgba(1, 1, 1, 0.15))
                : (picker.colors ? Qt.rgba(picker.colors.surfaceContainer.r, picker.colors.surfaceContainer.g, picker.colors.surfaceContainer.b, 0.85) : Qt.rgba(0.1, 0.12, 0.18, 0.85)))
        property color strokeColor: picker._popupOpen
            ? (picker.colors ? Qt.rgba(picker.colors.primary.r, picker.colors.primary.g, picker.colors.primary.b, 0.55) : Qt.rgba(1, 1, 1, 0.35))
            : (picker.colors ? Qt.rgba(picker.colors.primary.r, picker.colors.primary.g, picker.colors.primary.b, 0.18) : Qt.rgba(1, 1, 1, 0.08))

        onFillColorChanged: requestPaint()
        onStrokeColorChanged: requestPaint()
        onWidthChanged: requestPaint()

        onPaint: {
            var ctx = getContext("2d")
            ctx.clearRect(0, 0, width, height)
            var sk = picker.skew
            ctx.fillStyle = fillColor
            ctx.beginPath()
            ctx.moveTo(sk, 0)
            ctx.lineTo(width, 0)
            ctx.lineTo(width - sk, height)
            ctx.lineTo(0, height)
            ctx.closePath()
            ctx.fill()
            ctx.strokeStyle = strokeColor
            ctx.lineWidth = 1
            ctx.stroke()
        }
    }

    Row {
        id: _contentRow
        anchors.centerIn: parent
        spacing: 8 * Config.uiScale

        Text {
            text: picker.displayValue || picker.label
            anchors.verticalCenter: parent.verticalCenter
            font.family: Style.fontFamily
            font.pixelSize: 11 * Config.uiScale
            font.weight: Font.Bold
            font.letterSpacing: 0.5
            color: picker._popupOpen
                ? (picker.colors ? picker.colors.primary : Style.fallbackAccent)
                : (picker.colors ? picker.colors.tertiary : "#8bceff")
        }

        Text {
            text: picker._popupOpen ? "▲" : "▼"
            anchors.verticalCenter: parent.verticalCenter
            font.pixelSize: 8 * Config.uiScale
            color: picker._popupOpen
                ? (picker.colors ? picker.colors.primary : Style.fallbackAccent)
                : (picker.colors ? Qt.rgba(picker.colors.tertiary.r, picker.colors.tertiary.g, picker.colors.tertiary.b, 0.8) : "#8bceff")
        }
    }

    MouseArea {
        id: _mouse
        anchors.fill: parent
        hoverEnabled: true
        enabled: picker.enabled
        cursorShape: picker.enabled ? Qt.PointingHandCursor : Qt.ArrowCursor
        onClicked: { if (picker.enabled) _popup.open() }
    }

    Popup {
        id: _popup
        x: 0
        y: picker.height + 4
        padding: 6
        closePolicy: Popup.CloseOnEscape | Popup.CloseOnPressOutside
        onOpenedChanged: picker._popupOpen = opened

        background: Rectangle {
            radius: 4
            color: picker.colors ? Qt.rgba(picker.colors.surface.r, picker.colors.surface.g, picker.colors.surface.b, 0.96) : Qt.rgba(0.1, 0.12, 0.18, 0.96)
            border.width: 1
            border.color: picker.colors ? Qt.rgba(picker.colors.primary.r, picker.colors.primary.g, picker.colors.primary.b, 0.22) : Qt.rgba(1, 1, 1, 0.1)
        }

        contentItem: Item {
            implicitWidth: picker.popupWidth
            implicitHeight: Math.min(_itemsCol.implicitHeight, picker.popupMaxHeight)

            Flickable {
                id: _flick
                anchors.fill: parent
                contentWidth: width
                contentHeight: _itemsCol.implicitHeight
                clip: true
                interactive: contentHeight > height
                boundsBehavior: Flickable.StopAtBounds

                ScrollBar.vertical: ScrollBar {
                    policy: _flick.interactive ? ScrollBar.AsNeeded : ScrollBar.AlwaysOff
                }

                Column {
                    id: _itemsCol
                    width: _flick.width
                    spacing: 1

                    Repeater {
                        model: picker.model

                        Item {
                            id: _itemRoot
                            width: _itemsCol.width
                            height: (_showHeader ? _headerHeight : 0) + 24 * Config.uiScale

                            property bool _itemIsActive: picker.value === modelData.key
                            property bool _itemHovered: _itemMouse.containsMouse

                            // Draw a family header on the first entry of each family, so
                            // the flat list reads as the Fade / Wipe / Warp / Break groups.
                            readonly property string _family: modelData.family || ""
                            readonly property bool _showHeader: _family !== ""
                                && (index === 0 || (picker.model[index - 1].family || "") !== _family)
                            readonly property real _headerHeight: 22 * Config.uiScale
                            readonly property string _familyLabel: picker.familyLabel(_family)

                            Text {
                                visible: _itemRoot._showHeader
                                height: _itemRoot._showHeader ? _itemRoot._headerHeight : 0
                                anchors.left: parent.left
                                anchors.right: parent.right
                                anchors.leftMargin: 10 * Config.uiScale
                                anchors.rightMargin: 10 * Config.uiScale
                                verticalAlignment: Text.AlignBottom
                                bottomPadding: 3 * Config.uiScale
                                text: _itemRoot._familyLabel
                                elide: Text.ElideRight
                                font.family: Style.fontFamily
                                font.pixelSize: 9 * Config.uiScale
                                font.weight: Font.Bold
                                font.letterSpacing: 1.5
                                font.capitalization: Font.AllUppercase
                                color: picker.colors
                                    ? Qt.rgba(picker.colors.tertiary.r, picker.colors.tertiary.g, picker.colors.tertiary.b, 0.7)
                                    : Qt.rgba(0.55, 0.8, 1, 0.7)
                            }

                            Item {
                                id: _rowBody
                                anchors.left: parent.left
                                anchors.right: parent.right
                                anchors.bottom: parent.bottom
                                height: 24 * Config.uiScale

                                Rectangle {
                                    anchors.fill: parent
                                    radius: 2
                                    color: _itemRoot._itemIsActive
                                        ? (picker.colors ? Qt.rgba(picker.colors.primary.r, picker.colors.primary.g, picker.colors.primary.b, 0.28) : Qt.rgba(1, 1, 1, 0.18))
                                        : (_itemRoot._itemHovered
                                            ? (picker.colors ? Qt.rgba(picker.colors.surfaceVariant.r, picker.colors.surfaceVariant.g, picker.colors.surfaceVariant.b, 0.45) : Qt.rgba(1, 1, 1, 0.08))
                                            : "transparent")
                                    Behavior on color { ColorAnimation { duration: Style.animVeryFast } }
                                }

                                Rectangle {
                                    visible: _itemRoot._itemIsActive
                                    width: 3
                                    height: parent.height - 8
                                    anchors.left: parent.left
                                    anchors.leftMargin: 2
                                    anchors.verticalCenter: parent.verticalCenter
                                    color: picker.colors ? picker.colors.primary : Style.fallbackAccent
                                    radius: 1
                                }

                                Text {
                                    anchors.verticalCenter: parent.verticalCenter
                                    anchors.left: parent.left
                                    anchors.leftMargin: 14 * Config.uiScale
                                    anchors.right: parent.right
                                    anchors.rightMargin: 10 * Config.uiScale
                                    text: I18n.tr(modelData.label)
                                    elide: Text.ElideRight
                                    font.family: Style.fontFamily
                                    font.pixelSize: 11 * Config.uiScale
                                    font.weight: _itemRoot._itemIsActive ? Font.Bold : Font.Medium
                                    font.letterSpacing: 0.3
                                    color: _itemRoot._itemIsActive
                                        ? (picker.colors ? picker.colors.primary : Style.fallbackAccent)
                                        : (picker.colors ? picker.colors.surfaceText : "#e0e0e0")
                                }

                                MouseArea {
                                    id: _itemMouse
                                    anchors.fill: parent
                                    hoverEnabled: true
                                    cursorShape: Qt.PointingHandCursor
                                    onClicked: {
                                        picker.selected(modelData.key)
                                        _popup.close()
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
