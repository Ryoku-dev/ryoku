import QtQuick
import QtQuick.Controls
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "Singletons"

// Where the catalogue comes from: the official Ryostore sources, always there,
// and the stores the user added, each one switchable and removable.
Item {
    id: view

    property bool reducedMotion: false
    property string confirmId: ""

    readonly property string guideUrl: "https://github.com/ryoku-dev/ryostore-revamped/blob/main/docs/host-a-store.md"

    signal addRequested()

    component SectionLabel: Row {
        property string label: ""
        property int count: 0
        spacing: Tokens.s2
        Text {
            text: parent.label
            color: Tokens.inkDim
            font.family: Tokens.mono
            font.pixelSize: Tokens.fMicro
            font.letterSpacing: Tokens.trackLabel
        }
        Text {
            text: String(parent.count).padStart(2, "0")
            color: Tokens.inkFaint
            font.family: Tokens.mono
            font.pixelSize: Tokens.fMicro
            font.letterSpacing: Tokens.trackLabel
        }
    }

    component SourceRow: Rectangle {
        id: row
        required property var modelData
        readonly property var source: row.modelData
        readonly property bool removed: row.source.status === "removed"
        readonly property bool failed: row.source.status === "error"
        readonly property bool busy: Sources.busyId === row.source.id
        readonly property bool confirming: view.confirmId === row.source.id
        readonly property string rowError: Sources.changeErrorId === row.source.id ? Sources.changeError : ""

        width: parent ? parent.width : 0
        implicitHeight: rowBody.implicitHeight + Tokens.s4 * 2
        radius: Tokens.radius
        color: rowHover.hovered ? Tokens.tint5 : Tokens.paper
        border.width: Tokens.border
        border.color: row.failed ? Tokens.alert : (rowHover.hovered ? Tokens.lineStrong : Tokens.line)
        opacity: row.source.enabled || row.source.official ? 1 : 0.72
        Behavior on color { enabled: !view.reducedMotion; ColorAnimation { duration: Tokens.snap } }
        Behavior on opacity { enabled: !view.reducedMotion; NumberAnimation { duration: Tokens.snap } }
        HoverHandler { id: rowHover }

        Column {
            id: rowBody
            anchors { left: parent.left; right: parent.right; top: parent.top; margins: Tokens.s4 }
            spacing: Tokens.s2

            Item {
                width: parent.width
                height: Math.max(textCol.implicitHeight, controls.implicitHeight)

                Column {
                    id: textCol
                    anchors { left: parent.left; right: controls.left; rightMargin: Tokens.s4; verticalCenter: parent.verticalCenter }
                    spacing: Tokens.s1

                    Row {
                        spacing: Tokens.s2
                        Text {
                            anchors.verticalCenter: parent.verticalCenter
                            text: row.source.name
                            color: Tokens.ink
                            font.family: Tokens.display
                            font.pixelSize: Tokens.fRow
                            font.weight: Font.Medium
                        }
                        Repeater {
                            model: {
                                const chips = [];
                                if (row.source.official)
                                    chips.push(I18n.tr("OFFICIAL"));
                                else
                                    chips.push(I18n.tr("COMMUNITY"));
                                if (row.removed)
                                    chips.push(I18n.tr("REMOVED"));
                                else if (row.failed)
                                    chips.push(I18n.tr("UNREACHABLE"));
                                else if (!row.source.enabled || row.source.status === "disabled")
                                    chips.push(I18n.tr("OFF"));
                                return chips;
                            }
                            delegate: Rectangle {
                                required property string modelData
                                required property int index
                                anchors.verticalCenter: parent.verticalCenter
                                width: chipText.implicitWidth + Tokens.s2 * 2
                                height: chipText.implicitHeight + 4
                                radius: Tokens.radius
                                color: index === 0 && row.source.official ? Tokens.tint10 : "transparent"
                                border.width: Tokens.border
                                border.color: modelData === I18n.tr("UNREACHABLE") ? Tokens.alert : Tokens.line
                                Text {
                                    id: chipText
                                    anchors.centerIn: parent
                                    text: parent.modelData
                                    color: parent.modelData === I18n.tr("UNREACHABLE") ? Tokens.alert : Tokens.inkDim
                                    font.family: Tokens.mono
                                    font.pixelSize: Tokens.fTiny
                                    font.letterSpacing: Tokens.trackLabel
                                }
                            }
                        }
                    }
                    Text {
                        width: parent.width
                        text: row.source.url
                        color: Tokens.inkMuted
                        font.family: Tokens.mono
                        font.pixelSize: Tokens.fTiny
                        elide: Text.ElideMiddle
                    }
                    Text {
                        width: parent.width
                        visible: text !== ""
                        text: {
                            const parts = [];
                            if (row.source.description !== "")
                                parts.push(row.source.description);
                            if (row.source.items > 0)
                                parts.push(row.source.items === 1 ? I18n.tr("1 item") : I18n.tr("%1 items").arg(row.source.items));
                            if (row.source.installed > 0)
                                parts.push(I18n.tr("%1 installed").arg(row.source.installed));
                            if (row.source.categories.length > 0)
                                parts.push(row.source.categories.map(id => {
                                    const known = Store.category(id);
                                    return known && known.name ? I18n.tr(String(known.name)) : id;
                                }).join(", "));
                            return parts.join("  ·  ");
                        }
                        color: Tokens.inkDim
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                        wrapMode: Text.Wrap
                        maximumLineCount: 2
                        elide: Text.ElideRight
                    }
                }

                Row {
                    id: controls
                    anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                    spacing: Tokens.s3
                    visible: !row.source.official && !row.removed

                    Sw {
                        objectName: "ryostore-source-switch-" + row.source.id
                        anchors.verticalCenter: parent.verticalCenter
                        visible: !row.confirming
                        on: row.source.enabled
                        enabled: Sources.busyId === ""
                        onToggled: v => Sources.setEnabled(row.source.id, v)
                        Accessible.role: Accessible.CheckBox
                        Accessible.name: I18n.tr("Use %1").arg(row.source.name)
                        Accessible.checked: on
                    }
                    Btn {
                        anchors.verticalCenter: parent.verticalCenter
                        visible: !row.confirming
                        compact: true
                        text: I18n.tr("REMOVE")
                        armed: Sources.busyId === ""
                        motionEnabled: !view.reducedMotion
                        onAct: view.confirmId = row.source.id
                    }
                    Btn {
                        anchors.verticalCenter: parent.verticalCenter
                        visible: row.confirming
                        compact: true
                        primary: true
                        text: I18n.tr("REMOVE SOURCE")
                        armed: Sources.busyId === ""
                        motionEnabled: !view.reducedMotion
                        onAct: {
                            view.confirmId = "";
                            Sources.remove(row.source.id);
                        }
                    }
                    Btn {
                        anchors.verticalCenter: parent.verticalCenter
                        visible: row.confirming
                        compact: true
                        text: I18n.tr("KEEP")
                        motionEnabled: !view.reducedMotion
                        onAct: view.confirmId = ""
                    }
                }
            }

            Text {
                width: parent.width
                visible: row.confirming
                text: I18n.tr("Its items stop showing here. What you installed from it keeps working, without updates.")
                color: Tokens.ink
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                wrapMode: Text.Wrap
            }
            Text {
                width: parent.width
                visible: text !== ""
                text: row.rowError !== "" ? row.rowError
                      : (row.failed && row.source.error !== "" ? row.source.error : row.source.note)
                color: row.rowError !== "" || row.failed ? Tokens.alert : Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                wrapMode: Text.Wrap
            }
            Text {
                visible: row.busy
                text: I18n.tr("UPDATING THE CATALOGUE")
                color: Tokens.inkDim
                font.family: Tokens.mono
                font.pixelSize: Tokens.fTiny
                font.letterSpacing: Tokens.trackLabel
            }
        }
    }

    Flickable {
        id: scroller
        anchors.fill: parent
        anchors.leftMargin: Tokens.s6
        anchors.rightMargin: Tokens.s6
        contentWidth: width
        contentHeight: content.implicitHeight + Tokens.s6
        boundsBehavior: Flickable.StopAtBounds
        clip: true

        Column {
            id: content
            width: scroller.width
            topPadding: Tokens.s5
            spacing: Tokens.s3

            Item {
                width: parent.width
                height: Math.max(introText.implicitHeight, introActions.implicitHeight)

                Text {
                    id: introText
                    anchors { left: parent.left; right: introActions.left; rightMargin: Tokens.s5; verticalCenter: parent.verticalCenter }
                    text: I18n.tr("Ryostore builds its catalogue from these stores. Official ones are run by Ryoku; anything else is run by whoever hosts it.")
                    color: Tokens.inkDim
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                    wrapMode: Text.Wrap
                }
                Row {
                    id: introActions
                    anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                    spacing: Tokens.s2
                    Btn {
                        objectName: "ryostore-sources-guide"
                        text: I18n.tr("HOST A STORE ↗")
                        motionEnabled: !view.reducedMotion
                        onAct: Qt.openUrlExternally(view.guideUrl)
                        Accessible.role: Accessible.Link
                        Accessible.name: I18n.tr("How to host a store")
                    }
                    Btn {
                        objectName: "ryostore-sources-add"
                        text: I18n.tr("ADD A SOURCE")
                        primary: true
                        armed: Sources.busyId === ""
                        motionEnabled: !view.reducedMotion
                        onAct: view.addRequested()
                    }
                }
            }

            Item { width: 1; height: Tokens.s2 }

            SectionLabel { label: I18n.tr("OFFICIAL"); count: Sources.official.length }
            Repeater {
                model: Sources.official
                delegate: SourceRow { }
            }

            Item { width: 1; height: Tokens.s3 }

            SectionLabel { label: I18n.tr("YOUR SOURCES"); count: Sources.mine.length }
            Repeater {
                model: Sources.mine
                delegate: SourceRow { }
            }
            Rectangle {
                width: parent.width
                visible: Sources.mine.length === 0
                height: emptyText.implicitHeight + Tokens.s5 * 2
                radius: Tokens.radius
                color: "transparent"
                border.width: Tokens.border
                border.color: Tokens.lineSoft
                Text {
                    id: emptyText
                    anchors { left: parent.left; right: parent.right; verticalCenter: parent.verticalCenter; margins: Tokens.s5 }
                    text: I18n.tr("None yet. A source is a store someone else hosts; add one only when you trust who runs it.")
                    color: Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                    wrapMode: Text.Wrap
                }
            }

            Text {
                width: parent.width
                visible: Sources.error !== ""
                text: Sources.error
                color: Tokens.alert
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                wrapMode: Text.Wrap
            }
        }

        WheelScroll { }
        ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
    }
}
