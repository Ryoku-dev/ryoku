pragma ComponentBehavior: Bound

import QtQuick
import Caelestia.Config
import caelestia.components

Item {
    id: root

    required property DrawerVisibilities visibilities
    required property real maxWidth

    readonly property var browser: loader.item
    readonly property var currentItem: root.browser?.currentItem ?? null
    readonly property int count: root.browser?.count ?? 0

    function refresh(): void {
        root.browser?.refresh();
    }

    function launch(app): void {
        root.browser?.launch(app);
    }

    function selectTile(index: int): void {
        root.browser?.selectTile(index);
    }

    function incrementCurrentIndex(): void {
        root.browser?.incrementCurrentIndex();
    }

    function decrementCurrentIndex(): void {
        root.browser?.decrementCurrentIndex();
    }

    function moveLeft(): void {
        root.browser?.moveLeft();
    }

    function moveRight(): void {
        root.browser?.moveRight();
    }

    function toggleFocus(): void {
        root.browser?.toggleFocus();
    }

    function activateCurrent(): void {
        root.browser?.activateCurrent();
    }

    implicitWidth: root.browser?.implicitWidth ?? 0
    implicitHeight: root.browser?.implicitHeight ?? 0

    Loader {
        id: loader

        anchors.fill: parent
        sourceComponent: {
            switch (Config.launcher.browseLayout) {
            case LauncherBrowseLayout.Simple:
                return simpleBrowser;
            case LauncherBrowseLayout.Compact:
                return compactBrowser;
            default:
                return gridBrowser;
            }
        }
    }

    Component {
        id: gridBrowser

        AppBrowserGrid {
            visibilities: root.visibilities
            maxWidth: root.maxWidth
        }
    }

    Component {
        id: simpleBrowser

        AppBrowserSimple {
            visibilities: root.visibilities
            maxWidth: root.maxWidth
        }
    }

    Component {
        id: compactBrowser

        AppBrowserCompact {
            visibilities: root.visibilities
            maxWidth: root.maxWidth
        }
    }
}
