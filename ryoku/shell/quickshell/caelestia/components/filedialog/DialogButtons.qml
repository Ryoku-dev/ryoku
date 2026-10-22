import QtQuick.Layouts
import Caelestia.Config
import caelestia.components
import caelestia.services
import caelestia.utils

StyledRect {
    id: root

    required property var dialog
    required property FolderContents folder

    implicitHeight: inner.implicitHeight + Tokens.padding.medium * 2

    color: Colours.tPalette.m3surfaceContainer

    RowLayout {
        id: inner

        anchors.fill: parent
        anchors.margins: Tokens.padding.medium

        spacing: Tokens.spacing.small

        StyledText {
            text: qsTr("Filter:")
        }

        StyledRect {
            Layout.fillWidth: true
            Layout.fillHeight: true
            Layout.rightMargin: Tokens.spacing.medium

            color: Colours.tPalette.m3surfaceContainerHigh
            radius: Tokens.rounding.medium

            StyledText {
                anchors.fill: parent
                anchors.margins: Tokens.padding.medium

                text: root.dialog.selectFolder ? qsTr("Folders") : `${root.dialog.filterLabel} (${root.dialog.filters.map(f => `*.${f}`).join(", ")})`
            }
        }

        StyledRect {
            color: Colours.tPalette.m3surfaceContainerHigh
            radius: Tokens.rounding.medium

            implicitWidth: selectText.implicitWidth + Tokens.padding.medium * 2
            implicitHeight: selectText.implicitHeight + Tokens.padding.medium * 2

            StateLayer {
                disabled: !root.dialog.selectionValid
                onClicked: {
                    if (root.dialog.selectFolder) {
                        if (root.folder.currentItem?.modelData?.isDir) {
                            root.dialog.accepted(root.folder.currentItem.modelData.path);
                        } else {
                            const currentPath = root.dialog.cwd[0] === "Home"
                                ? (Paths.home + (root.dialog.cwd.length > 1 ? `/${root.dialog.cwd.slice(1).join("/")}` : ""))
                                : root.dialog.cwd.join("/");
                            root.dialog.accepted(currentPath);
                        }
                    } else if (root.folder.currentItem?.modelData) {
                        root.dialog.accepted(root.folder.currentItem.modelData.path);
                    }
                }
            }

            StyledText {
                id: selectText

                anchors.centerIn: parent
                anchors.margins: Tokens.padding.medium

                text: root.dialog.selectFolder ? qsTr("Select Folder") : qsTr("Select")
                color: root.dialog.selectionValid ? Colours.palette.m3onSurface : Colours.palette.m3outline
            }
        }

        StyledRect {
            color: Colours.tPalette.m3surfaceContainerHigh
            radius: Tokens.rounding.medium

            implicitWidth: cancelText.implicitWidth + Tokens.padding.medium * 2
            implicitHeight: cancelText.implicitHeight + Tokens.padding.medium * 2

            StateLayer {
                onClicked: {
                    root.dialog.rejected();
                }
            }

            StyledText {
                id: cancelText

                anchors.centerIn: parent
                anchors.margins: Tokens.padding.medium

                text: qsTr("Cancel")
            }
        }
    }
}
