pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls as QQC
import Lithic
import Ryoku.Ui.Singletons
import shell.services
import "../ask/look"

Item {
    id: root

    property real s: 1
    property string glyph: ""
    property string label: ""
    property string detail: ""
    property string caption: root.label
    property bool on: false
    property bool available: true
    property string size: "large"
    property bool split: false
    property bool toggles: true
    property string tone: ""
    property string pageName: I18n.tr("%1 settings").arg(root.label)
    signal toggled()
    signal opened()

    readonly property bool small: root.size === "small"
    readonly property real buttonHeight: Math.round(QsLook.tileHeight(root.size) * root.s)
    readonly property real cutPx: QsLook.tileCut(root.size) * root.s
    readonly property real arrowWidth: Math.round((QsLook.fluent ? 34 : 40) * root.s)
    readonly property string switchName: root.on ? I18n.tr("Turn off %1").arg(root.label)
        : I18n.tr("Turn on %1").arg(root.label)

    implicitHeight: root.buttonHeight + Math.round(QsLook.tileLabelRow * root.s)
    opacity: root.available ? 1 : Lithic.disabledOpacity
    Behavior on opacity {
        NumberAnimation { duration: QsLook.dur(QsLook.stateMs) }
    }

    function act() {
        if (root.toggles)
            root.toggled();
        else
            root.opened();
    }

    property real fill: root.on ? 1 : 0
    Behavior on fill {
        NumberAnimation { duration: QsLook.dur(260); easing.type: Easing.OutCubic }
    }
    property real nod: 0
    onOnChanged: if (!QsLook.still && QsLook.nod !== 0) nodRun.restart()
    SequentialAnimation {
        id: nodRun
        NumberAnimation { target: root; property: "nod"; to: QsLook.nod; duration: 90; easing.type: Easing.OutQuad }
        NumberAnimation { target: root; property: "nod"; to: 0; duration: 340; easing.type: Easing.OutBack; easing.overshoot: 2.4 }
    }

    component Glyph: Text {
        property real px: 20
        property color ink: QsLook.tileGlyphInk(root.on)
        text: root.glyph
        color: ink
        font.family: Look.icons
        font.pixelSize: Math.round(px * root.s)
        font.variableAxes: ({ "FILL": Look.fillChosen ? root.fill : 0, "wght": Look.iconWeight })
        renderType: Text.NativeRendering
        horizontalAlignment: Text.AlignHCenter
        verticalAlignment: Text.AlignVCenter
        rotation: root.nod
        Accessible.ignored: true
        Behavior on color {
            ColorAnimation { duration: QsLook.dur(QsLook.stateMs) }
        }
    }
    component Name: Text {
        property real px: 13
        text: root.label
        color: QsLook.tileInk(root.on)
        font.family: QsLook.sans
        font.pixelSize: Math.round(px * root.s)
        font.weight: QsLook.adwaita ? Font.DemiBold : Font.Medium
        elide: Text.ElideRight
        renderType: Text.NativeRendering
        Behavior on color {
            ColorAnimation { duration: QsLook.dur(QsLook.stateMs) }
        }
    }
    component Detail: Text {
        text: root.detail
        visible: text.length > 0
        color: QsLook.tileDetailInk(root.on)
        font.family: QsLook.lithic ? QsLook.mono : QsLook.sans
        font.pixelSize: Math.round((QsLook.lithic ? 10 : 12) * root.s)
        elide: Text.ElideRight
        renderType: Text.NativeRendering
        Behavior on color {
            ColorAnimation { duration: QsLook.dur(QsLook.stateMs) }
        }
    }
    component Target: QQC.AbstractButton {
        property bool switching: true
        hoverEnabled: true
        focusPolicy: Qt.StrongFocus
        enabled: root.available
        background: Item {}
        contentItem: Item {}
        Accessible.role: switching && root.toggles ? Accessible.CheckBox : Accessible.Button
        Accessible.checkable: switching && root.toggles
        Accessible.checked: root.on
        Accessible.description: root.detail
        Keys.onRightPressed: event => {
            if (root.split) {
                root.opened();
                event.accepted = true;
            } else {
                event.accepted = false;
            }
        }
        HoverHandler {
            enabled: parent.enabled
            cursorShape: Qt.PointingHandCursor
        }
    }

    Loader {
        id: form
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        height: root.buttonHeight
        sourceComponent: QsLook.material ? material : QsLook.adwaita ? adwaita
            : QsLook.fluent ? fluent : lithic
    }

    Text {
        visible: QsLook.fluent
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: form.bottom
        anchors.topMargin: Math.round(6 * root.s)
        text: root.caption
        color: Lithic.fg
        font.family: QsLook.sans
        font.pixelSize: Math.round(12 * root.s)
        horizontalAlignment: Text.AlignHCenter
        wrapMode: Text.Wrap
        maximumLineCount: 2
        elide: Text.ElideRight
        renderType: Text.NativeRendering
    }

    Component {
        id: lithic

        Item {
            id: plaque

            readonly property bool pressed: body.down || toggle.down
            readonly property real cut: root.cutPx + (plaque.pressed ? Lithic.pressCutGrow * root.s : 0)

            CutRect {
                id: plane
                anchors.fill: parent
                cut: plaque.cut
                fill: QsLook.tileFill(root.on, body.hovered, body.down)
                strokeColor: QsLook.tileStroke(root.on)
                strokeWidth: Math.max(1, Math.round(root.s))
                Behavior on cut {
                    NumberAnimation { duration: QsLook.dur(LithicTokens.motion.press); easing.type: Easing.OutCubic }
                }
                Behavior on fill {
                    ColorAnimation { duration: QsLook.dur(body.hovered ? LithicTokens.motion.hoverIn : LithicTokens.motion.hoverOut) }
                }
            }
            Notch {
                size: Math.round(9 * root.s)
                fill: Lithic.sel
                corner: "topLeft"
                scale: root.on ? 1 : 0
                transformOrigin: Item.TopLeft
                Behavior on scale {
                    NumberAnimation { duration: QsLook.dur(220); easing.type: Easing.OutBack; easing.overshoot: 1.6 }
                }
            }

            Item {
                anchors.fill: parent
                anchors.topMargin: plaque.pressed ? Lithic.pressSink * root.s : 0

                Glyph {
                    id: mark
                    x: Math.round(12 * root.s)
                    anchors.verticalCenter: parent.verticalCenter
                    width: Math.round(20 * root.s)
                    px: 18
                }
                Column {
                    anchors.left: mark.right
                    anchors.leftMargin: Math.round(10 * root.s)
                    anchors.right: tail.left
                    anchors.rightMargin: Math.round(8 * root.s)
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Math.round(3 * root.s)

                    Name {
                        width: parent.width
                    }
                    Row {
                        width: parent.width
                        visible: !root.small && root.detail.length > 0
                        spacing: Math.round(5 * root.s)

                        Flake {
                            visible: root.tone.length > 0
                            anchors.verticalCenter: parent.verticalCenter
                            tone: root.tone
                            size: Math.round(7 * root.s)
                        }
                        Detail {
                            width: parent.width - (root.tone.length > 0 ? Math.round(15 * root.s) : 0)
                        }
                    }
                }
                Item {
                    id: tail
                    anchors.right: parent.right
                    anchors.rightMargin: Math.round(10 * root.s)
                    anchors.verticalCenter: parent.verticalCenter
                    width: root.split && root.toggles ? toggle.width : stateWord.implicitWidth
                    height: parent.height

                    Text {
                        id: stateWord
                        visible: !root.split
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        text: !root.toggles ? "" : root.on ? I18n.tr("On") : I18n.tr("Off")
                        color: root.on ? Lithic.sel : Lithic.fg3
                        font.family: QsLook.mono
                        font.pixelSize: Math.round(9 * root.s)
                        font.capitalization: Font.AllUppercase
                        font.letterSpacing: Look.tracking * root.s
                        renderType: Text.NativeRendering
                    }
                    Symbol {
                        visible: root.split && !root.toggles
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        s: root.s
                        name: "chevron_right"
                        size: 16
                        ink: Lithic.fg3
                    }
                }
            }

            Target {
                id: body
                anchors.fill: parent
                switching: !root.split
                Accessible.name: root.split ? root.pageName : root.label
                onClicked: root.split ? root.opened() : root.act()
            }
            SidebarToggle {
                id: toggle
                visible: root.split && root.toggles
                anchors.right: parent.right
                anchors.rightMargin: Math.round(10 * root.s)
                anchors.verticalCenter: parent.verticalCenter
                s: root.s
                small: true
                on: root.on
                enabled: root.available
                Accessible.name: root.switchName
                onToggleRequested: root.toggled()
            }
            FocusFrame {
                s: root.s
                shown: body.visualFocus
                cut: plaque.cut
            }
        }
    }

    Component {
        id: material

        Item {
            id: tonal

            readonly property bool pressed: body.down || icon.down
            readonly property real cut: tonal.pressed ? QsLook.tilePressedCut * root.s : root.cutPx

            scale: tonal.pressed ? 0.96 : 1
            Behavior on scale {
                NumberAnimation { duration: QsLook.dur(tonal.pressed ? 90 : 360); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.springSoft }
            }

            CutRect {
                anchors.fill: parent
                cut: tonal.cut
                fill: QsLook.tileFill(root.on, body.hovered, body.down)
                Behavior on cut {
                    NumberAnimation { duration: QsLook.dur(tonal.pressed ? 90 : 360); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.springSoft }
                }
                Behavior on fill {
                    ColorAnimation { duration: QsLook.dur(QsLook.stateMs * 2) }
                }
            }

            Target {
                id: body
                anchors.fill: parent
                switching: !root.split
                Accessible.name: root.split ? root.pageName : root.label
                onClicked: root.split ? root.opened() : root.act()
            }

            CutRect {
                id: square
                visible: !root.small
                x: Math.round(12 * root.s)
                anchors.verticalCenter: parent.verticalCenter
                width: Math.round(48 * root.s)
                height: width
                cut: (icon.down ? 4 : 6) * root.s
                fill: root.split ? Lithic.mix(QsLook.tileRest(root.on), QsLook.tileInk(root.on),
                    icon.down ? 0.2 : icon.hovered ? 0.16 : 0.1) : "transparent"
                Behavior on cut {
                    NumberAnimation { duration: QsLook.dur(200); easing.type: Easing.OutCubic }
                }
                Behavior on fill {
                    ColorAnimation { duration: QsLook.dur(QsLook.stateMs) }
                }
            }
            Glyph {
                anchors.centerIn: root.small ? parent : square
                px: 24
            }
            Column {
                visible: !root.small
                anchors.left: square.right
                anchors.leftMargin: Math.round(12 * root.s)
                anchors.right: chevron.visible ? chevron.left : parent.right
                anchors.rightMargin: Math.round(12 * root.s)
                anchors.verticalCenter: parent.verticalCenter
                spacing: Math.round(2 * root.s)

                Name {
                    width: parent.width
                    px: 14
                }
                Detail {
                    width: parent.width
                }
            }
            Symbol {
                id: chevron
                // dual-target tiles open from the label like Android's, so only open-only tiles keep the chevron
                visible: !root.small && root.split && !root.toggles
                anchors.right: parent.right
                anchors.rightMargin: Math.round(12 * root.s)
                anchors.verticalCenter: parent.verticalCenter
                s: root.s
                name: "chevron_right"
                size: 18
                ink: QsLook.tileDetailInk(root.on)
            }
            Target {
                id: icon
                visible: root.split && !root.small && root.toggles
                anchors.fill: square
                Accessible.name: root.switchName
                onClicked: root.toggled()
            }
            FocusFrame {
                s: root.s
                shown: body.visualFocus
                cut: tonal.cut
            }
            FocusFrame {
                s: root.s
                anchors.fill: square
                anchors.margins: -outset
                shown: icon.visualFocus
                cut: square.cut
            }
            CornerTip {
                s: root.s
                visible: root.small && body.hovered && !body.down
                text: root.on ? I18n.tr("%1, on").arg(root.label) : I18n.tr("%1, off").arg(root.label)
            }
        }
    }

    Component {
        id: adwaita

        Item {
            id: pill

            readonly property real radius: height / 2
            readonly property bool divided: root.split && root.toggles
            readonly property real arrow: pill.divided ? root.arrowWidth : 0
            readonly property real seam: Math.max(1, Math.round(2 * root.s))

            Rectangle {
                id: main
                anchors.left: parent.left
                anchors.top: parent.top
                anchors.bottom: parent.bottom
                width: parent.width - (pill.divided ? pill.arrow + pill.seam : 0)
                topLeftRadius: pill.radius
                bottomLeftRadius: pill.radius
                topRightRadius: pill.divided ? 0 : pill.radius
                bottomRightRadius: pill.divided ? 0 : pill.radius
                color: QsLook.tileFill(root.on, body.hovered, body.down)
                Behavior on color {
                    ColorAnimation { duration: QsLook.dur(QsLook.stateMs) }
                }
            }
            Rectangle {
                id: side
                visible: pill.divided
                anchors.right: parent.right
                anchors.top: parent.top
                anchors.bottom: parent.bottom
                width: pill.arrow
                topRightRadius: pill.radius
                bottomRightRadius: pill.radius
                color: QsLook.tileFill(root.on, more.hovered, more.down)
                Behavior on color {
                    ColorAnimation { duration: QsLook.dur(QsLook.stateMs) }
                }
            }

            Glyph {
                id: mark
                x: Math.round(16 * root.s)
                anchors.verticalCenter: parent.verticalCenter
                width: Math.round(18 * root.s)
                px: 17
            }
            Column {
                anchors.left: mark.right
                anchors.leftMargin: Math.round(10 * root.s)
                anchors.right: main.right
                anchors.rightMargin: Math.round((pill.divided ? 8 : 14) * root.s)
                anchors.verticalCenter: parent.verticalCenter
                spacing: Math.round(1 * root.s)

                Name {
                    width: parent.width
                }
                Detail {
                    width: parent.width
                    visible: !root.small && text.length > 0
                }
            }
            Symbol {
                visible: pill.divided || (root.split && !root.toggles)
                anchors.horizontalCenter: pill.divided ? side.horizontalCenter : undefined
                anchors.right: pill.divided ? undefined : parent.right
                anchors.rightMargin: Math.round(14 * root.s)
                anchors.verticalCenter: parent.verticalCenter
                s: root.s
                name: "chevron_right"
                size: 18
                ink: QsLook.tileInk(root.on)
            }

            Target {
                id: body
                anchors.fill: main
                switching: true
                Accessible.name: root.toggles ? root.label : root.pageName
                onClicked: root.act()
            }
            Target {
                id: more
                visible: pill.divided
                anchors.fill: side
                switching: false
                Accessible.name: root.pageName
                onClicked: root.opened()
            }
            FocusFrame {
                s: root.s
                shown: body.visualFocus || more.visualFocus
                pill: true
            }
        }
    }

    Component {
        id: fluent

        Item {
            id: button

            readonly property bool divided: root.split && root.toggles
            readonly property real arrow: button.divided ? root.arrowWidth : 0
            readonly property real r: Look.corner(QsLook.tileCut(root.size), width / root.s, height / root.s) * root.s
            readonly property real seam: Math.max(1, Math.round(root.s))

            scale: body.down || more.down ? 0.97 : 1
            Behavior on scale {
                NumberAnimation { duration: QsLook.dur(body.down || more.down ? 70 : 167); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.fluentIn }
            }

            Rectangle {
                id: main
                anchors.left: parent.left
                anchors.top: parent.top
                anchors.bottom: parent.bottom
                width: parent.width - (button.divided ? button.arrow + button.seam : 0)
                topLeftRadius: button.r
                bottomLeftRadius: button.r
                topRightRadius: button.divided ? 0 : button.r
                bottomRightRadius: button.divided ? 0 : button.r
                color: QsLook.tileFill(root.on, body.hovered, body.down)
                border.width: root.on ? 0 : Math.max(1, Math.round(root.s))
                border.color: QsLook.tileStroke(root.on)
                Behavior on color {
                    ColorAnimation { duration: QsLook.dur(QsLook.stateMs) }
                }
            }
            Rectangle {
                id: side
                visible: button.divided
                anchors.right: parent.right
                anchors.top: parent.top
                anchors.bottom: parent.bottom
                width: button.arrow
                topRightRadius: button.r
                bottomRightRadius: button.r
                color: QsLook.tileFill(root.on, more.hovered, more.down)
                border.width: root.on ? 0 : Math.max(1, Math.round(root.s))
                border.color: QsLook.tileStroke(root.on)
                Behavior on color {
                    ColorAnimation { duration: QsLook.dur(QsLook.stateMs) }
                }
            }

            Glyph {
                anchors.centerIn: main
                px: 20
            }
            Symbol {
                visible: button.divided || (root.split && !root.toggles)
                anchors.centerIn: button.divided ? side : undefined
                anchors.right: button.divided ? undefined : parent.right
                anchors.rightMargin: Math.round(8 * root.s)
                anchors.verticalCenter: button.divided ? undefined : parent.verticalCenter
                s: root.s
                name: "chevron_right"
                size: 16
                ink: QsLook.tileInk(root.on)
            }

            Target {
                id: body
                anchors.fill: main
                switching: true
                Accessible.name: root.toggles ? root.label : root.pageName
                onClicked: root.act()
            }
            Target {
                id: more
                visible: button.divided
                anchors.fill: side
                switching: false
                Accessible.name: root.pageName
                onClicked: root.opened()
            }
            FocusFrame {
                s: root.s
                shown: body.visualFocus || more.visualFocus
                cut: QsLook.tileCut(root.size) * root.s
            }
            CornerTip {
                s: root.s
                visible: (body.hovered || more.hovered) && root.detail.length > 0
                text: root.detail
            }
        }
    }
}
