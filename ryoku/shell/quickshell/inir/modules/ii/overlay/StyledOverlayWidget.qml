pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import inir
import inir.modules.common
import inir.modules.common.widgets
import inir.modules.iris.components
import inir.modules.iris.style
import inir.services

Item {
    id: root

    required property Item contentItem
    required property var modelData
    readonly property string identifier: String(modelData?.identifier ?? "")
    readonly property string materialSymbol: String(modelData?.materialSymbol ?? "widgets")
    readonly property var persistentStateEntry: Persistent.states.overlay[identifier]
    property string title: Translation.tr(OverlayLook.titles[identifier] ?? identifier)
    property bool fancyBorders: true
    property bool showCenterButton: false
    property bool showClickabilityButton: true
    property bool resizable: true
    property string titlebarActionSymbol: ""
    property string titlebarActionTooltip: ""
    signal titlebarActionClicked()

    readonly property bool pinned: persistentStateEntry?.pinned ?? false
    readonly property bool clickthrough: persistentStateEntry?.clickthrough ?? false
    readonly property bool actuallyPinned: root.pinned && root.open
    readonly property bool actuallyClickable: root.actuallyPinned && !root.clickthrough
    readonly property bool open: Array.from(Persistent.states.overlay.open ?? []).includes(root.identifier)
    property real minimumWidth: root.contentItem?.implicitWidth ?? 220
    property real minimumHeight: root.contentItem?.implicitHeight ?? 120
    property real panelOpacityOverride: -1
    readonly property real contentRadius: Math.max(0, IrisStyle.radiusSheet - Math.round(7 * IrisStyle.density))

    x: Math.max(0, Math.min((parent?.width ?? 1920) - width, Math.round(Number(root.persistentStateEntry?.x ?? 80))))
    y: Math.max(0, Math.min((parent?.height ?? 1080) - height, Math.round(Number(root.persistentStateEntry?.y ?? 80))))
    width: Math.max(root.minimumWidth + Math.round(14 * IrisStyle.density), Number(root.persistentStateEntry?.width ?? root.minimumWidth))
    height: Math.max(root.minimumHeight + Math.round(48 * IrisStyle.density), Number(root.persistentStateEntry?.height ?? root.minimumHeight))
    visible: GlobalStates.overlayOpen || root.actuallyPinned
    z: drag.active ? 10 : 1

    function reportState(): void {
        OverlayContext.pin(root.identifier, root.actuallyPinned)
        OverlayContext.registerClickableWidget(root, root.actuallyClickable)
    }
    function saveGeometry(): void {
        if (!root.persistentStateEntry) return
        root.persistentStateEntry.x = Math.round(root.x)
        root.persistentStateEntry.y = Math.round(root.y)
        root.persistentStateEntry.width = Math.round(root.width)
        root.persistentStateEntry.height = Math.round(root.height)
    }
    function close(): void {
        Persistent.states.overlay.open = Array.from(Persistent.states.overlay.open ?? []).filter(value => value !== root.identifier)
    }
    function togglePinned(): void { root.persistentStateEntry.pinned = !root.pinned }
    function toggleClickthrough(): void { root.persistentStateEntry.clickthrough = !root.clickthrough }
    function center(): void {
        root.x = Math.round((root.parent.width - root.width) / 2)
        root.y = Math.round((root.parent.height - root.height) / 2)
        root.saveGeometry()
    }

    onActuallyPinnedChanged: root.reportState()
    onActuallyClickableChanged: root.reportState()
    Component.onCompleted: root.reportState()
    Component.onDestruction: {
        OverlayContext.pin(root.identifier, false)
        OverlayContext.registerClickableWidget(root, false)
    }

    Rectangle {
        anchors.fill: parent
        radius: IrisStyle.radiusSheet
        color: root.fancyBorders ? IrisStyle.bodySurface : "transparent"
        border.width: root.fancyBorders ? 1 : 0
        border.color: IrisStyle.border
        opacity: root.panelOpacityOverride >= 0 ? root.panelOpacityOverride : 1
    }

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: root.fancyBorders ? Math.round(6 * IrisStyle.density) : 0
        spacing: 0

        Rectangle {
            id: titlebar
            visible: GlobalStates.overlayOpen
            Layout.fillWidth: true
            implicitHeight: visible ? Math.round(36 * IrisStyle.density) : 0
            color: "transparent"
            RowLayout {
                anchors.fill: parent
                spacing: Math.round(2 * IrisStyle.density)
                MaterialSymbol {
                    Layout.leftMargin: Math.round(6 * IrisStyle.density)
                    text: root.materialSymbol
                    iconSize: Math.round(18 * IrisStyle.density)
                    color: IrisStyle.textSecondary
                }
                IrisText {
                    Layout.fillWidth: true
                    text: root.title
                    color: IrisStyle.text
                    font.pixelSize: IrisStyle.typeLabel
                    font.weight: IrisStyle.weight(Font.DemiBold)
                    elide: Text.ElideRight
                }
                ToolButton { visible: root.titlebarActionSymbol.length > 0; glyph: root.titlebarActionSymbol; tip: root.titlebarActionTooltip; onClicked: root.titlebarActionClicked() }
                ToolButton { visible: root.showCenterButton; glyph: "recenter"; tip: Translation.tr("Center"); onClicked: root.center() }
                ToolButton { visible: root.pinned && root.showClickabilityButton; glyph: "mouse"; selected: !root.clickthrough; tip: Translation.tr("Clickable when pinned"); onClicked: root.toggleClickthrough() }
                ToolButton { glyph: "keep"; selected: root.pinned; tip: Translation.tr("Pin"); onClicked: root.togglePinned() }
                ToolButton { glyph: "close"; tip: Translation.tr("Close"); onClicked: root.close() }
            }
            DragHandler {
                id: drag
                target: root
                enabled: GlobalStates.overlayOpen
                onActiveChanged: if (!active) root.saveGeometry()
            }
        }

        Item {
            id: contentHost
            Layout.fillWidth: true
            Layout.fillHeight: true
            children: [root.contentItem]
            Binding { target: root.contentItem; property: "width"; value: contentHost.width }
            Binding { target: root.contentItem; property: "height"; value: contentHost.height }
        }
    }

    MouseArea {
        id: resizeHandle
        visible: root.resizable && GlobalStates.overlayOpen
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        width: Math.round(18 * IrisStyle.density)
        height: width
        cursorShape: Qt.SizeFDiagCursor
        property real startWidth: 0
        property real startHeight: 0
        onPressed: {
            startWidth = root.width
            startHeight = root.height
        }
        onPositionChanged: event => {
            if (!pressed) return
            root.width = Math.max(root.minimumWidth, startWidth + event.x)
            root.height = Math.max(root.minimumHeight, startHeight + event.y)
        }
        onReleased: root.saveGeometry()
    }

    component ToolButton: IrisIconButton {
        id: control
        property string glyph: ""
        property string tip: ""
        materialIcon: glyph
        StyledToolTip { text: control.tip }
    }
}
