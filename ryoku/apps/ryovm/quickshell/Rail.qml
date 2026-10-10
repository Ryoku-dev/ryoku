pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Controls
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "Singletons"
// The harbour rail: ryoport's one always-present chrome. A framed masthead seal,
// the fleet nav (Latin name and kanji seal side by side, the live berth taking
// the sheet's // lead on a bone plate), and a scannable foot plate. The same
// title-block grammar the Hub rail carries, flying ryoport's colours.
Item {
    id: rail

    property string section: "dashboard"
    signal navigate(string key)
    signal openSettings()
    signal requestQuit()
    signal openLog()

    width: Tokens.railW

    // the fleet alerts plus one sticky engine fault: the number on the plate
    // is the number of things the harbour log will show.
    readonly property int alertCount: Remotes.alertCount + (Vm.fault && Vm.fault.length > 0 ? 1 : 0)

    readonly property var groups: [
        { name: I18n.tr("OVERVIEW"), items: [ { key: "dashboard", name: I18n.tr("Dashboard") } ] },
        { name: I18n.tr("FLEET"), items: [ { key: "machines", name: I18n.tr("Machines") }, { key: "remotes", name: I18n.tr("Remotes") }, { key: "passthrough", name: I18n.tr("Looking Glass") } ] },
        { name: I18n.tr("EXPERIMENTAL"), items: [ { key: "consoles", name: I18n.tr("Consoles") } ] }
    ]
    readonly property var jpName: ({ "dashboard": "一覧", "machines": "仮想", "remotes": "遠隔", "passthrough": "透過", "consoles": "接續" })

    Rectangle { anchors.right: parent.right; width: 1; height: parent.height; color: Tokens.line }

    Column {
        id: railHead
        anchors { left: parent.left; right: parent.right; top: parent.top }
        anchors.margins: Tokens.s5
        spacing: Tokens.s4

        Rectangle {
            width: parent.width
            height: 64
            color: "transparent"
            radius: Tokens.radius
            border.width: Tokens.border
            border.color: Tokens.line
            Ticks { }
            Row {
                anchors { left: parent.left; verticalCenter: parent.verticalCenter; leftMargin: Tokens.s4 }
                spacing: Tokens.s3
                Text { text: "力"; color: Tokens.ink; font.family: Tokens.jp; font.pixelSize: 22 }
                Column {
                    spacing: 1
                    anchors.verticalCenter: parent.verticalCenter
                    Text {
                        text: I18n.tr("RYOPORT"); color: Tokens.ink; font.family: Tokens.ui
                        font.pixelSize: 14; font.weight: Font.Medium; font.letterSpacing: 2.4
                    }
                    Text {
                        text: I18n.tr("//HARBOUR_"); color: Tokens.inkMuted
                        font.family: Tokens.mono; font.pixelSize: 10; font.letterSpacing: 1.4
                    }
                }
            }
            Text {
                anchors { right: parent.right; top: parent.top; margins: Tokens.s2 }
                text: "港"; color: Tokens.inkFaint
                font.family: Tokens.jp; font.pixelSize: 13
            }
        }

        // the alerts readout rides under the seal: the count is never a dead
        // number, it is the door to the harbour log.
        Rectangle {
            width: parent.width
            height: 40
            color: "transparent"
            border.width: Tokens.border
            border.color: Tokens.line
            radius: Tokens.radius
            antialiasing: false
            Row {
                anchors { left: parent.left; verticalCenter: parent.verticalCenter; leftMargin: Tokens.s4 }
                spacing: Tokens.s3
                Text {
                    text: I18n.tr("ALERTS"); color: Tokens.inkMuted
                    font.family: Tokens.ui; font.pixelSize: 9
                    font.weight: Font.Medium; font.letterSpacing: 2
                    anchors.verticalCenter: parent.verticalCenter
                }
                Annunciator {
                    anchors.verticalCenter: parent.verticalCenter
                    label: rail.alertCount > 0
                        ? String(rail.alertCount).padStart(2, "0")
                        : I18n.tr("QUIET")
                    lit: rail.alertCount > 0
                    tileW: 54
                }
            }
            Text {
                anchors { right: parent.right; verticalCenter: parent.verticalCenter; rightMargin: Tokens.s4 }
                text: I18n.tr("log"); color: Tokens.inkFaint
                font.family: Tokens.mono; font.pixelSize: 9
            }
            HoverHandler { cursorShape: Qt.PointingHandCursor }
            TapHandler { onTapped: rail.openLog() }
        }
    }

    Flickable {
        id: navFlick
        anchors { left: parent.left; right: parent.right; top: railHead.bottom; bottom: railFoot.top }
        anchors.margins: Tokens.s5
        anchors.topMargin: Tokens.s4
        contentHeight: nav.height
        clip: true
        ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }

        Column {
            id: nav
            width: navFlick.width - 12
            spacing: 0

            Repeater {
                model: rail.groups
                Column {
                    id: grp
                    required property var modelData
                    required property int index
                    width: nav.width
                    spacing: 0

                    Item {
                        width: parent.width
                        height: 30
                        Row {
                            anchors.verticalCenter: parent.verticalCenter
                            spacing: Tokens.s2
                            Text {
                                text: (grp.index + 1 < 10 ? "0" : "") + (grp.index + 1)
                                color: Tokens.inkFaint
                                font.family: Tokens.mono; font.pixelSize: 9
                                anchors.verticalCenter: parent.verticalCenter
                            }
                            Text {
                                text: grp.modelData.name; color: Tokens.inkFaint
                                font.family: Tokens.ui; font.pixelSize: 9
                                font.weight: Font.Medium; font.letterSpacing: 2
                                anchors.verticalCenter: parent.verticalCenter
                            }
                            Rectangle {
                                width: Math.max(0, nav.width - 130); height: 1; color: Tokens.lineSoft
                                anchors.verticalCenter: parent.verticalCenter
                            }
                            Rectangle {
                                width: 1; height: 5; color: Tokens.line
                                anchors.verticalCenter: parent.verticalCenter
                                anchors.verticalCenterOffset: -2
                            }
                        }
                    }

                    Repeater {
                        model: grp.modelData.items
                        Item {
                            id: navItem
                            required property var modelData
                            width: nav.width
                            height: 34
                            readonly property bool sel: rail.section === modelData.key

                            Rectangle {
                                anchors.fill: parent
                                anchors.topMargin: 1; anchors.bottomMargin: 1
                                radius: Tokens.radius
                                color: navItem.sel ? Tokens.bone : (nh.hovered ? Tokens.tint10 : "transparent")
                                Behavior on color { ColorAnimation { duration: Tokens.snap } }
                            }
                            Row {
                                x: Tokens.s3
                                anchors.verticalCenter: parent.verticalCenter
                                spacing: Tokens.s2
                                Text {
                                    visible: navItem.sel
                                    text: "//"
                                    color: Tokens.inkOnBoneDim
                                    font.family: Tokens.mono; font.pixelSize: 11
                                    anchors.verticalCenter: parent.verticalCenter
                                }
                                Text {
                                    text: navItem.modelData.name
                                    color: navItem.sel ? Tokens.inkOnBone : Tokens.inkDim
                                    font.family: Tokens.ui; font.pixelSize: 14
                                    anchors.verticalCenter: parent.verticalCenter
                                    Behavior on color { ColorAnimation { duration: Tokens.snap } }
                                }
                            }
                            Text {
                                anchors { right: parent.right; rightMargin: Tokens.s3; verticalCenter: parent.verticalCenter }
                                text: rail.jpName[navItem.modelData.key] || ""
                                color: navItem.sel ? Tokens.inkOnBoneDim : Tokens.inkFaint
                                font.family: Tokens.jp; font.pixelSize: 12
                                Behavior on color { ColorAnimation { duration: Tokens.snap } }
                            }
                            HoverHandler { id: nh; cursorShape: Qt.PointingHandCursor }
                            TapHandler { onTapped: rail.navigate(navItem.modelData.key) }
                        }
                    }
                }
            }
        }
    }

    Item {
        id: railFoot
        anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
        anchors.margins: Tokens.s5
        anchors.bottomMargin: Tokens.s4
        height: Tokens.s3 + edition.height + Tokens.s3 + plate.implicitHeight
        Rectangle { anchors { left: parent.left; right: parent.right; top: parent.top } height: 1; color: Tokens.lineSoft }

        Marginalia {
            id: edition
            anchors { left: parent.left; top: parent.top; topMargin: Tokens.s3 }
            index: Version.editionIndex; label: Version.editionNumber
            glyph: "column"; glyph2: ""
            chevrons: false
        }
        // the window controls ride the foot, opposite the edition register, so
        // the head plate stays a clean seal.
        Row {
            anchors { right: parent.right; top: parent.top; topMargin: Tokens.s2 }
            spacing: Tokens.s1
            IconBtn { glyph: "\u2699"; onAct: rail.openSettings() }
            IconBtn { glyph: "\u2715"; onAct: rail.requestQuit() }
        }
        Barcode {
            id: plate
            anchors { left: parent.left; bottom: parent.bottom }
            text: I18n.tr("RYOPORT")
            unit: 1.1
            barHeight: 14
        }
        Text {
            anchors { right: parent.right; bottom: parent.bottom; bottomMargin: 2 }
            text: "+"; color: Tokens.inkFaint
            font.family: Tokens.mono; font.pixelSize: 10
        }
    }
}
