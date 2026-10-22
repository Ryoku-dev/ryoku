pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtCore
import Caelestia
import Caelestia.Config
import caelestia.components
import caelestia.components.controls
import caelestia.services
import caelestia.utils
import caelestia.modules.launcher.services

Item {
    id: root

    required property var content
    required property DrawerVisibilities visibilities
    required property var panels
    required property real maxWidth
    required property real maxHeight
    required property StyledTextField search
    required property int padding
    required property int rounding

    property string currentWallpaperTab: "Main"

    readonly property bool showWallpapers: search.text.startsWith(`${GlobalConfig.launcher.actionPrefix}wallpaper `)
    readonly property bool showWindowSwitcher: search.text.startsWith(`${GlobalConfig.launcher.actionPrefix}windows `)
    readonly property bool showKeybinds: search.text.startsWith(`${GlobalConfig.launcher.actionPrefix}keybinds `)
    readonly property bool showAnimations: search.text.startsWith(`${GlobalConfig.launcher.actionPrefix}animations `)
    readonly property bool showAppsBrowser: root.state === "apps" && !search.text && Config.launcher.showBrowseOnEmpty
    readonly property var currentList: showWallpapers ? wallpaperList.item : (showWindowSwitcher ? windowSwitcherList.item : (showAnimations ? animationsList.item : (showKeybinds ? keybindsList.item : (showAppsBrowser ? browser.item : appList.item))))

    readonly property var wallpaperTabs: {
        const res = [];
        for (let dir of Wallpapers.categories) {
            res.push({ id: dir, text: dir });
        }
        return res;
    }

    anchors.horizontalCenter: parent.horizontalCenter
    anchors.bottom: parent.bottom

    width: implicitWidth
    height: implicitHeight

    clip: true
    state: showAnimations ? "animations" : (showWindowSwitcher ? "windowSwitcher" : (showKeybinds ? "keybinds" : (showWallpapers ? "wallpapers" : "apps")))

    onShowWindowSwitcherChanged: {
        if (!showWindowSwitcher) {
            Windows.isSwitching = false;
            Kwin.clearHighlight();
        } else if (!Windows.isSwitching) {
            Windows.selectedIndex = 0;
            Windows.updateItems();
        }
    }

    states: [
        State {
            name: "apps"

            PropertyChanges {
                target: root
                implicitWidth: root.showAppsBrowser ? browser.implicitWidth : root.Tokens.sizes.launcher.itemWidth
                implicitHeight: root.showAppsBrowser ? (Config.launcher.browseLayout === LauncherBrowseLayout.Compact ? Math.min(root.maxHeight, browser.implicitHeight) : Math.min(root.maxHeight, Math.max(root.Tokens.sizes.launcher.browseMinHeight, Math.min(browser.implicitHeight, root.Tokens.sizes.launcher.browseHeight)))) : Math.min(root.maxHeight, appList.implicitHeight > 0 ? appList.implicitHeight : empty.implicitHeight)
            }
        },
        State {
            name: "wallpapers"

            PropertyChanges {
                target: root
                implicitWidth: Math.max(root.Tokens.sizes.launcher.itemWidth * 1.2, wallpaperList.implicitWidth)
                implicitHeight: filtersRow.implicitHeight + Tokens.spacing.medium + root.Tokens.sizes.launcher.wallpaperHeight + wallpaperTabsWrapper.implicitHeight + 24
            }
        },
        State {
            name: "windowSwitcher"

            PropertyChanges {
                target: root
                implicitWidth: Math.max(root.Tokens.sizes.launcher.itemWidth * 1.2, windowSwitcherList.implicitWidth)
                implicitHeight: root.Tokens.sizes.launcher.windowSwitcherHeight
            }
        },
        State {
            name: "keybinds"

            PropertyChanges {
                target: root
                implicitWidth: root.Tokens.sizes.launcher.itemWidth
                implicitHeight: Math.min(root.maxHeight, root.Tokens.sizes.launcher.itemHeight * 7)
            }
        },
        State {
            name: "animations"

            PropertyChanges {
                target: root
                implicitWidth: root.Tokens.sizes.launcher.itemWidth
                implicitHeight: Math.min(root.maxHeight, root.Tokens.sizes.launcher.itemHeight * 7)
            }
        }
    ]

    Component.onCompleted: {
        Wallpapers.currentMediaFilter = wallpaperSettings.mediaFilter;
    }

    onActiveFocusChanged: {
        wallpaperSettings.mediaFilter = Wallpapers.currentMediaFilter;
    }

    onShowWallpapersChanged: {
        if (showWallpapers) {
            for (let category of Wallpapers.categories) {
                let walls = Wallpapers.grouped[category] || [];
                if (walls.some(w => w.path === Wallpapers.actualCurrent)) {
                    currentWallpaperTab = category;
                    break;
                }
            }
        }
    }

    Behavior on state {
        enabled: root.visibilities.launcher && !root.visibilities.skipLauncherAnim && root.opacity === 1 && !Visibilities.launcherInitialSearch

        SequentialAnimation {
            Anim {
                target: root
                property: "opacity"
                from: 1
                to: 0
                type: Anim.DefaultEffects
            }
            PropertyAction {}
            Anim {
                target: root
                property: "opacity"
                from: 0
                to: 1
                type: Anim.DefaultEffects
            }
        }
    }

    Settings {
        id: wallpaperSettings

        property string mediaFilter: "All"

        category: "Wallpapers"
    }

    Timer {
        id: keybindsTimer

        interval: 50
        onTriggered: {
            if (state === "keybinds" && keybindsList.item) {
                keybindsList.item.refreshModel();
            }
        }
    }

    Loader {
        id: appList

        active: root.state === "apps" && !root.showAppsBrowser

        anchors.fill: parent

        sourceComponent: AppList {
            search: root.search
            visibilities: root.visibilities
        }
    }

    Loader {
        id: browser

        active: root.state === "apps"
        visible: root.showAppsBrowser
        opacity: root.showAppsBrowser ? 1 : 0

        anchors.fill: parent

        sourceComponent: AppBrowser {
            visibilities: root.visibilities
            maxWidth: root.maxWidth
        }

        Behavior on opacity {
            enabled: root.visibilities.launcher && !root.visibilities.skipLauncherAnim

            Anim {
                type: Anim.DefaultEffects
            }
        }
    }

    Row {
        id: filtersRow

        anchors.top: parent.top
        anchors.horizontalCenter: parent.horizontalCenter
        spacing: Tokens.spacing.small

        visible: root.state === "wallpapers"

        IconTextButton {
            text: qsTr("Images")
            icon: "image"
            type: Wallpapers.currentMediaFilter === "Image" ? TextButton.Filled : TextButton.Tonal
            onClicked: {
                Wallpapers.currentMediaFilter = Wallpapers.currentMediaFilter === "Image" ? "All" : "Image";
                wallpaperSettings.mediaFilter = Wallpapers.currentMediaFilter;
            }
        }
        IconTextButton {
            text: qsTr("Animated")
            icon: "animation"
            type: Wallpapers.currentMediaFilter === "Animated" ? TextButton.Filled : TextButton.Tonal
            onClicked: {
                Wallpapers.currentMediaFilter = Wallpapers.currentMediaFilter === "Animated" ? "All" : "Animated";
                wallpaperSettings.mediaFilter = Wallpapers.currentMediaFilter;
            }
        }
        IconTextButton {
            text: qsTr("Videos")
            icon: "videocam"
            type: Wallpapers.currentMediaFilter === "Video" ? TextButton.Filled : TextButton.Tonal
            onClicked: {
                Wallpapers.currentMediaFilter = Wallpapers.currentMediaFilter === "Video" ? "All" : "Video";
                wallpaperSettings.mediaFilter = Wallpapers.currentMediaFilter;
            }
        }
    }

    Loader {
        id: wallpaperList

        asynchronous: true
        active: root.state === "wallpapers"

        anchors.top: filtersRow.bottom
        anchors.topMargin: Tokens.spacing.medium
        anchors.horizontalCenter: parent.horizontalCenter
        height: root.Tokens.sizes.launcher.wallpaperHeight

        sourceComponent: WallpaperList {
            search: root.search
            visibilities: root.visibilities
            panels: root.panels
            content: root.content
            contentList: root
        }
    }

    Item {
        id: wallpaperTabsWrapper

        anchors.horizontalCenter: parent.horizontalCenter
        anchors.bottom: parent.bottom
        anchors.bottomMargin: Tokens.padding.medium
        implicitWidth: Math.min(parent.width - Tokens.padding.large * 2, tabsRow.implicitWidth)
        implicitHeight: tabsRow.implicitHeight + indicator.implicitHeight + 5

        visible: root.state === "wallpapers"

        Flickable {
            id: tabsFlickable

                anchors.fill: parent
                contentWidth: tabsRow.implicitWidth
                contentHeight: parent.height
                flickableDirection: Flickable.HorizontalFlick
                clip: true

                ScrollBar.horizontal: StyledScrollBar {
                    flickable: tabsFlickable
                    active: tabsFlickable.moving || tabsFlickable.dragging
                }

            Row {
                id: tabsRow

                spacing: Tokens.spacing.large

                Repeater {
                    id: tabsRepeater

                    model: root.wallpaperTabs

                    delegate: Item {
                        id: tab

                        required property var modelData
                        required property int index

                        readonly property bool current: root.currentWallpaperTab === tab.modelData.id

                        implicitWidth: label.implicitWidth + Tokens.padding.medium * 2
                        implicitHeight: label.implicitHeight + Tokens.padding.small * 2

                        CustomMouseArea {
                            function onWheel(event: WheelEvent): void {
                                let idx = root.wallpaperTabs.findIndex(t => t.id === root.currentWallpaperTab);
                                if (event.angleDelta.y < 0 || event.angleDelta.x < 0)
                                    idx = Math.min(idx + 1, root.wallpaperTabs.length - 1);
                                else if (event.angleDelta.y > 0 || event.angleDelta.x > 0)
                                    idx = Math.max(idx - 1, 0);

                                root.currentWallpaperTab = root.wallpaperTabs[idx].id;
                            }

                            anchors.fill: parent
                            hoverEnabled: true
                            cursorShape: Qt.PointingHandCursor

                            StateLayer {
                                anchors.fill: parent
                                radius: Tokens.rounding.medium
                                color: tab.current ? Colours.palette.m3primary : Colours.palette.m3onSurface
                                onClicked: root.currentWallpaperTab = tab.modelData.id
                            }
                        }

                        StyledText {
                            id: label

                            anchors.centerIn: parent
                            text: tab.modelData.text
                            color: tab.current ? Colours.palette.m3primary : Colours.palette.m3onSurfaceVariant
                            font: Tokens.font.label.large
                        }
                    }
                }
            }

            Item {
                id: indicator

                property int currentIndex: Math.max(0, root.wallpaperTabs.findIndex(t => t.id === root.currentWallpaperTab))
                property Item currentTab: tabsRepeater.itemAt(currentIndex)

                anchors.top: tabsRow.bottom
                anchors.topMargin: 5

                implicitWidth: currentTab ? currentTab.implicitWidth : 0
                implicitHeight: 3
                x: currentTab ? tabsRow.x + currentTab.x : 0
                clip: true

                onCurrentIndexChanged: {
                    if (currentTab) {
                        const targetX = currentTab.x;
                        const targetWidth = currentTab.implicitWidth;
                        if (targetX < tabsFlickable.contentX)
                            tabsFlickable.contentX = targetX;
                        else if (targetX + targetWidth > tabsFlickable.contentX + tabsFlickable.width)
                            tabsFlickable.contentX = targetX + targetWidth - tabsFlickable.width;
                    }
                }

                StyledRect {
                    anchors.top: parent.top
                    anchors.left: parent.left
                    anchors.right: parent.right
                    implicitHeight: parent.implicitHeight * 2
                    color: Colours.palette.m3primary
                    radius: Tokens.rounding.full
                }

                Behavior on x {
                    Anim {}
                }
                Behavior on implicitWidth {
                    Anim {}
                }
            }
        }
    }

    Loader {
        id: windowSwitcherList

        asynchronous: true
        active: root.state === "windowSwitcher"

        anchors.top: parent.top
        anchors.bottom: parent.bottom
        anchors.horizontalCenter: parent.horizontalCenter

        sourceComponent: WindowSwitcherList {
            search: root.search
            visibilities: root.visibilities
            panels: root.panels
            content: root.content
        }
    }

    Loader {
        id: keybindsList

        asynchronous: true
        active: root.state === "keybinds"

        anchors.fill: parent

        sourceComponent: KeybindsList {
            search: root.search
            visibilities: root.visibilities
        }
    }

    Loader {
        id: animationsList

        asynchronous: true
        active: root.state === "animations"

        anchors.fill: parent

        sourceComponent: AnimationsList {
            search: root.search
            visibilities: root.visibilities
        }
    }

    Row {
        id: empty

        readonly property bool cliphistMissing: root.currentList?.state === "clipboard" && !Clipboard.available

        opacity: (!root.showAppsBrowser && root.currentList?.count === 0) ? 1 : 0
        scale: (!root.showAppsBrowser && root.currentList?.count === 0) ? 1 : 0.5

        spacing: Tokens.spacing.medium
        padding: Tokens.padding.large

        anchors.horizontalCenter: parent.horizontalCenter
        anchors.verticalCenter: parent.verticalCenter

        MaterialIcon {
            text: {
                if (empty.cliphistMissing)
                    return "content_paste_off";
                if (root.state === "wallpapers")
                    return "wallpaper_slideshow";
                if (root.state === "keybinds")
                    return "keyboard";
                if (root.state === "animations")
                    return "animation";
                return "manage_search";
            }
            color: Colours.palette.m3onSurfaceVariant
            fontStyle: Tokens.font.icon.extraLarge

            anchors.verticalCenter: parent.verticalCenter
        }

        Column {
            anchors.verticalCenter: parent.verticalCenter

            StyledText {
                text: {
                    if (empty.cliphistMissing)
                        return qsTr("cliphist not found");
                    if (root.state === "wallpapers")
                        return qsTr("No wallpapers found");
                    if (root.state === "keybinds")
                        return qsTr("No keybinds found");
                    if (root.state === "animations")
                        return qsTr("No animations found");
                    return qsTr("No results");
                }
                color: Colours.palette.m3onSurfaceVariant
                font: Tokens.font.body.builders.large.weight(Font.Medium).build()
            }

            StyledText {
                text: {
                    if (empty.cliphistMissing)
                        return qsTr("Install cliphist to enable clipboard history");
                    if (root.state === "wallpapers")
                        return Wallpapers.list.length === 0 ? qsTr("Try putting some wallpapers in %1").arg(Paths.shortenHome(Paths.wallsdir)) : qsTr("Try searching for something else");
                    if (root.state === "keybinds")
                        return qsTr("No keybinds match your search");
                    if (root.state === "animations")
                        return qsTr("Try adding .lua files to\n~/.config/caelestia/animations/");
                    return qsTr("Try searching for something else");
                }
                color: Colours.palette.m3onSurfaceVariant
                font: Tokens.font.body.medium
            }
        }

        Behavior on opacity {
            Anim {
                type: Anim.DefaultEffects
            }
        }

        Behavior on scale {
            Anim {}
        }
    }

    Behavior on implicitWidth {
        enabled: root.visibilities.launcher && !root.visibilities.skipLauncherAnim

        Anim {}
    }

    Behavior on implicitHeight {
        enabled: root.visibilities.launcher && !root.visibilities.skipLauncherAnim

        Anim {}
    }
}
