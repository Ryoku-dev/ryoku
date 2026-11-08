import QtQuick
import QtQuick.Effects
import Quickshell
import Quickshell.Io
import Quickshell.Wayland
import Ryoku.Ui.Singletons
import "Singletons"
import "lib/pinlayout.js" as PinLayout

PanelWindow {
    id: pin

    property var entry: ({})
    signal closed(string path)
    signal painted()

    WlrLayershell.layer: WlrLayer.Top
    WlrLayershell.namespace: "ryopin"
    WlrLayershell.keyboardFocus: WlrKeyboardFocus.OnDemand
    exclusionMode: ExclusionMode.Ignore
    color: "transparent"

    anchors { top: true; left: true }

    readonly property int screenW: screen ? screen.width : 0
    readonly property int screenH: screen ? screen.height : 0

    readonly property int natW: shot.sourceSize.width
    readonly property int natH: shot.sourceSize.height
    // Image.status reaches Ready a frame before sourceSize is populated, so the
    // dimensions themselves are the seeding trigger.
    readonly property bool measured: natW > 0 && natH > 0
    readonly property var fitted: PinLayout.fitSize(natW > 0 ? natW : 1, natH > 0 ? natH : 1, screenW, screenH)

    // fitted seeds the size; the wheel breaks these bindings to resize by hand.
    property int frameW: fitted.w
    property int frameH: fitted.h

    onMeasuredChanged: if (measured && !seeded) seedSlot()
    Component.onCompleted: if (measured && !seeded) seedSlot()

    // Transparent room around the card for the drop shadow to spill into.
    readonly property int gutter: 28

    property int slotX: 0
    property int slotY: 0
    property bool seeded: false
    property bool notified: false

    implicitWidth: frameW + gutter * 2
    implicitHeight: frameH + gutter * 2

    margins.left: slotX - gutter
    margins.top: slotY - gutter

    /**
     * Places a fresh pin in its slot. The fit is recomputed here rather than
     * read off frameW, which still carries the placeholder size on the frame
     * where the image first reports its dimensions.
     */
    function seedSlot() {
        var f = PinLayout.fitSize(natW > 0 ? natW : 1, natH > 0 ? natH : 1, screenW, screenH);
        frameW = f.w;
        frameH = f.h;
        var p = PinLayout.slotPosition(entry.index || 0, f, screenW, screenH);
        var c = pin.onScreen(p.x, p.y);
        slotX = c.x;
        slotY = c.y;
        seeded = true;
    }

    // The surface carries a transparent gutter for the shadow, so the card is
    // held one gutter inside the output or the compositor clips the far edge.
    function onScreen(x, y) {
        return PinLayout.clamp({ x: x, y: y }, { w: pin.frameW, h: pin.frameH },
            pin.screenW, pin.screenH, pin.gutter);
    }

    function reclamp() {
        var c = pin.onScreen(slotX, slotY);
        slotX = c.x;
        slotY = c.y;
    }

    function editShot() {
        Spawn.run(["env", "RYOSHOT_OPEN=" + entry.path, "ryoku-cmd-ryoshot"]);
        pin.closeSelf();
    }
    function copyShot() { Bus.run("clip-copy image/png " + entry.path); }
    function copyLink() { linkFile.setText(String(entry.path)); }

    // the daemon copies from a file so the path goes there first
    FileView {
        id: linkFile
        path: "/tmp/ryopin-path.txt"
        preload: false
        printErrors: false
        onSaved: Bus.run("clip-copy text/plain " + linkFile.path)
    }
    function closeSelf() { pin.closed(entry.path); }

    FocusScope {
        id: content
        anchors.fill: parent
        focus: true

        Keys.onEscapePressed: pin.closeSelf()

        Item {
            id: body
            x: pin.gutter
            y: pin.gutter
            width: pin.frameW
            height: pin.frameH

            HoverHandler { id: hoverH }

            MultiEffect {
                id: shadowFx
                anchors.fill: frame
                source: frame
                shadowEnabled: true
                shadowColor: Theme.shadow
                shadowBlur: 1.0
                shadowVerticalOffset: 8
                blurMax: 32
                autoPaddingEnabled: true
            }

            Rectangle {
                id: frame
                anchors.fill: parent
                radius: Theme.radius
                color: Theme.panelSolid
                border.color: Theme.hair
                border.width: 1

                Image {
                    id: shot
                    anchors.fill: parent
                    anchors.margins: 6
                    source: "file://" + pin.entry.path
                    fillMode: Image.PreserveAspectFit
                    smooth: true
                    mipmap: true
                    onStatusChanged: if (status === Image.Error) console.error("ryopin: cannot load " + pin.entry.path)
                }
            }

            MouseArea {
                id: drag
                anchors.fill: frame
                hoverEnabled: true
                acceptedButtons: Qt.LeftButton | Qt.MiddleButton
                cursorShape: dragging ? Qt.ClosedHandCursor : Qt.OpenHandCursor

                property bool dragging: false
                property real px: 0
                property real py: 0

                onPressed: (m) => {
                    pin.forceActiveFocus();
                    if (m.button === Qt.MiddleButton) { pin.closeSelf(); return; }
                    dragging = true;
                    px = m.x;
                    py = m.y;
                }
                onReleased: dragging = false
                onPositionChanged: (m) => {
                    if (!dragging) return;
                    var c = pin.onScreen(pin.slotX + (m.x - px), pin.slotY + (m.y - py));
                    pin.slotX = c.x;
                    pin.slotY = c.y;
                }
                onWheel: (w) => {
                    var f = w.angleDelta.y > 0 ? 1.1 : 0.9;
                    var s = PinLayout.fitSize(pin.frameW * f, pin.frameH * f, pin.screenW, pin.screenH);
                    pin.frameW = s.w;
                    pin.frameH = s.h;
                    pin.reclamp();
                }
            }

            Row {
                id: strip
                anchors.top: frame.top
                anchors.right: frame.right
                anchors.margins: 8
                spacing: 4
                opacity: hoverH.hovered ? 1 : 0
                visible: opacity > 0.01

                Behavior on opacity { NumberAnimation { duration: Theme.snap } }

                PinButton { glyph: "\u270E"; onClicked: pin.editShot() }
                PinButton { glyph: "\u29C9"; onClicked: pin.copyShot() }
                PinButton { glyph: "\u26D3"; onClicked: pin.copyLink() }
                PinButton { glyph: "\u2715"; onClicked: pin.closeSelf() }
            }
        }

        FrameAnimation {
            running: pin.visible && !pin.notified
            onTriggered: { pin.notified = true; pin.painted(); }
        }
    }
}
