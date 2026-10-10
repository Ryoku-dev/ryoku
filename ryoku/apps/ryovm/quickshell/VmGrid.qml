import QtQuick
import QtQuick.Controls
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "Singletons"

// The yard: a scrollable column of 64-tall machine rows bound to Vm.vms. Picking
// a row selects it for the stage on the right. The populate cascade survives:
// it is the roll-call, and the flaps are already doing it; an empty yard invites
// a build.
Item {
    id: g

    property string filter: ""
    signal buildRequested()

    readonly property var shown: {
        if (g.filter.length === 0)
            return Vm.vms;
        var f = g.filter.toLowerCase();
        return Vm.vms.filter(v => v.name.toLowerCase().indexOf(f) >= 0 || (v.guest || "").toLowerCase().indexOf(f) >= 0);
    }

    ListView {
        id: list
        anchors.fill: parent
        visible: g.shown.length > 0
        clip: true
        spacing: 0
        model: g.shown
        cacheBuffer: 800
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollRail {}

        // roll-call on first population only (never on scroll-back or refresh).
        populate: Transition {
            id: popT
            SequentialAnimation {
                PropertyAction { property: "opacity"; value: 0 }
                PauseAnimation { duration: 40 * Math.min(popT.ViewTransition.index, 8) }
                NumberAnimation { property: "opacity"; from: 0; to: 1; duration: Tokens.swap; easing.type: Tokens.ease }
            }
        }
        add: Transition {
            NumberAnimation { property: "opacity"; from: 0; to: 1; duration: Tokens.swap; easing.type: Tokens.ease }
        }

        delegate: Rectangle {
            id: slot
            required property var modelData
            required property int index
            width: list.width
            height: 66
            color: Vm.selectedName === slot.modelData.name ? Tokens.bone : "transparent"
            antialiasing: false
            Rectangle {
                anchors.bottom: parent.bottom
                width: parent.width
                height: 1
                color: Vm.selectedName === slot.modelData.name ? Tokens.inkOnBone : Tokens.lineSoft
                opacity: 0.35
            }
            Column {
                anchors.left: parent.left
                anchors.leftMargin: Tokens.s3
                anchors.right: stats.left
                anchors.rightMargin: Tokens.s2
                anchors.verticalCenter: parent.verticalCenter
                spacing: 3
                Text {
                    width: parent.width
                    text: slot.modelData.name
                    elide: Text.ElideRight
                    color: Vm.selectedName === slot.modelData.name ? Tokens.inkOnBone : Tokens.ink
                    font.family: Tokens.ui
                    font.pixelSize: 12
                    font.weight: Font.Medium
                }
                Text {
                    width: parent.width
                    text: (slot.modelData.running ? I18n.tr("RUNNING") : I18n.tr("STOPPED"))
                        + "  /  " + (slot.modelData.os || slot.modelData.guest || "-")
                    elide: Text.ElideRight
                    color: Vm.selectedName === slot.modelData.name ? Tokens.inkOnBone : Tokens.inkMuted
                    opacity: Vm.selectedName === slot.modelData.name ? 0.72 : 1
                    font.family: Tokens.mono
                    font.pixelSize: 9
                }
            }
            Item {
                id: stats
                anchors.right: parent.right
                anchors.rightMargin: Tokens.s3
                anchors.verticalCenter: parent.verticalCenter
                width: 116
                height: 42
                MetricSparkline {
                    anchors { left: parent.left; right: parent.left; rightMargin: -72; top: parent.top; bottom: parent.bottom }
                    values: Vm.series(slot.modelData.name, "cpu")
                    fixedMax: 100
                    stroke: Vm.selectedName === slot.modelData.name ? Tokens.inkOnBone : Tokens.inkMuted
                }
                Text {
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    text: slot.modelData.cores + "c\\n" + slot.modelData.ram
                    horizontalAlignment: Text.AlignRight
                    color: Vm.selectedName === slot.modelData.name ? Tokens.inkOnBone : Tokens.inkFaint
                    font.family: Tokens.mono
                    font.pixelSize: 8
                }
            }
            MouseArea {
                anchors.fill: parent
                cursorShape: Qt.PointingHandCursor
                onClicked: Vm.select(slot.modelData.name)
            }
        }
    }

    // empty / loading state, anchored on the app mark.
    Column {
        anchors.centerIn: parent
        spacing: Tokens.s4
        width: parent.width - 40
        visible: g.shown.length === 0
        Mark {
            anchors.horizontalCenter: parent.horizontalCenter
            size: 96
        }
        Text {
            anchors.horizontalCenter: parent.horizontalCenter
            horizontalAlignment: Text.AlignHCenter
            width: parent.width
            wrapMode: Text.WordWrap
            text: Vm.vmsLoading ? I18n.tr("Loading your machines")
                : (g.filter.length > 0 ? I18n.tr("No machines match")
                : I18n.tr("No machines yet."))
            color: Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: 12
        }
        Btn {
            anchors.horizontalCenter: parent.horizontalCenter
            visible: !Vm.vmsLoading && g.filter.length === 0
            primary: true
            text: I18n.tr("OPEN CATALOG")
            onAct: g.buildRequested()
        }
    }
}
