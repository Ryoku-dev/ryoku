pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons

// A live-preview card for a desktop widget installed from RyoStore. It mounts
// the plugin's own view under a faked host contract -- the real one is supplied
// by the shell's Desktop layer -- so its face reads exactly as it will on the
// wallpaper, scaled to fit and never blown up past life size. A plugin that
// fails to load or reports no size shows its manifest glyph instead of taking
// the page down. The toggle applies at once through ryoku-plugins-place; it
// never joins this page's draft/Save flow.
Rectangle {
    id: card

    property string title: ""
    property bool on: false
    property string icon: ""
    property string dir: ""
    property var settings: ({})
    signal toggled(bool value)

    height: 236
    radius: Tokens.radius
    color: Tokens.paperLift
    border.width: Tokens.border
    border.color: hov.hovered ? Tokens.sun : (card.on ? Tokens.line : Tokens.lineSoft)
    Behavior on border.color { ColorAnimation { duration: Tokens.snap } }

    HoverHandler { id: hov }

    // the host object the plugin reads: only the fields safe to fake in a
    // preview. mainInstance is the plugin's service singleton on the real
    // desktop; here there is none, and saveSettings must be inert.
    QtObject {
        id: apiStub
        property var mainInstance: null
        property var pluginSettings: card.settings
        property string pluginDir: card.dir
        function saveSettings() {}
    }

    readonly property bool previewOk: view.status === Loader.Ready && view.item
        && (view.item.implicitWidth > 0 || view.item.implicitHeight > 0)

    function applyHost() {
        var it = view.item;
        if (view.status !== Loader.Ready || !it)
            return;
        // a plugin that never declared a host property would throw on assign; a
        // preview must swallow that rather than let it reach the page.
        try {
            it.pluginApi = apiStub;
            it.density = "compact";
            it.s = 1;
            it.widthBudget = 360;
            it.active = true;
        } catch (e) {}
    }

    Item {
        id: pbody
        anchors { left: parent.left; right: parent.right; top: parent.top }
        anchors.margins: Tokens.s3
        height: card.height - footer.height - Tokens.s3 * 3
        clip: true
        opacity: card.on ? 1 : 0.45
        Behavior on opacity { NumberAnimation { duration: Tokens.snap } }

        Item {
            id: pwrap
            readonly property real natW: (view.item && view.item.implicitWidth > 0) ? view.item.implicitWidth : 360
            readonly property real natH: (view.item && view.item.implicitHeight > 0) ? view.item.implicitHeight : 200
            width: pwrap.natW
            height: pwrap.natH
            anchors.centerIn: parent
            visible: card.previewOk
            // never upscale past life size: a preview blown up to fill clips
            // flush against the footer and reads as overlapping the label row.
            scale: Math.min(pbody.width / pwrap.width, pbody.height / pwrap.natH, 1.0)
            Loader {
                id: view
                anchors.fill: parent
                asynchronous: true
                source: card.dir.length > 0 ? "file://" + card.dir + "/content/Widget.qml" : ""
                onLoaded: card.applyHost()
                onStatusChanged: card.applyHost()
            }
        }

        // broken, empty, or size-less plugin: its manifest glyph, never a blank
        // card and never a crash. Held back while the async view is still
        // loading so the face fills in cleanly instead of flashing the glyph.
        Text {
            anchors.centerIn: parent
            visible: !card.previewOk && view.status !== Loader.Loading
            text: card.icon.length > 0 ? card.icon : "widgets"
            color: Tokens.inkFaint
            font.family: "Material Symbols Rounded"
            font.pixelSize: 40
        }
    }

    Item {
        id: footer
        anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
        anchors.leftMargin: Tokens.s4; anchors.rightMargin: Tokens.s3; anchors.bottomMargin: Tokens.s3
        height: 34
        Column {
            anchors { left: parent.left; verticalCenter: parent.verticalCenter }
            spacing: 2
            Text {
                text: card.title
                color: Tokens.ink; font.family: Tokens.ui
                font.pixelSize: Tokens.fRow; font.weight: Font.Medium
            }
            Text {
                text: (card.on ? I18n.tr("On") : I18n.tr("Off")).toUpperCase()
                color: card.on ? Tokens.inkMuted : Tokens.inkFaint
                font.family: Tokens.mono; font.pixelSize: Tokens.fTiny; font.letterSpacing: 0.6
            }
        }
        Sw {
            anchors { right: parent.right; verticalCenter: parent.verticalCenter }
            on: card.on
            onToggled: (v) => card.toggled(v)
        }
    }
}
