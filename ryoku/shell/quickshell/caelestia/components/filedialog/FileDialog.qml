pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Quickshell
import caelestia.components
import caelestia.services

LazyLoader {
    id: loader

    property list<string> cwd: ["Home"]
    property string filterLabel: qsTr("All files")
    property list<string> filters: ["*"]
    property bool selectFolder: false
    property string title: selectFolder ? qsTr("Select a folder") : qsTr("Select a file")
    property bool countedOpen: false

    signal accepted(path: string)
    signal rejected

    function open(): void {
        activeAsync = true;
    }

    function close(): void {
        rejected();
    }

    onAccepted: activeAsync = false
    onRejected: activeAsync = false

    onActiveAsyncChanged: {
        if (activeAsync !== countedOpen) {
            Visibilities.openDialogs += activeAsync ? 1 : -1;
            countedOpen = activeAsync;
        }
    }
    Component.onDestruction: {
        if (countedOpen)
            Visibilities.openDialogs -= 1;
    }

    FloatingWindow {
        id: root

        property bool selectFolder: loader.selectFolder
        property list<string> cwd: loader.cwd
        property string filterLabel: loader.filterLabel
        property list<string> filters: loader.filters

        readonly property bool selectionValid: {
            if (loader.selectFolder) return true;
            const file = folderContents.currentItem?.modelData;
            return (file && !file.isDir && (filters.includes("*") || filters.some(f => f.toLowerCase() === file.suffix.toLowerCase()))) ?? false;
        }

        function accepted(path: string): void {
            loader.accepted(path);
        }

        function rejected(): void {
            loader.rejected();
        }

        implicitWidth: 1000
        implicitHeight: 600
        minimumSize.width: 400
        minimumSize.height: 300
        color: Colours.tPalette.m3surface
        surfaceFormat.opaque: false
        title: loader.title

        onVisibleChanged: {
            if (!visible)
                rejected();
        }

        RowLayout {
            anchors.fill: parent

            spacing: 0

            Sidebar {
                Layout.fillHeight: true
                dialog: root
            }

            ColumnLayout {
                Layout.fillWidth: true
                Layout.fillHeight: true

                spacing: 0

                HeaderBar {
                    Layout.fillWidth: true
                    dialog: root
                }

                FolderContents {
                    id: folderContents

                    Layout.fillWidth: true
                    Layout.fillHeight: true
                    dialog: root
                }

                DialogButtons {
                    Layout.fillWidth: true
                    dialog: root
                    folder: folderContents
                }
            }
        }

        Behavior on color {
            CAnim {}
        }
    }
}
