import QtQuick
import Quickshell
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "Singletons"
import "lib/apps.js" as AppsLogic
import "lib/store.js" as StoreLogic

// One of Ryoku's own apps: what it is, the version on this computer, its size,
// whether an update (or a prerelease) waits, and the actions that apply.
Rectangle {
    id: card

    required property var app
    property bool reducedMotion: false
    property bool confirming: false

    readonly property string key: "ryoku:" + card.app.id
    readonly property bool busy: Jobs.app.busyFor(card.key)
    readonly property bool otherBusy: Jobs.app.running && !card.busy
    readonly property var actions: AppsLogic.ryokuActions(card.app)
    readonly property string size: AppsLogic.formatBytes(card.app.sizeBytes)
    readonly property bool prerelease: card.app.channel === "prerelease"
    // read through the application list, so the lookup runs again once the
    // entries are scanned (and after an install adds the app's entry)
    readonly property var entry: {
        const id = String(card.app.desktopId || "").replace(/\.desktop$/, "");
        const scanned = DesktopEntries.applications ? DesktopEntries.applications.values : [];
        return id !== "" && scanned.length >= 0 ? DesktopEntries.byId(id) : null;
    }
    readonly property string icon: {
        const name = card.entry && card.entry.icon ? String(card.entry.icon) : card.app.id;
        return Quickshell.iconPath(name, true);
    }
    readonly property bool hovered: hover.hovered

    function doing(action) {
        if (action === "update")
            return I18n.tr("Updating %1").arg(card.app.name);
        if (action === "remove")
            return I18n.tr("Removing %1").arg(card.app.name);
        return I18n.tr("Installing %1").arg(card.app.name);
    }

    function run(action) {
        card.confirming = false;
        Jobs.app.start(card.key, action, card.app.name, AppsLogic.ryokuCommand(card.app, action));
    }

    function open() {
        if (!card.entry)
            return;
        if (card.entry.command && card.entry.command.length > 0)
            Spawn.runApp(card.entry.command, { label: card.app.id });
        else
            card.entry.execute();
    }

    implicitHeight: body.implicitHeight + Tokens.s5 * 2
    radius: Tokens.radius
    color: card.hovered ? Tokens.tint5 : Tokens.paper
    border.width: card.app.update ? Tokens.border * 2 : Tokens.border
    border.color: card.app.update ? Tokens.bone : (card.hovered ? Tokens.lineStrong : Tokens.line)
    Behavior on color { enabled: !card.reducedMotion; ColorAnimation { duration: Tokens.snap } }
    Behavior on border.color { enabled: !card.reducedMotion; ColorAnimation { duration: Tokens.snap } }

    Accessible.role: Accessible.Grouping
    Accessible.name: card.app.name

    HoverHandler { id: hover }

    Column {
        id: body
        anchors { left: parent.left; right: parent.right; top: parent.top; margins: Tokens.s5 }
        spacing: Tokens.s4

        Row {
            width: parent.width
            spacing: Tokens.s4

            Rectangle {
                id: iconPlate
                width: Tokens.px(64)
                height: Tokens.px(64)
                radius: Tokens.radius
                color: Tokens.paperLift
                border.width: Tokens.border
                border.color: Tokens.line
                scale: card.hovered && !card.reducedMotion ? 1.04 : 1
                Behavior on scale { enabled: !card.reducedMotion; NumberAnimation { duration: Tokens.move; easing.type: Tokens.ease } }

                Image {
                    anchors.centerIn: parent
                    width: parent.width - Tokens.s3 * 2
                    height: width
                    source: card.icon
                    sourceSize: Qt.size(width * 2, height * 2)
                    fillMode: Image.PreserveAspectFit
                    asynchronous: true
                    visible: status === Image.Ready
                }
                Text {
                    anchors.centerIn: parent
                    visible: card.icon === ""
                    text: StoreLogic.initials(null, card.app.name)
                    color: Tokens.inkMuted
                    font.family: Tokens.mono
                    font.pixelSize: Tokens.fBody
                    font.weight: Font.Medium
                }
            }

            Column {
                width: parent.width - iconPlate.width - parent.spacing
                anchors.verticalCenter: parent.verticalCenter
                spacing: Tokens.s1

                Text {
                    width: parent.width
                    text: card.app.name
                    color: Tokens.ink
                    font.family: Tokens.display
                    font.pixelSize: Tokens.px(22)
                    font.weight: Font.Medium
                    elide: Text.ElideRight
                }
                Text {
                    objectName: "ryostore-ryoku-app-state-" + card.app.id
                    text: card.app.update
                          ? (card.prerelease ? I18n.tr("UPDATE · PRERELEASE") : I18n.tr("UPDATE AVAILABLE"))
                          : (card.app.installed ? I18n.tr("INSTALLED") : I18n.tr("NOT INSTALLED"))
                    color: card.app.update ? Tokens.ink : Tokens.inkMuted
                    font.family: Tokens.mono
                    font.pixelSize: Tokens.fTiny
                    font.weight: card.app.update ? Font.Medium : Font.Normal
                    font.letterSpacing: Tokens.trackLabel
                }
            }
        }

        Text {
            width: parent.width
            text: card.app.summary
            visible: text !== ""
            color: Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            wrapMode: Text.Wrap
            maximumLineCount: 3
            elide: Text.ElideRight
            height: Math.max(implicitHeight, fontMetrics.height * 3)
            FontMetrics { id: fontMetrics; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall }
        }

        // the facts, ruled like a spec sheet
        Column {
            width: parent.width
            spacing: 0

            Repeater {
                model: [
                    { label: I18n.tr("VERSION"), value: card.app.installed && card.app.version !== "" ? AppsLogic.displayVersion(card.app.version) : "–" },
                    { label: card.app.installed ? I18n.tr("LATEST") : I18n.tr("AVAILABLE"),
                      value: card.app.latest !== ""
                             ? AppsLogic.displayVersion(card.app.latest) + (card.prerelease ? "  " + I18n.tr("PRERELEASE") : "")
                             : "–" },
                    { label: I18n.tr("SIZE"), value: card.app.installed && card.size !== "" ? card.size : "–" }
                ]
                delegate: Item {
                    required property var modelData
                    width: parent.width
                    height: 28

                    Rectangle {
                        anchors { left: parent.left; right: parent.right; top: parent.top }
                        height: Tokens.border
                        color: Tokens.lineSoft
                    }
                    Text {
                        anchors { left: parent.left; verticalCenter: parent.verticalCenter }
                        text: parent.modelData.label
                        color: Tokens.inkMuted
                        font.family: Tokens.mono
                        font.pixelSize: Tokens.fTiny
                        font.letterSpacing: Tokens.trackLabel
                    }
                    Text {
                        anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                        text: parent.modelData.value
                        color: Tokens.ink
                        font.family: Tokens.mono
                        font.pixelSize: Tokens.fMicro
                    }
                }
            }
        }

        // the actions, or the removal asked once more
        Item {
            width: parent.width
            height: card.confirming ? confirmBox.implicitHeight : actionRow.implicitHeight

            // a narrow card wraps its actions to a second line instead of clipping them
            Flow {
                id: actionRow
                visible: !card.confirming
                width: parent.width
                spacing: Tokens.s2

                Repeater {
                    model: card.actions
                    delegate: Btn {
                        required property string modelData
                        required property int index
                        objectName: "ryostore-ryoku-app-" + modelData + "-" + card.app.id
                        primary: index === 0 && modelData !== "open" && modelData !== "remove"
                        armed: modelData === "open" ? card.entry !== null : !Jobs.app.running
                        motionEnabled: !card.reducedMotion
                        text: modelData === "install" ? I18n.tr("INSTALL")
                              : (modelData === "update" ? I18n.tr("UPDATE")
                                 : (modelData === "open" ? I18n.tr("OPEN") : I18n.tr("REMOVE")))
                        onAct: {
                            if (modelData === "open")
                                card.open();
                            else if (modelData === "remove")
                                card.confirming = true;
                            else
                                card.run(modelData);
                        }
                        Accessible.role: Accessible.Button
                        Accessible.name: text + " " + card.app.name
                    }
                }
            }

            Column {
                id: confirmBox
                visible: card.confirming
                width: parent.width
                spacing: Tokens.s2

                Text {
                    width: parent.width
                    text: card.size !== ""
                          ? I18n.tr("Remove %1 from this computer? %2 comes back.").arg(card.app.name).arg(card.size)
                          : I18n.tr("Remove %1 from this computer?").arg(card.app.name)
                    color: Tokens.ink
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                    wrapMode: Text.Wrap
                }
                Row {
                    spacing: Tokens.s2
                    Btn {
                        objectName: "ryostore-ryoku-app-confirm-remove-" + card.app.id
                        text: I18n.tr("REMOVE")
                        primary: true
                        armed: !Jobs.app.running
                        motionEnabled: !card.reducedMotion
                        onAct: card.run("remove")
                    }
                    Btn {
                        text: I18n.tr("KEEP IT")
                        motionEnabled: !card.reducedMotion
                        onAct: card.confirming = false
                    }
                }
            }
        }

        Text {
            width: parent.width
            visible: card.otherBusy && !card.confirming
            text: I18n.tr("Waiting for %1 to finish.").arg(Jobs.app.label)
            color: Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: Tokens.fTiny
            elide: Text.ElideRight
        }

        JobLine {
            width: parent.width
            target: card.key
            doing: card.doing(Jobs.app.action)
            reducedMotion: card.reducedMotion
        }
    }
}
