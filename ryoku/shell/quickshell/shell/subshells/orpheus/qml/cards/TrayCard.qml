pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import shell.services
import Ryoku.Ui.Singletons
import "../kit" as K
import "../settings" as S

K.Sheet {
    id: root

    required property var scene
    property bool shown: false
    property var args: ({})
    property string menuService: ""
    property var fallbackItem: null
    property var pathIds: []
    property real pageShift: 0
    property real pageOpacity: 1

    readonly property bool overflowMode: args && args.overflow === true
    readonly property var requestedIdentity: args && args.item ? args.item : null
    readonly property var requestedItem: requestedIdentity
        && typeof requestedIdentity === "object" ? requestedIdentity : null
    readonly property string requestedService: requestedItem && requestedItem.service
        ? String(requestedItem.service) : String(requestedIdentity || "")
    readonly property bool showingOverflow: overflowMode && menuService.length === 0
    readonly property var currentItem: itemForService(menuService)
        || fallbackItem || requestedItem
    readonly property var menuTree: currentItem ? currentItem.menu : null
    readonly property var currentNode: nodeAtPath(menuTree, pathIds)
    readonly property var menuRows: visibleChildren(currentNode)
    readonly property bool menuHasToggles: menuRows.some(row => row && row.toggleState !== undefined)
    readonly property bool menuHasIcons: menuRows.some(row => row && !!row.iconName)
    readonly property int trayLimit: Math.max(3, Math.min(12,
        Number(S.Prefs.trayLimit) || 6))
    readonly property var overflowItems: (Tray.items || []).slice(trayLimit)

    pageWidth: 320
    maxHeight: scene ? scene.cardMaxHeight : 0
    title: showingOverflow ? "Tray" : displayName(currentItem)
    backVisible: pathIds.length > 0 || (overflowMode && menuService.length > 0)
    onBack: backPage()

    function itemForService(serviceName) {
        const items = Tray.items || [];
        for (let index = 0; index < items.length; ++index) {
            if (items[index] && String(items[index].service || "") === serviceName)
                return items[index];
        }
        return null;
    }

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

    function description(item) {
        if (!item)
            return "";
        const text = String(item.tooltip && item.tooltip.description || "").trim();
        return text || (item.status === "NeedsAttention" ? "Needs attention" : "");
    }

    function iconSource(item) {
        if (!item)
            return "";
        if (item.iconPath)
            return "file://" + item.iconPath;
        return item.iconName ? Icons.path(item.iconName, true) : "";
    }

    function nodeIconSource(node) {
        return node && node.iconName ? Icons.path(node.iconName, true) : "";
    }

    function cleanLabel(label) {
        return String(label || "").replace(/_([^_])/g, "$1").replace(/__/g, "_");
    }

    function visibleChildren(node) {
        const source = node && Array.isArray(node.children) ? node.children : [];
        const result = [];
        for (let index = 0; index < source.length; ++index) {
            if (source[index] && source[index].visible !== false)
                result.push(source[index]);
        }
        return result;
    }

    function childWithId(node, nodeId) {
        const children = node && Array.isArray(node.children) ? node.children : [];
        for (let index = 0; index < children.length; ++index) {
            if (children[index] && children[index].id === nodeId)
                return children[index];
        }
        return null;
    }

    function nodeAtPath(tree, ids) {
        let node = tree;
        for (let index = 0; node && index < ids.length; ++index)
            node = childWithId(node, ids[index]);
        return node || tree;
    }

    function validatePath() {
        let node = menuTree;
        let valid = 0;
        for (; node && valid < pathIds.length; ++valid) {
            const next = childWithId(node, pathIds[valid]);
            if (!next)
                break;
            node = next;
        }
        if (valid !== pathIds.length)
            pathIds = pathIds.slice(0, valid);
    }

    function animatePage(direction) {
        pageSlide.stop();
        pageFade.stop();
        if (K.Style.still) {
            pageShift = 0;
            pageOpacity = 1;
            return;
        }
        pageShift = direction * 28;
        pageOpacity = 0;
        pageSlide.restart();
        pageFade.restart();
    }

    function openMenu(item) {
        if (!item || !item.service)
            return;
        fallbackItem = item;
        menuService = String(item.service);
        pathIds = [];
        Tray.aboutToShow(menuService);
        animatePage(1);
    }

    function drill(node) {
        if (!node || !Array.isArray(node.children) || node.children.length === 0)
            return;
        const next = pathIds.slice();
        next.push(node.id);
        pathIds = next;
        animatePage(1);
    }

    function backPage() {
        if (pathIds.length > 0) {
            pathIds = pathIds.slice(0, pathIds.length - 1);
        } else if (overflowMode && menuService.length > 0) {
            menuService = "";
            fallbackItem = null;
        } else {
            return;
        }
        animatePage(-1);
    }

    function activateMenu(node) {
        if (!node || node.enabled === false)
            return;
        Tray.menuEvent(menuService, node.id);
        scene.closeCard();
    }

    function activateTrayItem(item) {
        if (!item || !item.service)
            return;
        if (item.itemIsMenu) {
            openMenu(item);
            return;
        }
        Tray.activate(item.service, 0, 0);
        scene.closeCard();
    }

    function syncArgs() {
        pathIds = [];
        fallbackItem = requestedItem;
        menuService = overflowMode ? "" : requestedService;
        if (shown && menuService)
            Tray.aboutToShow(menuService);
    }

    onArgsChanged: syncArgs()
    onShownChanged: {
        if (shown)
            syncArgs();
    }
    onMenuTreeChanged: validatePath()
    onCurrentItemChanged: {
        if (!shown || !menuService || currentItem)
            return;
        if (overflowMode) {
            menuService = "";
            fallbackItem = null;
            pathIds = [];
        } else {
            scene.closeCard();
        }
    }

    NumberAnimation {
        id: pageSlide
        target: root
        property: "pageShift"
        to: 0
        duration: K.Style.effects
        easing.type: Easing.OutCubic
    }

    NumberAnimation {
        id: pageFade
        target: root
        property: "pageOpacity"
        to: 1
        duration: K.Style.effects
        easing.type: Easing.OutCubic
    }

    Item {
        width: parent.width
        implicitHeight: pageColumn.implicitHeight
        height: implicitHeight
        opacity: root.pageOpacity
        transform: Translate { x: root.pageShift }

        Column {
            id: pageColumn

            width: parent.width
            spacing: K.Style.s1

            Repeater {
                model: root.showingOverflow ? root.overflowItems : []

                delegate: Item {
                    id: overflowRow

                    required property int index
                    required property var modelData
                    readonly property string source: root.iconSource(modelData)
                    readonly property bool hasMenu: modelData && modelData.menu !== null

                    width: pageColumn.width
                    height: K.Style.rowH
                    scale: overflowPress.pressScale

                    Rectangle {
                        visible: overflowRow.index > 0
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.top: parent.top
                        height: K.Style.border
                        color: K.Style.lineSoft
                    }

                    Image {
                        id: overflowIcon
                        anchors.left: parent.left
                        anchors.leftMargin: K.Style.s2
                        anchors.verticalCenter: parent.verticalCenter
                        width: 18
                        height: 18
                        source: overflowRow.source
                        sourceSize: Qt.size(36, 36)
                        fillMode: Image.PreserveAspectFit
                        asynchronous: true
                        smooth: true
                        mipmap: true
                        visible: source.toString().length > 0 && status !== Image.Error
                    }

                    K.Glyph {
                        anchors.centerIn: overflowIcon
                        visible: !overflowIcon.visible
                        name: "apps"
                        size: 18
                        tone: "variant"
                    }

                    Column {
                        anchors.left: overflowIcon.right
                        anchors.leftMargin: K.Style.s3
                        anchors.right: overflowChevron.left
                        anchors.rightMargin: K.Style.s2
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: K.Style.s1

                        K.Label {
                            width: parent.width
                            text: root.displayName(overflowRow.modelData)
                            role: "bodyStrong"
                        }

                        K.Label {
                            width: parent.width
                            visible: text.length > 0
                            text: root.description(overflowRow.modelData)
                            role: "caption"
                            tone: "variant"
                        }
                    }

                    K.Glyph {
                        id: overflowChevron
                        anchors.right: parent.right
                        anchors.rightMargin: K.Style.s2
                        anchors.verticalCenter: parent.verticalCenter
                        visible: overflowRow.hasMenu
                        name: "chevron_right"
                        size: 18
                        tone: "variant"
                    }

                    K.Press {
                        id: overflowPress
                        radius: K.Style.radius
                        onClicked: mouse => root.activateTrayItem(overflowRow.modelData)
                        onRightClicked: root.openMenu(overflowRow.modelData)
                    }
                }
            }

            Repeater {
                model: root.showingOverflow ? [] : root.menuRows

                delegate: Item {
                    id: menuEntry

                    required property var modelData
                    required property int index
                    readonly property bool separator: modelData.type === "separator"
                    readonly property bool rowEnabled: modelData.enabled !== false
                    readonly property bool hasChildren: !separator
                        && Array.isArray(modelData.children) && modelData.children.length > 0
                    readonly property bool toggled: modelData.toggleState !== undefined
                        && Number(modelData.toggleState) !== 0
                    readonly property string source: root.nodeIconSource(modelData)

                    width: pageColumn.width
                    height: separator ? K.Style.s2 : K.Style.rowH
                    opacity: rowEnabled ? 1 : 0.38
                    scale: separator ? 1 : menuPress.pressScale

                    Rectangle {
                        visible: menuEntry.separator
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        height: K.Style.border
                        color: K.Style.lineSoft
                    }

                    Rectangle {
                        visible: !menuEntry.separator && menuEntry.index > 0
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.top: parent.top
                        height: K.Style.border
                        color: K.Style.lineSoft
                    }

                    Item {
                        id: markSlot
                        visible: !menuEntry.separator && root.menuHasToggles
                        anchors.left: parent.left
                        anchors.leftMargin: K.Style.s2
                        anchors.verticalCenter: parent.verticalCenter
                        width: root.menuHasToggles ? 18 : 0
                        height: 18

                        Rectangle {
                            visible: menuEntry.toggled
                            anchors.centerIn: parent
                            width: 16
                            height: 16
                            radius: 2
                            color: K.Style.bone

                            K.Glyph {
                                anchors.centerIn: parent
                                name: "check"
                                size: 12
                                tone: "onBone"
                            }
                        }
                    }

                    Image {
                        id: menuIcon
                        visible: !menuEntry.separator && source.toString().length > 0
                            && status !== Image.Error
                        anchors.left: markSlot.right
                        anchors.leftMargin: root.menuHasToggles ? K.Style.s2 : 0
                        anchors.verticalCenter: parent.verticalCenter
                        width: root.menuHasIcons ? 18 : 0
                        height: 18
                        source: menuEntry.source
                        sourceSize: Qt.size(36, 36)
                        fillMode: Image.PreserveAspectFit
                        asynchronous: true
                        smooth: true
                        mipmap: true
                    }

                    K.Label {
                        visible: !menuEntry.separator
                        anchors.left: menuIcon.right
                        anchors.leftMargin: root.menuHasIcons ? K.Style.s2 : 0
                        anchors.right: submenuGlyph.left
                        anchors.rightMargin: K.Style.s2
                        anchors.verticalCenter: parent.verticalCenter
                        text: root.cleanLabel(menuEntry.modelData.label)
                        role: "body"
                    }

                    K.Glyph {
                        id: submenuGlyph
                        visible: menuEntry.hasChildren
                        anchors.right: parent.right
                        anchors.rightMargin: K.Style.s2
                        anchors.verticalCenter: parent.verticalCenter
                        name: "chevron_right"
                        size: 18
                        tone: "variant"
                    }

                    K.Press {
                        id: menuPress
                        enabled: !menuEntry.separator && menuEntry.rowEnabled
                        radius: K.Style.radius
                        onClicked: mouse => {
                            if (menuEntry.hasChildren)
                                root.drill(menuEntry.modelData);
                            else
                                root.activateMenu(menuEntry.modelData);
                        }
                    }
                }
            }

            Item {
                width: parent.width
                height: 120
                visible: root.showingOverflow ? root.overflowItems.length === 0
                    : root.menuRows.length === 0

                K.Label {
                    anchors.centerIn: parent
                    text: root.showingOverflow ? "No hidden tray items" : "No commands"
                    role: "body"
                    tone: "variant"
                }
            }
        }
    }
}
