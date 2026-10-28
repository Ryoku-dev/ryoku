pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Quickshell
import Ryoku.Ui
import Ryoku.Ui.Singletons
import stage.services
import stage.modules.common
import stage.modules.common.widgets

ColumnLayout {
    id: root

    required property string screenName
    required property string folderId
    property var refreshKey: null
    property real targetX: NaN
    property real targetY: NaN
    readonly property bool desktopMode: folderId === ""
    property string query: ""
    signal done()
    signal applicationAdded(string appId)

    readonly property bool writable: Persistent.ready && !Persistent.blockWrites
    // The list depends on the search only; each row reads its own placement,
    // so adding or taking out an app never resets the scroll.
    readonly property var applications: {
        const search = root.query.trim().toLocaleLowerCase();
        return Array.from(DesktopEntries.applications.values)
            .filter(app => !app.noDisplay && (!search
                || String(app.name || app.id).toLocaleLowerCase().includes(search)))
            .sort((a, b) => String(a.name || a.id).localeCompare(String(b.name || b.id)));
    }
    onVisibleChanged: {
        if (!visible)
            return;
        root.query = "";
        searchField.text = "";
        Qt.callLater(() => searchField.grabFocus());
    }

    function placement(appId) {
        return DesktopShortcuts.locate(root.screenName, appId);
    }

    function toggle(appId) {
        if (!root.writable)
            return;
        const app = DesktopShortcuts.application(appId);
        if (!app)
            return;
        if (root.desktopMode) {
            if (DesktopShortcuts.add(root.screenName, [app], root.targetX, root.targetY))
                root.applicationAdded(appId);
            return;
        }
        const location = root.placement(appId);
        if (location.where === "folder" && location.folderId === root.folderId) {
            if (DesktopShortcuts.canTakeOut(root.screenName, root.folderId))
                DesktopShortcuts.takeOut(root.screenName, root.folderId, appId);
            return;
        }
        DesktopShortcuts.addToFolder(root.screenName, root.folderId, [app]);
    }

    spacing: Tokens.s2

    Field {
        id: searchField
        Layout.fillWidth: true
        toolbar: true
        placeholder: Translation.tr("Search applications")
        onEdited: root.query = text
        Component.onCompleted: if (root.visible) Qt.callLater(() => grabFocus())
    }

    Item {
        Layout.fillWidth: true
        Layout.preferredHeight: Math.min(Tokens.railW + Tokens.s7,
            Math.max(Tokens.rowH + Tokens.s2, appList.contentHeight))

        ListView {
            id: appList
            anchors.fill: parent
            clip: true
            reuseItems: true
            spacing: Tokens.s1
            model: root.applications

            TouchpadScrollHandler {
                flickable: appList
            }

            delegate: Rectangle {
                id: appRow
                required property var modelData
                required property int index
                readonly property var placementInfo: {
                    void DesktopShortcuts.screens;
                    void root.refreshKey;
                    return root.placement(modelData.id);
                }
                readonly property bool inThisFolder: placementInfo.where === "folder"
                    && placementInfo.folderId === root.folderId
                readonly property bool canToggle: root.writable
                    && (!inThisFolder || DesktopShortcuts.canTakeOut(root.screenName, root.folderId))
                readonly property string statusText: inThisFolder
                    ? Translation.tr("In this folder")
                    : placementInfo.where === "desktop"
                        ? Translation.tr("On the desktop")
                        : placementInfo.where === "folder"
                            ? Translation.tr("In %1").arg(placementInfo.folderName || Translation.tr("Folder"))
                            : Translation.tr("Not on the desktop")

                width: ListView.view.width
                height: Tokens.rowH + Tokens.s2
                radius: Tokens.radius
                color: inThisFolder ? Tokens.bone
                    : rowTap.pressed ? Tokens.tint16
                    : rowHover.hovered ? Tokens.tint10 : "transparent"
                border.width: Tokens.border
                border.color: inThisFolder ? Tokens.bone
                    : rowHover.hovered ? Tokens.lineStrong : Tokens.line
                opacity: canToggle ? 1 : 0.55
                activeFocusOnTab: canToggle

                Behavior on color {
                    ColorAnimation { duration: Tokens.snap }
                }

                Image {
                    id: appIcon
                    anchors.left: parent.left
                    anchors.leftMargin: Tokens.s3
                    anchors.verticalCenter: parent.verticalCenter
                    width: Tokens.s6
                    height: Tokens.s6
                    sourceSize: Qt.size(width, height)
                    source: Quickshell.iconPath(appRow.modelData.icon, "image-missing")
                    fillMode: Image.PreserveAspectFit
                }

                Column {
                    anchors.left: appIcon.right
                    anchors.leftMargin: Tokens.s3
                    anchors.right: stateGlyph.left
                    anchors.rightMargin: Tokens.s2
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Tokens.border

                    Text {
                        width: parent.width
                        text: appRow.modelData.name || appRow.modelData.id
                        color: appRow.inThisFolder ? Tokens.inkOnBone : Tokens.ink
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fBody
                        font.weight: Font.Medium
                        elide: Text.ElideRight
                    }
                    Text {
                        width: parent.width
                        text: appRow.statusText
                        color: appRow.inThisFolder ? Tokens.inkOnBoneDim : Tokens.inkMuted
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                        elide: Text.ElideRight
                    }
                }

                MaterialSymbol {
                    id: stateGlyph
                    anchors.right: parent.right
                    anchors.rightMargin: Tokens.s3
                    anchors.verticalCenter: parent.verticalCenter
                    text: appRow.inThisFolder ? "check_circle" : "add_circle"
                    iconSize: Tokens.s5
                    color: appRow.inThisFolder ? Tokens.inkOnBone : Tokens.inkDim
                }

                HoverHandler {
                    id: rowHover
                    enabled: appRow.canToggle
                    cursorShape: Qt.PointingHandCursor
                }
                TapHandler {
                    id: rowTap
                    enabled: appRow.canToggle
                    onTapped: root.toggle(appRow.modelData.id)
                }
                Keys.onPressed: event => {
                    if ((event.key === Qt.Key_Return || event.key === Qt.Key_Enter
                        || event.key === Qt.Key_Space) && !event.isAutoRepeat) {
                        root.toggle(appRow.modelData.id);
                        event.accepted = true;
                    }
                }
            }
        }

        Empty {
            anchors.centerIn: parent
            visible: appList.count === 0
            caption: Translation.tr("No applications match this search.")
        }
    }

    Btn {
        Layout.alignment: Qt.AlignRight
        compact: true
        text: Translation.tr("Done")
        onAct: root.done()
    }
}
