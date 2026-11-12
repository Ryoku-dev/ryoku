import QtQuick
import QtQuick.Controls
import Ryoku.Ui.Singletons
import "FolioLayout.js" as Layout
import "FolioActions.js" as Actions

Item {
    id: page

    property var tab
    property int sectionIndex: 0
    property var state
    property var options
    property var host
    property var condCtx
    property int rev: 0
    property string jumpControlId: ""
    property var expandedGroups: ({})

    function setGroupExpanded(id, expanded) {
        var next = Object.assign({}, page.expandedGroups);
        next[id] = expanded;
        page.expandedGroups = next;
    }

    signal closeRequested()

    readonly property var section: (page.tab && page.tab.sections && page.sectionIndex >= 0
        && page.sectionIndex < page.tab.sections.length) ? page.tab.sections[page.sectionIndex] : null
    readonly property string subtitle: page.section
        ? (page.section.subtitle && page.section.subtitle.length > 0 ? page.section.subtitle
           : (page.tab ? page.tab.note : "")) : ""
    readonly property var items: page.section ? Layout.buildRows(page.section) : []

    readonly property var _kbControls: {
        var out = [];
        if (!page.section) return out;
        var cs = page.section.controls || [];
        for (var i = 0; i < cs.length; i++)
            if (cs[i].kind === "keybind") out.push(cs[i]);
        return out;
    }
    readonly property var _kbKeys: {
        var out = [];
        for (var i = 0; i < page._kbControls.length; i++)
            if (page._kbControls[i].key) out.push(page._kbControls[i].key);
        return out;
    }
    function _conflicts() {
        page.rev;
        var seen = ({});
        var res = [];
        for (var i = 0; i < page._kbControls.length; i++) {
            var c = page._kbControls[i];
            var b = String(Settings.value(c.key) || "");
            if (b.length === 0) continue;
            if (seen[b] !== undefined)
                res.push({ first: seen[b], second: c.label, binding: b });
            else
                seen[b] = c.label;
        }
        return res;
    }

    property real _reveal: 1
    // a new page opens at its top; a deep link scrolls on from there
    onSectionChanged: { page._reveal = 0; revealAnim.restart(); flick.contentY = 0; }
    onTabChanged: { page._reveal = 0; revealAnim.restart(); flick.contentY = 0; }
    NumberAnimation { id: revealAnim; target: page; property: "_reveal"; from: 0; to: 1
        duration: Theme.standard; easing.type: Theme.revealEasing }

    function scrollToControl(id) {
        for (var row = 0; row < page.items.length; row++) {
            var entry = page.items[row];
            if (entry.control && entry.control.id === id && entry.group && !page.expandedGroups[entry.group]) {
                page.setGroupExpanded(entry.group, true);
                Qt.callLater(function () { page.scrollToControl(id); });
                return;
            }
        }
        if (page._placeRow(id)) {
            page.flashId = id;
            flashTimer.restart();
        }
    }
    function _placeRow(id) {
        for (var i = 0; i < bodyRepeater.count; i++) {
            var d = bodyRepeater.itemAt(i);
            if (d && d.cid === id) {
                flick.contentY = Math.max(0, Math.min(d.y - 12 * Theme.scale,
                    Math.max(0, flick.contentHeight - flick.height)));
                return true;
            }
        }
        return false;
    }
    // a deep link or search result marks where it landed for a moment
    property string flashId: ""
    Timer { id: flashTimer; interval: 2400; onTriggered: page.flashId = "" }
    // rows that fill in after the jump (the app list, display cards) must not push the landed row away
    Connections {
        target: body
        function onHeightChanged() { if (flashTimer.running) page._placeRow(page.flashId) }
    }
    onJumpControlIdChanged: if (page.jumpControlId.length > 0) Qt.callLater(function () { page.scrollToControl(page.jumpControlId); })

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
            spacing: 12 * Theme.scale

            Item {
                width: parent.width
                height: Math.max(titleText.implicitHeight, closeBtn.height)

                Text {
                    id: titleText
                    anchors.left: parent.left
                    anchors.right: closeBtn.left
                    anchors.rightMargin: 16 * Theme.scale
                    anchors.top: parent.top
                    text: page.section ? page.section.title : ""
                    font.family: Theme.display
                    font.pixelSize: Theme.fontSection
                    lineHeight: 1.0
                    color: Theme.withAlpha(Theme.surfaceText, page._reveal)
                    wrapMode: Text.WordWrap
                    renderType: Text.NativeRendering
                }
                FolioAction {
                    id: closeBtn
                    anchors.right: parent.right
                    anchors.top: parent.top
                    label: "\u00d7"
                    minWidth: 34
                    onTriggered: page.closeRequested()
                }
            }
            Text {
                width: parent.width
                visible: page.subtitle.length > 0
                text: page.subtitle
                font.family: Theme.sans
                font.weight: Font.Medium
                font.pixelSize: Theme.fontBase
                color: Theme.withAlpha(Theme.surfaceText, 0.56 * page._reveal)
                lineHeight: 1.4
                wrapMode: Text.WordWrap
                renderType: Text.NativeRendering
            }
            FolioRule { width: parent.width; alpha: 0.58; reveal: page._reveal }
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
            spacing: 28 * Theme.scale
            opacity: page._reveal
            y: (1 - page._reveal) * 10 * Theme.scale

            Repeater {
                id: bodyRepeater
                model: page.items

                delegate: Item {
                    id: dele
                    required property var modelData
                    width: body.width
                    implicitHeight: Math.max(groupLoader.height, dispLoader.height, listLoader.height, ctlLoader.height)
                    height: implicitHeight
                    visible: !dele.modelData.group || !!page.expandedGroups[dele.modelData.group]

                    Rectangle {
                        anchors.fill: parent
                        anchors.margins: -10 * Theme.scale
                        radius: Theme.radius
                        color: Theme.withAlpha(Theme.primary, 0.14)
                        border.width: 1
                        border.color: Theme.withAlpha(Theme.primary, 0.5)
                        opacity: page.flashId.length > 0 && page.flashId === dele.cid ? 1 : 0
                        visible: opacity > 0
                        Behavior on opacity { NumberAnimation { duration: Theme.standard } }
                    }

                    Loader {
                        id: groupLoader
                        width: parent.width
                        active: dele.modelData.type === "group"
                        height: active && item ? item.implicitHeight : 0
                        sourceComponent: Component {
                            FolioAction {
                                width: groupLoader.width
                                label: (page.expandedGroups[dele.modelData.id] ? "\u25be  " : "\u25b8  ") + dele.modelData.title
                                onTriggered: page.setGroupExpanded(dele.modelData.id, !page.expandedGroups[dele.modelData.id])
                            }
                        }
                    }

                    readonly property string cid: dele.modelData.type === "control"
                        ? (dele.modelData.control ? dele.modelData.control.id : "")
                        : (dele.modelData.type === "list" ? dele.modelData.base : "")

                    Loader {
                        id: dispLoader
                        width: parent.width
                        active: dele.modelData.type === "display"
                        height: active && item ? item.implicitHeight : 0
                        sourceComponent: Component {
                            DisplayRow {
                                width: dispLoader.width
                                state: page.state; options: page.options
                                reveal: page._reveal; controls: dele.modelData.controls
                            }
                        }
                    }
                    Loader {
                        id: listLoader
                        width: parent.width
                        active: dele.modelData.type === "list"
                        height: active && item ? item.implicitHeight : 0
                        sourceComponent: Component {
                            ListRow {
                                width: listLoader.width
                                state: page.state; options: page.options; host: page.host
                                reveal: page._reveal; group: dele.modelData
                            }
                        }
                    }
                    Loader {
                        id: ctlLoader
                        width: parent.width
                        active: dele.modelData.type === "control"
                        height: active && item ? item.implicitHeight : 0
                        sourceComponent: Component {
                            Item {
                                id: ctlArea
                                width: ctlLoader.width
                                implicitHeight: inner.implicitHeight
                                readonly property var c: dele.modelData.control
                                readonly property bool _shader: typeof ctlArea.c.id === "string" && ctlArea.c.id.indexOf("{shader}") >= 0
                                readonly property bool _conflict: typeof ctlArea.c.id === "string" && ctlArea.c.id.indexOf("keybinds.conflict") === 0
                                readonly property bool _reset: Actions.idOf(ctlArea.c.action) === "ResetKeybinds"

                                Column {
                                    id: inner
                                    width: parent.width
                                    spacing: 14 * Theme.scale

                                    Repeater {
                                        model: ctlArea._shader && page.options
                                            ? (page.options.revision >= 0 ? page.options.transitionOptions() : [])
                                            : []
                                        delegate: FolioControl {
                                            required property var modelData
                                            width: inner.width
                                            state: page.state; options: page.options; host: page.host; reveal: page._reveal
                                            control: ({ id: "transition.shaderScopes." + modelData.value,
                                                        kind: "dropdown", label: modelData.label,
                                                        help: ctlArea.c.help, options: ctlArea.c.options })
                                            bound: false
                                            boundValue: {
                                                var m = Settings.value("transition.shaderScopes");
                                                return (m && m[modelData.value] !== undefined) ? m[modelData.value] : "all";
                                            }
                                            onEdited: (v) => Actions.setMapField(Settings, "transition.shaderScopes", modelData.value, v)
                                        }
                                    }

                                    Repeater {
                                        model: ctlArea._conflict ? page._conflicts() : []
                                        delegate: FolioControl {
                                            required property var modelData
                                            width: inner.width
                                            state: page.state; reveal: page._reveal
                                            control: ctlArea.c
                                            staticValues: modelData
                                        }
                                    }

                                    FolioControl {
                                        width: inner.width
                                        visible: !ctlArea._shader && !ctlArea._conflict
                                        state: page.state; options: page.options; host: page.host
                                        condCtx: page.condCtx; rev: page.rev; reveal: page._reveal
                                        control: ctlArea.c
                                        actionArgs: ctlArea._reset ? ({ keys: page._kbKeys }) : ({})
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
