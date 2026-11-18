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
    readonly property var labels: [I18n.tr("Explain"), I18n.tr("Text"), I18n.tr("Translate"), I18n.tr("Ask")]
    readonly property var modes: ["explain", "text", "translate", "ask"]
    readonly property real segmentPadding: 9 * s
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

    Segmented {
        id: segments
        height: 32 * root.s
        implicitWidth: segmentRow.implicitWidth + 6 * root.s
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
            // itemAt is not a binding source; reading count re-evaluates once the delegates exist.
            readonly property Item current: { segmentRepeater.count; return segmentRepeater.itemAt(Math.max(0, segments.currentIndex)); }
            CutRect {
                x: 3 * root.s + (track.current ? track.current.x : 0)
                y: 3 * root.s
                width: track.current ? track.current.width : 0
                height: 26 * root.s
                cut: 5; fill: Lithic.sel
                Behavior on x { NumberAnimation { duration: Lithic.duration(240); easing.type: Easing.BezierSpline; easing.bezierCurve: [0.2, 0, 0, 1, 1, 1] } }
                Behavior on width { NumberAnimation { duration: Lithic.duration(240); easing.type: Easing.BezierSpline; easing.bezierCurve: [0.2, 0, 0, 1, 1, 1] } }
            }
            Row {
                id: segmentRow
                x: 3 * root.s; y: 3 * root.s
                Repeater {
                    id: segmentRepeater
                    model: root.labels
                    delegate: T.AbstractButton {
                        id: segment
                        required property string modelData
                        required property int index
                        leftPadding: root.segmentPadding; rightPadding: root.segmentPadding
                        implicitWidth: contentItem.implicitWidth + leftPadding + rightPadding
                        width: implicitWidth; height: 26 * root.s
                        hoverEnabled: true; focusPolicy: Qt.NoFocus
                        Accessible.role: Accessible.RadioButton
                        Accessible.name: modelData
                        Accessible.checked: index === segments.currentIndex
                        onClicked: segments.select(index)
                        scale: down ? 0.97 : 1
                        Behavior on scale { NumberAnimation { duration: Lithic.duration(90) } }
                        background: CutRect {
                            cut: 5; fill: Lithic.alpha(Lithic.fg, 0.08)
                            opacity: segment.hovered && segment.index !== segments.currentIndex ? 1 : 0
                            Behavior on opacity { NumberAnimation { duration: Lithic.duration(segment.hovered ? 160 : 100) } }
                        }
                        contentItem: Label {
                            text: segment.modelData; typeRole: "label"
                            ink: segment.index === segments.currentIndex ? Lithic.onSel : Lithic.fg2
                            horizontalAlignment: Text.AlignHCenter; verticalAlignment: Text.AlignVCenter
                        }
                    }
                }
            }
        }
    }
    Item {
        x: segments.width
        width: Math.max(8 * root.s, root.width - segments.width - controls.width)
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
