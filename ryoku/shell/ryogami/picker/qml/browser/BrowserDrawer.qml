import QtQuick
import Ryoku.Ui.Singletons

// Owns no state; every edit is reported to the panel.
Rectangle {
    id: drawer

    property string provider: ""
    property var sources: null
    property var state: ({})
    property var collections: []
    property bool manual: false
    property real reveal: 1
    readonly property bool hasFilters: chips.model.length > 0

    signal filtersChanged(var next)
    signal applyPressed()

    color: Theme.withAlpha(Theme.surfaceVariant, 0.3)
    border.width: 1
    border.color: Theme.withAlpha(Theme.outline, 0.34)

    readonly property real _innerW: width - 28 * Theme.scale

    Flickable {
        id: flick
        anchors.top: parent.top
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: applyBtn.visible ? applyBtn.top : parent.bottom
        anchors.topMargin: 15 * Theme.scale
        anchors.leftMargin: 14 * Theme.scale
        anchors.rightMargin: 4 * Theme.scale
        anchors.bottomMargin: applyBtn.visible ? 8 * Theme.scale : 15 * Theme.scale
        clip: true
        contentWidth: width
        contentHeight: col.implicitHeight
        boundsBehavior: Flickable.StopAtBounds

        Column {
            id: col
            width: flick.width - 10 * Theme.scale
            spacing: 10 * Theme.scale

            SectionLabel {
                visible: drawer.hasFilters
                width: parent.width
                text: I18n.tr("Filters")
            }

            BrowserChips {
                id: chips
                width: drawer._innerW
                provider: drawer.provider
                sources: drawer.sources
                state: drawer.state
                collections: drawer.collections
                onChanged: function(next) { drawer.filtersChanged(next) }
            }

            Text {
                visible: !drawer.hasFilters
                width: parent.width
                text: I18n.tr("No filters for this source")
                color: Theme.withAlpha(Theme.surfaceText, 0.44)
                font.family: Theme.sans
                font.pixelSize: Theme.fontBase
                renderType: Text.NativeRendering
            }
        }
    }

    FolioAction {
        id: applyBtn
        visible: drawer.manual && drawer.hasFilters
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        anchors.leftMargin: 14 * Theme.scale
        anchors.rightMargin: 14 * Theme.scale
        anchors.bottomMargin: 15 * Theme.scale
        label: I18n.tr("Apply")
        onTriggered: drawer.applyPressed()
    }
}
