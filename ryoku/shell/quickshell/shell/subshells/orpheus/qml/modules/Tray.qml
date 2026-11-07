pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import shell.services
import "../kit" as K
import "../settings" as S

Item {
    id: root

    required property var scene
    required property string lane
    property bool present: Tray.items.length > 0

    readonly property int limit: S.Prefs.trayLimit
    readonly property var shownItems: Tray.items.slice(0, limit)
    readonly property bool hasOverflow: Tray.items.length > limit

    implicitWidth: present ? iconRow.implicitWidth + K.Style.hitPad * 2 : 0
    implicitHeight: K.Style.barH

    function displayName(item) {
        if (!item)
            return "Tray app";
        let name = String(item.title || "").trim();
        if (!name)
            name = String(item.tooltip && item.tooltip.title || "").trim();
        if (!name)
            name = String(item.id || "").trim();
        const slash = name.lastIndexOf("/");
        if (slash >= 0 && slash < name.length - 1)
            name = name.substring(slash + 1);
        name = name.replace(/^org\.(kde|ayatana|freedesktop)\./i, "")
            .replace(/[_-]+/g, " ");
        return name || "Tray app";
    }

    function pointFor(item) {
        return item.mapToGlobal(item.width / 2, item.height);
    }

    function activate(item, anchor) {
        if (!item || !item.service)
            return;
        if (item.itemIsMenu) {
            scene.toggleCard("tray", anchor, { item: item.service });
            return;
        }
        const point = pointFor(anchor);
        Tray.activate(item.service, Math.round(point.x), Math.round(point.y));
    }

    function secondaryActivate(item, anchor) {
        if (!item || !item.service)
            return;
        const point = pointFor(anchor);
        Tray.send("tray.secondaryActivate", {
            service: item.service,
            x: Math.round(point.x),
            y: Math.round(point.y)
        });
    }

    Row {
        id: iconRow

        anchors.centerIn: parent
        spacing: 8 * K.Style.scale

        Repeater {
            model: root.shownItems

            delegate: Item {
                id: trayItem

                required property var modelData

                width: K.Style.barIcon
                height: K.Style.barH
                scale: itemPress.pressScale

                Image {
                    anchors.centerIn: parent
                    width: K.Style.barIcon
                    height: K.Style.barIcon
                    source: trayItem.modelData.iconPath
                        ? "file://" + trayItem.modelData.iconPath
                        : Quickshell.iconPath(trayItem.modelData.iconName || "", true)
                    sourceSize: Qt.size(
                        Math.max(1, Math.round(width * Screen.devicePixelRatio * 2)),
                        Math.max(1, Math.round(height * Screen.devicePixelRatio * 2)))
                    fillMode: Image.PreserveAspectFit
                    asynchronous: true
                    smooth: true
                    mipmap: true
                }

                K.Press {
                    id: itemPress
                    radius: K.Style.radius
                    onClicked: mouse => root.activate(trayItem.modelData, trayItem)
                    onRightClicked: root.scene.toggleCard("tray", trayItem,
                        { item: trayItem.modelData.service })
                    onMiddleClicked: root.secondaryActivate(trayItem.modelData, trayItem)
                }

                K.Hint {
                    target: trayItem
                    text: root.displayName(trayItem.modelData)
                    shown: itemPress.hovered
                }
            }
        }

        Item {
            id: overflowButton

            visible: root.hasOverflow
            width: root.hasOverflow ? K.Style.barIcon : 0
            height: K.Style.barH
            scale: overflowPress.pressScale

            K.Glyph {
                anchors.centerIn: parent
                name: "expand_more"
                size: K.Style.barIcon
                tone: "dim"
            }

            K.Press {
                id: overflowPress
                radius: K.Style.radius
                onClicked: mouse => root.scene.toggleCard("tray", overflowButton,
                    { overflow: true })
            }

            K.Hint {
                target: overflowButton
                text: "More tray items"
                shown: overflowPress.hovered
            }
        }
    }
}
