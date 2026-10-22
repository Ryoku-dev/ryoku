pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Widgets
import Caelestia.Config
import caelestia.components
import caelestia.components.containers
import caelestia.components.controls
import caelestia.components.effects
import caelestia.components.images
import caelestia.services
import caelestia.utils

Item {
    id: root

    property string searchQuery: ""
    property var currentResults: []
    property bool isLoading: false
    property bool isDownloading: false
    property real downloadProgressValue: 0
    property string downloadingWallpaperId: ""
    property string downloadState: "idle"
    // Set to false when embedded in a full page (e.g. Nexus) where the
    // drawer-style frame would look like a stray gray border.
    property bool showFrame: true
    property string downloadMessage: ""
    property var selectedWallpaper: null
    property bool detailPanelOpen: false
    property int selectedIndex: -1
    readonly property bool selectedWallpaperDownloading: downloadState === "progress" && selectedWallpaper?.id === downloadingWallpaperId
    readonly property bool selectedWallpaperFeedbackVisible: downloadState !== "idle" && selectedWallpaper?.id === downloadingWallpaperId

    function selectWallpaper(index) {
        if (index >= 0 && index < root.currentResults.length) {
            root.selectedWallpaper = root.currentResults[index];
            root.selectedIndex = index;
            root.detailPanelOpen = true;
        }
    }

    function selectNext() {
        if (root.selectedIndex < root.currentResults.length - 1) {
            selectWallpaper(root.selectedIndex + 1);
        }
    }

    function selectPrev() {
        if (root.selectedIndex > 0) {
            selectWallpaper(root.selectedIndex - 1);
        }
    }

    anchors.fill: parent

    onDetailPanelOpenChanged: {
        if (!detailPanelOpen) {
            clearWallpaperTimer.restart();
        }
    }

    ClippingRectangle {
        id: mainClippingRect

        anchors.fill: parent
        anchors.margins: root.showFrame ? Tokens.padding.medium : 0
        anchors.leftMargin: 0
        anchors.rightMargin: root.showFrame ? Tokens.padding.medium : 0

        radius: mainBorder.innerRadius
        color: "transparent"

        Loader {
            id: mainLoader

            anchors.fill: parent
            anchors.margins: Tokens.padding.large + Tokens.padding.medium
            anchors.leftMargin: Tokens.padding.large
            anchors.rightMargin: Tokens.padding.large

            asynchronous: true
            sourceComponent: mainContentComponent
        }
    }

    InnerBorder {
        id: mainBorder

        visible: root.showFrame
        leftThickness: 0
        rightThickness: Tokens.padding.medium
    }

    Component {
        id: mainContentComponent

        StyledFlickable {
            id: mainFlickable

            flickableDirection: Flickable.VerticalFlick
            contentHeight: mainLayout.height

            StyledScrollBar.vertical: StyledScrollBar {
                flickable: mainFlickable
            }

            ColumnLayout {
                id: mainLayout

                anchors.left: parent.left
                anchors.right: parent.right
                anchors.top: parent.top

                spacing: Tokens.spacing.medium

                RowLayout {
                    spacing: Tokens.spacing.small
                    Layout.fillWidth: true

                    StyledRect {
                        Layout.fillWidth: true
                        radius: Tokens.rounding.large
                        color: Colours.layer(Colours.palette.m3surfaceContainerHigh, 2)
                        implicitHeight: heroCol.implicitHeight + Tokens.padding.medium * 2

                        ColumnLayout {
                            id: heroCol

                            anchors.fill: parent
                            anchors.margins: Tokens.padding.medium
                            spacing: Tokens.spacing.extraSmall

                            StyledText {
                                text: qsTr("Wallhaven")
                                font: Tokens.font.body.builders.large.weight(Font.Medium).build()
                            }

                            StyledText {
                                text: qsTr("Search, preview, and set wallpapers instantly")
                                font: Tokens.font.body.small
                                color: Colours.palette.m3onSurfaceVariant
                            }
                        }
                    }

                    CircularIndicator {
                        implicitSize: 20
                        visible: root.isLoading
                    }
                }

                RowLayout {
                    Layout.fillWidth: true
                    spacing: Tokens.spacing.small

                    SearchBar {
                        id: searchField

                        Layout.fillWidth: true
                        placeholderText: qsTr("Search wallpapers...")
                        bg.color: Colours.tPalette.m3surfaceContainerLowest
                        bg.border.color: Colours.palette.m3outlineVariant
                        searchIcon.fontStyle: Tokens.font.icon.medium
                        clearIcon.visible: false
                        onTextChanged: root.searchQuery = text

                        Keys.onReturnPressed: {
                            if (root.searchQuery.trim()) {
                                root.isLoading = true;
                                WallhavenSearcher.search(root.searchQuery);
                            }
                        }
                    }

                    IconButton {
                        icon: "north"
                        onClicked: {
                            if (root.searchQuery.trim()) {
                                root.isLoading = true;
                                WallhavenSearcher.searchRandom(root.searchQuery);
                            }
                        }
                    }

                    IconButton {
                        icon: "refresh"
                        onClicked: {
                            if (root.searchQuery.trim()) {
                                root.isLoading = true;
                                WallhavenSearcher.search(root.searchQuery);
                            }
                        }
                    }
                }

                RowLayout {
                    StyledText {
                        text: root.currentResults.length > 0 ? qsTr("Found %1 wallpapers (page %2 of %3)").arg(root.currentResults.length).arg(WallhavenSearcher.currentPage).arg(WallhavenSearcher.lastPage) : qsTr("No results")
                        font: Tokens.font.body.small
                        color: Colours.palette.m3onSurfaceVariant
                    }

                    Item {
                        Layout.fillWidth: true
                    }

                    TextButton {
                        text: qsTr("Load more")
                        visible: WallhavenSearcher.currentPage < WallhavenSearcher.lastPage && !root.isLoading && root.currentResults.length > 0
                        onClicked: {
                            root.isLoading = true;
                            WallhavenSearcher.searchNextPage();
                        }
                    }
                }

                GridView {
                    id: resultsGrid

                    Layout.fillWidth: true
                    implicitHeight: 400

                    cellWidth: 200
                    cellHeight: 152

                    model: root.currentResults
                    clip: true

                    populate: Transition {
                        SequentialAnimation {
                            PropertyAction {
                                property: "scale"
                                value: 0.8
                            }
                            PropertyAction {
                                property: "opacity"
                                value: 0
                            }
                            NumberAnimation {
                                properties: "scale,opacity"
                                from: 0.8
                                to: 1
                                duration: Tokens.anim.durations.expressiveDefaultEffects
                            }
                        }
                    }

                    delegate: Item {
                        required property var modelData
                        required property int index
                        readonly property real itemMargin: Tokens.spacing.small / 2

                        width: resultsGrid.cellWidth
                        height: resultsGrid.cellHeight

                        StyledClippingRect {
                            anchors.fill: parent
                            anchors.leftMargin: itemMargin
                            anchors.rightMargin: itemMargin
                            anchors.topMargin: itemMargin
                            anchors.bottomMargin: itemMargin

                            radius: Tokens.rounding.large
                            color: Colours.layer(Colours.palette.m3surfaceContainerHigh, 1)

                            StateLayer {
                                onClicked: root.selectWallpaper(index)
                                radius: parent.radius
                                anchors.fill: parent
                            }

                            CachingImage {
                                anchors.fill: parent
                                source: modelData.thumbs?.large || modelData.thumbs?.small || ""
                                asynchronous: true
                                fillMode: Image.PreserveAspectCrop
                            }

                            Rectangle {
                                anchors.left: parent.left
                                anchors.right: parent.right
                                anchors.bottom: parent.bottom
                                implicitHeight: thumbFooter.implicitHeight + Tokens.padding.small * 2
                                color: Qt.rgba(0, 0, 0, 0.45)

                                StyledText {
                                    id: thumbFooter

                                    anchors.left: parent.left
                                    anchors.right: parent.right
                                    anchors.verticalCenter: parent.verticalCenter
                                    anchors.leftMargin: Tokens.padding.small
                                    anchors.rightMargin: Tokens.padding.small

                                    text: modelData.resolution ? modelData.resolution : (modelData.dimension_x && modelData.dimension_y ? `${modelData.dimension_x}x${modelData.dimension_y}` : "")
                                    elide: Text.ElideRight
                                    font: Tokens.font.label.small
                                    color: "white"
                                }
                            }
                        }
                    }
                }
            }
        }
    }

    StyledRect {
        id: detailPanel

        color: Colours.tPalette.m3surfaceContainer
        radius: Tokens.rounding.large

        anchors.fill: parent
        anchors.margins: Tokens.padding.large
        opacity: root.detailPanelOpen ? 1 : 0
        scale: root.detailPanelOpen ? 1 : 0.95
        clip: true
        enabled: root.detailPanelOpen

        Behavior on opacity {
            Anim {}
        }

        Behavior on scale {
            Anim {}
        }

        ColumnLayout {
            anchors.fill: parent
            spacing: Tokens.spacing.medium

            RowLayout {
                Layout.fillWidth: true

                IconButton {
                    icon: "close"
                    onClicked: root.detailPanelOpen = false
                }

                Item {
                    Layout.fillWidth: true
                }

                TextButton {
                    text: root.downloadState === "progress" ? qsTr("Downloading...") : qsTr("Download & Set")
                    enabled: root.downloadState !== "progress"
                    onClicked: {
                        const wallpaper = root.selectedWallpaper;
                        if (wallpaper) {
                            root.isDownloading = true;
                            root.downloadingWallpaperId = wallpaper.id;
                            root.downloadProgressValue = 0;
                            root.downloadState = "progress";
                            root.downloadMessage = qsTr("Connecting...");
                            WallhavenSearcher.downloadWallpaper(wallpaper);
                        }
                    }
                }
            }

            ColumnLayout {
                Layout.fillWidth: true
                visible: root.selectedWallpaperFeedbackVisible
                spacing: Tokens.spacing.extraSmall

                StyledText {
                    Layout.fillWidth: true
                    text: {
                        if (root.downloadState === "success")
                            return qsTr("Wallpaper applied");
                        if (root.downloadState === "error")
                            return root.downloadMessage || qsTr("Download failed");
                        return qsTr("Downloading... %1%").arg(Math.round(root.downloadProgressValue * 100));
                    }
                    font: Tokens.font.body.small
                    color: {
                        if (root.downloadState === "success")
                            return Colours.palette.m3primary;
                        if (root.downloadState === "error")
                            return Colours.palette.m3error;
                        return Colours.palette.m3outline;
                    }
                }

                StyledProgressBar {
                    Layout.fillWidth: true
                    visible: root.downloadState !== "error"
                    value: root.downloadState === "success" ? 1 : root.downloadProgressValue
                    indeterminate: root.downloadState === "progress" && root.downloadProgressValue <= 0
                }
            }

            Item {
                Layout.fillWidth: true
                Layout.fillHeight: true

                RowLayout {
                    anchors.fill: parent

                    IconButton {
                        icon: "chevron_left"
                        enabled: root.selectedIndex > 0
                        onClicked: root.selectPrev()
                    }

                    CachingImage {
                        Layout.fillWidth: true
                        Layout.fillHeight: true
                        anchors.margins: Tokens.padding.large
                        source: root.selectedWallpaper?.path || ""
                        asynchronous: true
                        fillMode: Image.PreserveAspectFit
                    }

                    IconButton {
                        icon: "chevron_right"
                        enabled: root.selectedIndex < root.currentResults.length - 1
                        onClicked: root.selectNext()
                    }
                }
            }

            RowLayout {
                Layout.fillWidth: true

                StyledText {
                    text: root.selectedWallpaper?.resolution || ""
                    font: Tokens.font.body.small
                    color: Colours.palette.m3onSurfaceVariant
                }

                Item {
                    Layout.fillWidth: true
                }

                StyledText {
                    text: "%1 / %2".arg(root.selectedIndex + 1).arg(root.currentResults.length)
                    font: Tokens.font.body.small
                    color: Colours.palette.m3onSurfaceVariant
                }
            }
        }
    }

    Timer {
        id: clearWallpaperTimer

        interval: Tokens.anim.durations.expressiveDefaultEffects
        onTriggered: {
            if (!detailPanelOpen)
                root.selectedWallpaper = null;
        }
    }

    Connections {
        function onSearchComplete(results, meta) {
            if (WallhavenSearcher.currentPage > 1) {
                const merged = [...root.currentResults, ...results];
                root.currentResults = merged;
            } else {
                root.currentResults = results;
            }
            root.isLoading = false;
        }

        function onDownloadComplete(id, path) {
            root.isDownloading = false;
            root.downloadProgressValue = 1;
            root.downloadingWallpaperId = id;
            root.downloadState = "success";
            root.downloadMessage = qsTr("Wallpaper applied");
            Logger.log("Wallhaven: Wallpaper saved to", path);
            if (root.selectedWallpaper && root.selectedWallpaper.id === id) {
                Wallpapers.setWallpaper(path);
            }
            feedbackResetTimer.restart();
        }

        function onDownloadFailed(id, error) {
            root.isDownloading = false;
            root.downloadProgressValue = 0;
            root.downloadingWallpaperId = id;
            root.downloadState = "error";
            root.downloadMessage = error || qsTr("Download failed");
            root.isLoading = false;
            console.error("Wallhaven: Download failed", error);
            feedbackResetTimer.restart();
        }

        function onDownloadProgress(id, progress) {
            root.isDownloading = true;
            root.downloadingWallpaperId = id;
            root.downloadState = "progress";
            root.downloadMessage = "";
            root.downloadProgressValue = progress;
        }

        target: WallhavenSearcher
    }

    Timer {
        id: feedbackResetTimer

        interval: root.downloadState === "error" ? 3500 : 1800
        onTriggered: {
            root.downloadState = "idle";
            root.downloadMessage = "";
            root.downloadProgressValue = 0;
            root.downloadingWallpaperId = "";
        }
    }
}
