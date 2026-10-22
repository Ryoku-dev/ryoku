pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Qt.labs.folderlistmodel
import Quickshell
import Quickshell.Widgets
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

    spacing: 0

    WrapperMouseArea {
        Layout.fillWidth: true

        cursorShape: Qt.PointingHandCursor
        onClicked: root.props.recordingListExpanded = !root.props.recordingListExpanded

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
                text: qsTr("Recordings")
                font: Tokens.font.body.medium
            }

            IconButton {
                icon: root.props.recordingListExpanded ? "unfold_less" : "unfold_more"
                type: IconButton.Text
                label.animate: true
                onClicked: root.props.recordingListExpanded = !root.props.recordingListExpanded
            }
        }
    }

    StyledListView {
        id: list

        model: FolderListModel {
            folder: "file://" + GlobalConfig.paths.recordingsDir
            nameFilters: ["recording_*.mp4", "recording_*.gif"]
            sortField: FolderListModel.Time
            sortReversed: false
        }

        Layout.fillWidth: true
        Layout.rightMargin: -Tokens.spacing.small
        implicitHeight: (Tokens.font.body.large.pointSize + Tokens.padding.small) * (root.props.recordingListExpanded ? 10 : 3)
        clip: true

        StyledScrollBar.vertical: StyledScrollBar {
            flickable: list
        }

        delegate: RowLayout {
            id: recording

            required property string fileBaseName
            required property string filePath

            property string baseName: fileBaseName
            property string path: filePath

            anchors.left: list.contentItem.left
            anchors.right: list.contentItem.right
            anchors.rightMargin: Tokens.spacing.small
            spacing: Tokens.spacing.extraSmall

            StyledText {
                Layout.fillWidth: true
                Layout.rightMargin: Tokens.spacing.extraSmall
                text: {
                    const time = recording.baseName;
                    const matches = time.match(/^recording_(\d{4})(\d{2})(\d{2})_(\d{2})-(\d{2})-(\d{2})/);
                    if (!matches)
                        return time;
                    const date = new Date(...matches.slice(1));
                    date.setMonth(date.getMonth() - 1);
                    return qsTr("Recording at %1").arg(Qt.formatDateTime(date, Qt.locale()));
                }
                color: Colours.palette.m3onSurfaceVariant
                elide: Text.ElideRight
            }

            IconButton {
                icon: "play_arrow"
                type: IconButton.Text
                onClicked: {
                    root.visibilities.utilities = false;
                    if (!Visibilities.sidebarPinned)
                        root.visibilities.sidebar = false;
                    Quickshell.execDetached([...GlobalConfig.general.apps.playback, recording.path]);
                }
            }

            IconButton {
                icon: "folder"
                type: IconButton.Text
                onClicked: {
                    root.visibilities.utilities = false;
                    if (!Visibilities.sidebarPinned)
                        root.visibilities.sidebar = false;
                    Quickshell.execDetached([...GlobalConfig.general.apps.explorer, GlobalConfig.paths.recordingsDir]);
                }
            }

            IconButton {
                icon: "delete_forever"
                type: IconButton.Text
                label.color: Colours.palette.m3error
                stateLayer.color: Colours.palette.m3error
                onClicked: root.props.recordingConfirmDelete = recording.path
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

                    opacity: root.props.recordingListExpanded ? 1 : 0
                    scale: root.props.recordingListExpanded ? 1 : 0
                    Layout.preferredHeight: root.props.recordingListExpanded ? implicitHeight : 0

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

                        opacity: !root.props.recordingListExpanded ? 1 : 0
                        scale: !root.props.recordingListExpanded ? 1 : 0
                        Layout.preferredWidth: !root.props.recordingListExpanded ? implicitWidth : 0

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
                        text: qsTr("No recordings found")
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
