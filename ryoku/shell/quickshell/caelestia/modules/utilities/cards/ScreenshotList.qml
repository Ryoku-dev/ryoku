pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Qt.labs.folderlistmodel
import Quickshell
import Quickshell.Widgets
import Caelestia
import Caelestia.Config
import Caelestia.Models
import caelestia.components
import caelestia.components.containers
import caelestia.components.controls
import caelestia.services
import caelestia.utils

ColumnLayout {
    id: root

    required property var props
    required property DrawerVisibilities visibilities

    property string confirmDelete: ""
    readonly property string shotsdir: GlobalConfig.paths.screenshotsDir

    spacing: 0

    WrapperMouseArea {
        Layout.fillWidth: true

        cursorShape: Qt.PointingHandCursor
        onClicked: root.props.screenshotListExpanded = !root.props.screenshotListExpanded

        RowLayout {
            spacing: Tokens.spacing.medium

            MaterialIcon {
                Layout.alignment: Qt.AlignVCenter
                text: "list"
                fontStyle: Tokens.font.icon.large
            }

            StyledText {
                Layout.alignment: Qt.AlignVCenter
                Layout.fillWidth: true
                text: qsTr("Screenshots")
                font: Tokens.font.body.medium
            }

            IconButton {
                icon: root.props.screenshotListExpanded ? "unfold_less" : "unfold_more"
                type: IconButton.Text
                label.animate: true
                onClicked: root.props.screenshotListExpanded = !root.props.screenshotListExpanded
            }
        }
    }

    StyledListView {
        id: list

        model: FolderListModel {
            folder: "file://" + root.shotsdir
            nameFilters: ["screenshot-*.png"]
            sortField: FolderListModel.Time
            sortReversed: false
        }

        Layout.fillWidth: true
        Layout.rightMargin: -Tokens.spacing.small
        implicitHeight: (Tokens.font.body.large.pointSize + Tokens.padding.small) * (root.props.screenshotListExpanded ? 10 : 3)
        clip: true

        StyledScrollBar.vertical: StyledScrollBar {
            flickable: list
        }

        delegate: RowLayout {
            id: shot

            required property string fileBaseName
            required property string filePath

            property string baseName: fileBaseName
            property string path: filePath
            property bool confirming: root.confirmDelete === path

            anchors.left: list.contentItem.left
            anchors.right: list.contentItem.right
            anchors.rightMargin: Tokens.spacing.small
            spacing: Tokens.spacing.extraSmall

            StyledText {
                Layout.fillWidth: true
                Layout.rightMargin: Tokens.spacing.extraSmall
                visible: !shot.confirming
                text: {
                    const time = shot.baseName;
                    const matches = time.match(/^screenshot-(\d{4})-(\d{2})-(\d{2})_(\d{2})\.(\d{2})\.(\d{2})/);
                    if (!matches)
                        return time;
                    const date = new Date(matches[1], matches[2] - 1, matches[3], matches[4], matches[5], matches[6]);
                    return qsTr("Screenshot at %1").arg(Qt.formatDateTime(date, Qt.locale()));
                }
                color: Colours.palette.m3onSurfaceVariant
                elide: Text.ElideRight
            }

            IconButton {
                visible: !shot.confirming
                icon: "play_arrow"
                type: IconButton.Text
                onClicked: {
                    root.visibilities.utilities = false;
                    if (!Visibilities.sidebarPinned)
                        root.visibilities.sidebar = false;
                    Quickshell.execDetached(["xdg-open", shot.path]);
                }
            }

            IconButton {
                visible: !shot.confirming
                icon: "folder"
                type: IconButton.Text
                onClicked: {
                    root.visibilities.utilities = false;
                    if (!Visibilities.sidebarPinned)
                        root.visibilities.sidebar = false;
                    Quickshell.execDetached(["xdg-open", root.shotsdir]);
                }
            }

            IconButton {
                visible: !shot.confirming
                icon: "delete_forever"
                type: IconButton.Text
                label.color: Colours.palette.m3error
                stateLayer.color: Colours.palette.m3error
                onClicked: root.confirmDelete = shot.path
            }

            StyledText {
                Layout.fillWidth: true
                visible: shot.confirming
                text: qsTr("Delete this screenshot?")
                color: Colours.palette.m3onSurfaceVariant
                elide: Text.ElideRight
            }

            TextButton {
                visible: shot.confirming
                text: qsTr("Cancel")
                type: TextButton.Text
                onClicked: root.confirmDelete = ""
            }

            TextButton {
                visible: shot.confirming
                text: qsTr("Delete")
                type: TextButton.Text
                onClicked: {
                    CUtils.deleteFile(Qt.resolvedUrl(shot.path));
                    root.confirmDelete = "";
                }
            }
        }

        add: Transition {
            Anim {
                type: Anim.DefaultEffects
                property: "opacity"
                from: 0
                to: 1
            }
        }

        remove: Transition {
            Anim {
                type: Anim.DefaultEffects
                property: "opacity"
                to: 0
            }
        }

        displaced: Transition {
            Anim {
                type: Anim.DefaultEffects
                property: "opacity"
                to: 1
            }
            Anim {
                property: "y"
            }
        }

        Loader {
            asynchronous: true
            anchors.centerIn: parent

            opacity: list.count === 0 ? 1 : 0
            active: opacity > 0

            sourceComponent: ColumnLayout {
                spacing: Tokens.spacing.small

                MaterialIcon {
                    Layout.alignment: Qt.AlignHCenter
                    text: "scan_delete"
                    color: Colours.palette.m3outline
                    fontStyle: Tokens.font.icon.extraLarge

                    opacity: root.props.screenshotListExpanded ? 1 : 0
                    scale: root.props.screenshotListExpanded ? 1 : 0
                    Layout.preferredHeight: root.props.screenshotListExpanded ? implicitHeight : 0

                    Behavior on opacity {
                        Anim {
                            type: Anim.DefaultEffects
                        }
                    }

                    Behavior on scale {
                        Anim {}
                    }

                    Behavior on Layout.preferredHeight {
                        Anim {}
                    }
                }

                RowLayout {
                    spacing: Tokens.spacing.medium

                    MaterialIcon {
                        Layout.alignment: Qt.AlignHCenter
                        text: "scan_delete"
                        color: Colours.palette.m3outline

                        opacity: !root.props.screenshotListExpanded ? 1 : 0
                        scale: !root.props.screenshotListExpanded ? 1 : 0
                        Layout.preferredWidth: !root.props.screenshotListExpanded ? implicitWidth : 0

                        Behavior on opacity {
                            Anim {
                                type: Anim.DefaultEffects
                            }
                        }

                        Behavior on scale {
                            Anim {}
                        }

                        Behavior on Layout.preferredWidth {
                            Anim {}
                        }
                    }

                    StyledText {
                        text: qsTr("No screenshots found")
                        color: Colours.palette.m3outline
                    }
                }
            }

            Behavior on opacity {
                Anim {
                    type: Anim.DefaultEffects
                }
            }
        }

        Behavior on implicitHeight {
            Anim {}
        }
    }
}
