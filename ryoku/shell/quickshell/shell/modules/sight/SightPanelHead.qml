pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Templates as T
import Lithic
import Ryoku.Ui.Singletons

Item {
    id: root
    property real s: 1
    property string mode: "explain"
    onModeChanged: Qt.callLater(syncMode)
    Component.onCompleted: syncMode()
    function syncMode() { segments.currentIndex = modes.indexOf(mode); }
    property bool copied: false
    property bool imageAvailable: true
    property bool kept: false
    required property var tabs
    property string temporaryAction: ""
    property int segmentsReady: 0
    readonly property var labels: tabs.map(row => row.label)
    readonly property var modes: tabs.map(row => row.id)
    onTabsChanged: Qt.callLater(syncMode)
    readonly property real segmentPadding: LithicTokens.space.s2 * s
    readonly property real segmentInset: LithicTokens.size.segmentInset * s
    readonly property rect tabCapBounds: tabFontMetrics.tightBoundingRect("H")
    FontMetrics {
        id: tabFontMetrics
        font.family: Lithic.type.label.family
        font.pixelSize: Math.round(Lithic.type.label.pixelSize * root.s)
        font.weight: Lithic.type.label.weight
    }
    readonly property rect moreIconBounds: moreFontMetrics.tightBoundingRect("apps")
    FontMetrics {
        id: moreFontMetrics
        font.family: Lithic.font.icons
        font.pixelSize: Math.round(14 * root.s)
        font.weight: Font.Normal
    }
    readonly property real minimumWidth: segments.implicitWidth + 8 * s + controls.implicitWidth
    readonly property var moreAnchor: moreButton
    readonly property bool dragging: drag.active
    property point lastDrag: Qt.point(0, 0)
    signal modeRequested(string mode)
    signal copyRequested()
    signal pinRequested()
    signal moreRequested()
    signal closeRequested()
    signal dragMoved(real dx, real dy)
    implicitHeight: 32 * s

    Flickable {
        id: tabScroll
        width: Math.min(segments.implicitWidth, Math.max(0, root.width - controls.implicitWidth - 8 * root.s))
        height: root.height
        contentWidth: segments.width
        contentHeight: height
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        function revealCurrent() {
            const current = track.current;
            if (!current) return;
            contentX = Math.max(0, Math.min(contentWidth - width, current.x + current.width + 3 * root.s > contentX + width
                ? current.x + current.width + 3 * root.s - width : Math.min(contentX, current.x)));
        }
        Connections { target: segments; function onCurrentIndexChanged() { Qt.callLater(tabScroll.revealCurrent); } }
    Segmented {
        id: segments
        height: 32 * root.s
        padding: 0
        implicitWidth: segmentRow.implicitWidth + root.segmentInset * 2
        width: implicitWidth
        model: root.labels
        currentIndex: root.modes.indexOf(root.mode)
        accessibleName: I18n.tr("Capture actions")
        onActivated: index => root.modeRequested(root.modes[index])
        background: Item {
            CutRect { anchors.fill: parent; cut: 7; fill: Lithic.field }
            FocusRing { anchors.fill: parent; cut: 7; keyboardFocusVisible: segments.visualFocus }
        }
        contentItem: Item {
            id: track
            readonly property Item current: {
                root.segmentsReady;
                segmentRepeater.count;
                const index = segments.currentIndex;
                return segmentRepeater.itemAt(Math.max(0, index));
            }
            property bool motionReady: false
            onCurrentChanged: {
                if (!current) return;
                Qt.callLater(function() {
                    if (!track.current) return;
                    tabScroll.revealCurrent();
                    track.motionReady = true;
                });
            }
            CutRect {
                x: root.segmentInset + (track.current ? track.current.x : 0)
                anchors.verticalCenter: parent.verticalCenter
                width: track.current ? track.current.width : 0
                height: segmentRow.height
                cut: LithicTokens.cut.segment; fill: Lithic.sel
                Behavior on x { enabled: track.motionReady && track.current !== null; NumberAnimation { duration: Lithic.duration(240); easing.type: Easing.BezierSpline; easing.bezierCurve: [0.2, 0, 0, 1, 1, 1] } }
                Behavior on width { enabled: track.motionReady && track.current !== null; NumberAnimation { duration: Lithic.duration(240); easing.type: Easing.BezierSpline; easing.bezierCurve: [0.2, 0, 0, 1, 1, 1] } }
            }
            Row {
                id: segmentRow
                x: root.segmentInset
                anchors.verticalCenter: parent.verticalCenter
                height: segments.availableHeight - root.segmentInset * 2
                Repeater {
                    id: segmentRepeater
                    model: root.tabs
                    delegate: T.AbstractButton {
                        id: segment
                        required property var modelData
                        required property int index
                        leftPadding: root.segmentPadding; rightPadding: root.segmentPadding
                        implicitWidth: contentItem.implicitWidth + leftPadding + rightPadding
                        width: implicitWidth; height: segmentRow.height
                        hoverEnabled: true; focusPolicy: Qt.NoFocus
                        Accessible.role: Accessible.RadioButton
                        Accessible.name: modelData.label
                        Accessible.checked: index === segments.currentIndex
                        onClicked: segments.select(index)
                        scale: down ? 0.97 : 1
                        Behavior on scale { NumberAnimation { duration: Lithic.duration(90) } }
                        property real entered: modelData.id === root.temporaryAction ? 0 : 1
                        opacity: entered
                        transform: Translate { x: (1 - segment.entered) * 12 * root.s }
                        Behavior on entered { NumberAnimation { duration: Lithic.duration(180); easing.type: Easing.OutCubic } }
                        Component.onCompleted: { entered = 1; ++root.segmentsReady; }
                        Component.onDestruction: --root.segmentsReady
                        background: CutRect {
                            cut: 5; fill: Lithic.alpha(Lithic.fg, 0.08)
                            opacity: segment.hovered && segment.index !== segments.currentIndex ? 1 : 0
                            Behavior on opacity { NumberAnimation { duration: Lithic.duration(segment.hovered ? 160 : 100) } }
                        }
                        contentItem: Row {
                            spacing: segment.modelData.id === "more" ? 4 * root.s : 0
                            Glyph {
                                visible: segment.modelData.id === "more"
                                name: "apps"; size: 14
                                ink: segment.index === segments.currentIndex ? Lithic.onSel : Lithic.fg2
                                y: (parent.height - root.moreIconBounds.height) / 2 - baselineOffset - root.moreIconBounds.y
                            }
                            Label {
                                id: tabLabel
                                // One cap-height reference keeps every tab on the same baseline.
                                y: (parent.height - root.tabCapBounds.height) / 2 - baselineOffset - root.tabCapBounds.y
                                text: segment.modelData.label; typeRole: "label"
                                ink: segment.index === segments.currentIndex ? Lithic.onSel : Lithic.fg2
                                horizontalAlignment: Text.AlignHCenter; verticalAlignment: Text.AlignVCenter
                            }
                        }
                    }
                }
            }
        }
    }
    }
    Item {
        x: tabScroll.width
        width: Math.max(8 * root.s, root.width - tabScroll.width - controls.width)
        height: root.height
        HoverHandler { enabled: root.kept; cursorShape: drag.active ? Qt.ClosedHandCursor : Qt.OpenHandCursor }
        DragHandler {
            id: drag
            target: null
            enabled: root.kept
            onActiveChanged: if (active) root.lastDrag = Qt.point(0, 0)
            onActiveTranslationChanged: {
                if (!active) return;
                root.dragMoved(activeTranslation.x - root.lastDrag.x, activeTranslation.y - root.lastDrag.y);
                root.lastDrag = activeTranslation;
            }
        }
    }
    Row {
        id: controls
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        spacing: 2 * root.s
        SightIconButton { s: root.s; glyph: "content_copy"; copied: root.copied; accessibleName: root.copied ? I18n.tr("Copied") : I18n.tr("Copy"); onClicked: root.copyRequested() }
        SightIconButton { s: root.s; glyph: "keep"; accessibleName: I18n.tr("Pin"); enabled: root.imageAvailable; onClicked: root.pinRequested() }
        SightIconButton { id: moreButton; s: root.s; glyph: "more_horiz"; accessibleName: I18n.tr("More"); onClicked: root.moreRequested() }
        Item { width: 9 * root.s; height: 30 * root.s; CutRect { anchors.centerIn: parent; width: 1; height: 18 * root.s; fill: Lithic.vein } }
        SightIconButton { s: root.s; glyph: "close"; accessibleName: I18n.tr("Close"); turnOnHover: true; onClicked: root.closeRequested() }
    }
}
