pragma ComponentBehavior: Bound

import "items"
import "services"
import QtQuick
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

    readonly property string searchQuery: (search.text.slice((GlobalConfig.launcher.actionPrefix + "keybinds ").length)).toLowerCase()

    function refreshModel() {
        const results = Keybinds.query(searchQuery);
        model.values = results;
    }

    function handleKeybindsLoaded() {
        refreshModel();
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
        if (state === "keybinds") {
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

    delegate: KeybindItem {
        list: root
    }

    Connections {
        function onLoaded() {
            handleKeybindsLoaded();
        }

        target: Keybinds
    }

    Connections {
        function onTextChanged() {
            handleSearchTextChanged();
        }

        target: search
    }
}