pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Layouts
import QtQuick.Effects
import "Singletons"
import "../stage/Singletons" as StageCfg
import "../../components"
import Ryoku.Ui.Singletons

// The Edit widgets bar: one floating instrument placed in the output's work area
// while the desktop is lifted for editing. Its host window sets exclusiveZone 0,
// so the bar sits clear of the frame band, the bar-style island and the dock on
// every bar style; it rests bottom-centre, above the dock. Its compact controls
// -- the grid snap and step, then Reset/Done -- flank one Widgets button that
// opens WidgetPicker, an attached panel that grows out of the bar: a category
// rail (Ryoku / Shima / Python / each installed plugin set) and a two-column
// grid of widget cards, searchable, wheel- and keyboard-driven. The session
// state (selection, dirty, escape ladder) stays in StageSession; this owns only
// the bar and reports host actions as signals. Grid snap and step live on
// stage.json so an unknown key never reaches the Hub's save.
Item {
    id: ed
    anchors.fill: parent

    property string monitor: ""
    // The roster: [{ id, label, icon, enabled, group }].
    property var items: []
    // Extra bottom clearance (px) so the bar clears a dock the work area does not
    // exclude; the host computes it from the active bar style's dock.
    property real dockClearance: 0

    signal done()
    signal addToggle(string id)
    // Forwarded from the picker: open a widget's inspector (Customize sheet).
    signal customize(string id)

    readonly property var ses: StageCfg.StageSession
    readonly property var grid: StageCfg.Config
    // Exposed so the host window can mask input to just the bar (RecordIsland
    // idiom): clicks off the bar fall through to the widgets for dragging.
    property alias barItem: bar

    // The picker panel is on when the Widgets button opens it. The host window
    // reads this to widen the input mask to the whole surface while it is up (a
    // click off the panel closes it) and to let the surface take keyboard focus
    // for the panel's search field and Up/Down/Space/Esc navigation.
    property bool pickerOpen: false


    // A compact tool: a glyph over an optional value, a quiet tile that washes on
    // hover and inverts to a bone plate when it reports an on state.
    component Tool: Item {
        id: tb
        property string icon: ""
        property string value: ""
        property bool on: false
        signal act()
        implicitWidth: Math.max(Theme.ctlH + 8, tbRow.implicitWidth + Theme.s3)
        implicitHeight: Theme.ctlH + 8
        readonly property color content: tb.on ? Theme.inkOnBone
            : (tbMa.containsMouse ? Theme.ink : Theme.inkDim)
        scale: tbMa.pressed ? 0.94 : 1
        Behavior on scale { NumberAnimation { duration: Theme.quick; easing.type: Theme.ease } }
        Rectangle {
            anchors.fill: parent
            radius: Theme.radiusTile
            color: tb.on ? Theme.bone
                : tbMa.pressed ? Theme.tilePress
                : tbMa.containsMouse ? Theme.tileHover : Theme.tile
            border.width: 1
            border.color: tb.on ? Theme.bone : Theme.line
            Behavior on color { ColorAnimation { duration: Theme.quick } }
        }
        Row {
            id: tbRow
            anchors.centerIn: parent
            spacing: Theme.s1
            MaterialIcon {
                anchors.verticalCenter: parent.verticalCenter
                text: tb.icon
                font.pixelSize: 18
                fill: tb.on ? 1 : 0
                color: tb.content
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                visible: tb.value.length > 0
                text: tb.value
                color: tb.content
                font.family: Theme.mono
                font.pixelSize: Theme.fSmall
                font.weight: Font.DemiBold
            }
        }
        MouseArea {
            id: tbMa
            anchors.fill: parent
            hoverEnabled: true
            cursorShape: Qt.PointingHandCursor
            onClicked: tb.act()
        }
    }

    // A hairline divider between clusters.
    component Div: Rectangle {
        Layout.preferredWidth: 1
        Layout.preferredHeight: Theme.ctlH
        Layout.alignment: Qt.AlignVCenter
        color: Theme.line
    }

    // With the picker open the whole surface takes input (docs/stage.md): a press
    // anywhere off the panel and the bar closes it, and no widget drags mid-pick.
    MouseArea {
        anchors.fill: parent
        visible: ed.pickerOpen
        acceptedButtons: Qt.AllButtons
        onPressed: ed.pickerOpen = false
    }

    MultiEffect {
        source: bar
        anchors.fill: bar
        visible: !Performance.shadowsDisabled
        shadowEnabled: true
        shadowColor: Theme.shadow
        shadowBlur: 1.0
        shadowVerticalOffset: 10
        blurMax: 40
        autoPaddingEnabled: true
    }

    Rectangle {
        id: bar
        x: Math.round((ed.width - width) / 2)
        y: ed.height - height - Theme.s3 - (ed.dockClearance > 0 ? ed.dockClearance + Theme.s3 : 0)
        width: Math.min(ed.width - Theme.s5 * 2, row.implicitWidth + Theme.s4 * 2)
        height: Theme.s7 + Theme.s1
        radius: Theme.menuRadius
        color: Theme.surface
        border.width: 1
        border.color: Theme.line

        // Swallow presses on the bar chrome so a click between controls never
        // leaks to a widget or the wallpaper beneath the lifted desktop.
        MouseArea { anchors.fill: parent; acceptedButtons: Qt.AllButtons }

        RowLayout {
            id: row
            anchors.fill: parent
            anchors.leftMargin: Theme.s4
            anchors.rightMargin: Theme.s4
            spacing: Theme.s3

            // Masthead: kanji seal + tracked scope, the desktop-menu idiom.
            RowLayout {
                Layout.alignment: Qt.AlignVCenter
                spacing: Theme.s2
                Text {
                    text: "\u90e8\u54c1"
                    color: Theme.faint
                    font.family: Theme.fontJp
                    font.pixelSize: Theme.fBody
                }
                Text {
                    text: I18n.tr("Edit widgets").toUpperCase()
                    color: Theme.inkDim
                    font.family: Theme.font
                    font.pixelSize: Theme.fMicro
                    font.weight: Font.DemiBold
                    font.letterSpacing: Theme.trackMark
                }
            }

            Div {}

            Tool {
                Layout.alignment: Qt.AlignVCenter
                icon: "grid_on"
                on: ed.grid.editGridSnap
                onAct: ed.grid.toggleEditGridSnap()
            }
            Tool {
                Layout.alignment: Qt.AlignVCenter
                icon: "grid_4x4"
                value: ed.grid.editGridSize + ""
                onAct: ed.grid.cycleEditGridSize()
            }

            Div {}

            // The Widgets button: opens the attached picker panel. It reads as on
            // -- a bone plate -- while the panel is up, the sidebar inversion idiom.
            Item {
                id: widgetsBtn
                Layout.alignment: Qt.AlignVCenter
                implicitWidth: wbRow.implicitWidth + Theme.s4
                implicitHeight: Theme.ctlH + 8
                readonly property bool on: ed.pickerOpen
                readonly property color content: widgetsBtn.on ? Theme.inkOnBone
                    : (wbMa.containsMouse ? Theme.ink : Theme.inkDim)
                scale: wbMa.pressed ? 0.95 : 1
                Behavior on scale { NumberAnimation { duration: Theme.quick; easing.type: Theme.ease } }
                Rectangle {
                    anchors.fill: parent
                    radius: Theme.radiusTile
                    color: widgetsBtn.on ? Theme.bone
                        : wbMa.pressed ? Theme.tilePress
                        : wbMa.containsMouse ? Theme.tileHover : Theme.tile
                    border.width: 1
                    border.color: widgetsBtn.on ? Theme.bone : Theme.line
                    Behavior on color { ColorAnimation { duration: Theme.quick } }
                }
                Row {
                    id: wbRow
                    anchors.centerIn: parent
                    spacing: Theme.s2
                    MaterialIcon {
                        anchors.verticalCenter: parent.verticalCenter
                        text: "widgets"
                        font.pixelSize: 18
                        fill: widgetsBtn.on ? 1 : 0
                        color: widgetsBtn.content
                    }
                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("Widgets")
                        color: widgetsBtn.content
                        font.family: Theme.font
                        font.pixelSize: Theme.fSmall
                        font.weight: Font.DemiBold
                    }
                }
                MouseArea {
                    id: wbMa
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: Qt.PointingHandCursor
                    onClicked: ed.pickerOpen = !ed.pickerOpen
                }
            }

            Div {}

            Tool {
                Layout.alignment: Qt.AlignVCenter
                icon: "restart_alt"
                opacity: ed.ses.dirty ? 1 : 0
                enabled: ed.ses.dirty
                onAct: ed.ses.reset()
                Behavior on opacity { NumberAnimation { duration: Theme.quick } }
            }

            // Done: the one bone plate, spelled out.
            Item {
                id: doneBtn
                Layout.alignment: Qt.AlignVCenter
                implicitWidth: doneText.implicitWidth + Theme.s5
                implicitHeight: Theme.ctlH + 8
                scale: doneMa.pressed ? 0.95 : 1
                Behavior on scale { NumberAnimation { duration: Theme.quick; easing.type: Theme.ease } }
                Rectangle {
                    anchors.fill: parent
                    radius: Theme.radiusTile
                    color: Theme.bone
                    opacity: doneMa.containsMouse ? 1 : 0.92
                    Behavior on opacity { NumberAnimation { duration: Theme.quick } }
                }
                Text {
                    id: doneText
                    anchors.centerIn: parent
                    text: I18n.tr("Done")
                    color: Theme.inkOnBone
                    font.family: Theme.font
                    font.pixelSize: Theme.fSmall
                    font.weight: Font.DemiBold
                }
                MouseArea {
                    id: doneMa
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: Qt.PointingHandCursor
                    onClicked: ed.done()
                }
            }
        }
    }

    // The picker grows out of the bar's top edge into the free work area above it,
    // sized and capped to what the surface allows so nothing truncates. It floats
    // over the lifted desktop, so it casts (the popout idiom).
    MultiEffect {
        source: picker
        anchors.fill: picker
        visible: picker.visible && !Performance.shadowsDisabled
        shadowEnabled: true
        shadowColor: Theme.shadow
        shadowBlur: 1.0
        shadowVerticalOffset: 10
        blurMax: 40
        autoPaddingEnabled: true
    }
    WidgetPicker {
        id: picker
        visible: ed.pickerOpen
        items: ed.items
        anchors.horizontalCenter: bar.horizontalCenter
        anchors.bottom: bar.top
        anchors.bottomMargin: Theme.s2
        maxWidth: ed.width - Theme.s5 * 2
        maxPanelHeight: bar.y - Theme.s2 - Theme.s5
        onToggle: id => ed.addToggle(id)
        onRequestClose: ed.pickerOpen = false
        onCustomize: id => ed.customize(id)
    }
}
