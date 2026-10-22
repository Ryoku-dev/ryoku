pragma ComponentBehavior: Bound

import "items"
import "services"
import QtQuick
import QtQuick.Layouts
import Quickshell
import Caelestia.Config
import caelestia.components
import caelestia.components.containers
import caelestia.components.controls
import caelestia.services

StyledListView {
    id: root

    required property StyledTextField search
    required property DrawerVisibilities visibilities

    readonly property string searchQuery: (search.text.slice((GlobalConfig.launcher.actionPrefix + "animations ").length)).toLowerCase()

    function refreshModel() {
        const results = Animations.query(search.text);
        model.values = results;
    }

    function handleSearchTextChanged() {
        refreshModel();
    }

    Component.onCompleted: {
        refreshModel();
    }

    model: ScriptModel {
        id: model

        values: []
        onValuesChanged: root.currentIndex = 0
    }

    onVisibleChanged: {
        if (visible) {
            refreshModel();
        }
    }

    onStateChanged: {
        if (state === "animations") {
            refreshModel();
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
            from: 1
            to: 0
        }
    }

    spacing: Tokens.spacing.small
    orientation: Qt.Vertical
    implicitHeight: Math.max(0, (Tokens.sizes.launcher.itemHeight + spacing) * Math.min(Config.launcher.maxShown, count) - spacing)

    preferredHighlightBegin: 0
    preferredHighlightEnd: height
    highlightRangeMode: ListView.ApplyRange

    highlightFollowsCurrentItem: false
    highlight: StyledRect {
        radius: Tokens.rounding.large
        color: Colours.palette.m3onSurface
        opacity: 0.08

        y: root.currentItem?.y ?? 0
        implicitWidth: root.width
        implicitHeight: root.currentItem?.implicitHeight ?? 0

        Behavior on y {
            Anim {
                type: Anim.DefaultSpatial
            }
        }
    }

    delegate: Item {
        id: delegateRoot

        required property var modelData

        implicitHeight: Tokens.sizes.launcher.itemHeight
        width: root.width

        function clicked() {
            if (!modelData || !modelData.onClicked)
                return;
            modelData.onClicked(root);
        }

        StateLayer {
            radius: Tokens.rounding.large
            onClicked: delegateRoot.clicked()
        }

        Item {
            anchors.fill: parent
            anchors.leftMargin: Tokens.padding.medium
            anchors.rightMargin: Tokens.padding.medium
            anchors.margins: Tokens.padding.small

            MaterialIcon {
                id: icon

                text: "animation"
                fontStyle: Tokens.font.icon.builders.large.scale(1.3).build()
                anchors.verticalCenter: parent.verticalCenter
                anchors.left: parent.left
            }

            ColumnLayout {
                anchors.left: icon.right
                anchors.leftMargin: Tokens.spacing.medium
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                spacing: 0

                StyledText {
                    text: modelData ? modelData.name : ""
                    font: Tokens.font.body.medium
                    color: Colours.palette.m3onSurface
                    elide: Text.ElideRight
                }

                StyledText {
                    text: modelData ? (modelData.path === "default" ? qsTr("Use default shell animations") : qsTr("Click to apply animation")) : ""
                    font: Tokens.font.body.small
                    color: Colours.palette.m3onSurfaceVariant
                    elide: Text.ElideRight
                }
            }
        }
    }

    Connections {
        function onTextChanged() {
            handleSearchTextChanged();
        }

        target: search
    }

    Connections {
        function onLoaded() {
            refreshModel();
        }

        target: Animations
    }
}
