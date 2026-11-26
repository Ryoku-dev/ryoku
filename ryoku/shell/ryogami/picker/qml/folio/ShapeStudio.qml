import QtQuick
import QtQuick.Controls
import Ryoku.Ui.Singletons
import "FolioConditions.js" as Cond

// Keeps the picker scene visible behind it so shape edits preview live.
Item {
    id: studio

    property var tab
    property int sectionIndex: 0
    property var state
    property var options
    property var host
    property var condCtx
    property int rev: 0
    property string jumpControlId: ""

    function scrollToControl(id) {
        for (var i = 0; i < bodyRepeater.count; i++) {
            var d = bodyRepeater.itemAt(i);
            if (d && d.cid === id) {
                flick.contentY = Math.max(0, Math.min(d.y - 12 * Theme.scale,
                    Math.max(0, flick.contentHeight - flick.height)));
                return;
            }
        }
    }
    onJumpControlIdChanged: if (studio.jumpControlId.length > 0) Qt.callLater(function () { studio.scrollToControl(studio.jumpControlId); })

    signal closeRequested()
    property bool showClose: true

    readonly property var section: (studio.tab && studio.tab.sections && studio.sectionIndex >= 0
        && studio.sectionIndex < studio.tab.sections.length) ? studio.tab.sections[studio.sectionIndex] : null
    readonly property var controls: studio.section ? (studio.section.controls || []) : []

    readonly property string _mode: String(Settings.value("components.wallpaperSelector.displayMode") || "slices")
    readonly property var _modeLabels: ({
        slices: I18n.tr("Slices"), depth: I18n.tr("Depth"), hex: I18n.tr("Geometric"),
        wall: I18n.tr("Wall"), sandy: I18n.tr("Sandy"), hand: I18n.tr("Card hand"),
        collection: I18n.tr("Collection"), duel: I18n.tr("Duel")
    })
    readonly property string _modeLabel: studio._modeLabels[studio._mode] ? studio._modeLabels[studio._mode] : studio._mode

    // The visible perMode keys, so a preset snapshots exactly what is on screen.
    readonly property var _paramKeys: {
        studio.rev;
        var out = [];
        for (var i = 0; i < studio.controls.length; i++) {
            var c = studio.controls[i];
            if (c.perMode === true && c.key && Cond.evaluate(c.visibleWhen, studio.condCtx))
                out.push(c.key);
        }
        return out;
    }

    property real _reveal: 1
    onSectionChanged: { studio._reveal = 0; revealAnim.restart(); }
    NumberAnimation { id: revealAnim; target: studio; property: "_reveal"; from: 0; to: 1
        duration: Theme.standard; easing.type: Theme.revealEasing }

    Item {
        id: head
        anchors.top: parent.top
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.leftMargin: 38 * Theme.scale
        anchors.rightMargin: 38 * Theme.scale
        anchors.topMargin: 34 * Theme.scale
        height: headCol.implicitHeight

        Column {
            id: headCol
            width: parent.width
            spacing: 10 * Theme.scale

            Item {
                width: parent.width
                height: Math.max(titleText.implicitHeight, closeBtn.height)
                Text {
                    id: titleText
                    anchors.left: parent.left
                    anchors.right: closeBtn.left
                    anchors.rightMargin: 16 * Theme.scale
                    anchors.verticalCenter: parent.verticalCenter
                    text: studio.section ? studio.section.title : ""
                    font.family: Theme.sans
                    font.weight: Font.Bold
                    font.pixelSize: Theme.fs(28)
                    color: Theme.withAlpha(Theme.surfaceText, studio._reveal)
                    elide: Text.ElideRight
                    renderType: Text.NativeRendering
                }
                FolioAction {
                    id: closeBtn
                    visible: studio.showClose
                    width: studio.showClose ? implicitWidth : 0
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    label: "\u00d7"
                    minWidth: 34
                    onTriggered: studio.closeRequested()
                }
            }
            Text {
                width: parent.width
                text: I18n.tr("Shape %1").replace("%1", studio._modeLabel) + "  \u00b7  "
                    + I18n.tr("Adjust the whole layout or individual cards; the scene previews live.")
                font.family: Theme.sans
                font.weight: Font.Medium
                font.pixelSize: Theme.fontBase
                color: Theme.withAlpha(Theme.surfaceText, 0.56 * studio._reveal)
                lineHeight: 1.4
                wrapMode: Text.WordWrap
                renderType: Text.NativeRendering
            }
            FolioRule { width: parent.width; alpha: 0.58; reveal: studio._reveal }
        }
    }

    Flickable {
        id: flick
        anchors.top: head.bottom
        anchors.topMargin: 18 * Theme.scale
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        anchors.leftMargin: 38 * Theme.scale
        anchors.rightMargin: 38 * Theme.scale
        anchors.bottomMargin: 48 * Theme.scale
        contentWidth: width
        contentHeight: body.height
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollBar { policy: ScrollBar.AsNeeded; visible: false }

        Column {
            id: body
            width: flick.width
            spacing: 26 * Theme.scale
            opacity: studio._reveal
            y: (1 - studio._reveal) * 10 * Theme.scale

            Repeater {
                id: bodyRepeater
                model: studio.controls

                delegate: FolioControl {
                    required property var modelData
                    readonly property string cid: modelData && modelData.id ? String(modelData.id) : ""
                    width: body.width
                    state: studio.state; options: studio.options; host: studio.host
                    condCtx: studio.condCtx; rev: studio.rev; reveal: studio._reveal
                    control: modelData
                    paramKeys: studio._paramKeys
                }
            }
        }
    }
}
