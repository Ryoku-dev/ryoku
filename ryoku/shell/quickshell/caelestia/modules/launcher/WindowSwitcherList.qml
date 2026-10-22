pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Caelestia.Config
import caelestia.components.controls
import caelestia.services
import "items"
import "services"

ListView {
    id: root

    required property StyledTextField search
    required property var visibilities
    required property var panels
    required property var content

    readonly property int itemWidth: Tokens.sizes.launcher.windowSwitcherWidth * 0.8 + Tokens.padding.largeIncreased * 2

    readonly property int numItems: {
        const screen = (QsWindow.window as QsWindow)?.screen;
        if (!screen)
            return 0;

        const isBarHorizontal = Config.bar.position === "top" || Config.bar.position === "bottom";
        const barThickness = isBarHorizontal ? panels.bar.implicitHeight : panels.bar.implicitWidth;
        const barMargins = Math.max(Config.border.thickness, barThickness);
        let outerMargins = 0;
        if (panels.popouts.hasCurrent && panels.popouts.currentCenter + panels.popouts.nonAnimHeight / 2 > screen.height - content.implicitHeight - Config.border.thickness * 2)
            outerMargins = panels.popouts.nonAnimWidth;
        if ((visibilities.utilities || visibilities.sidebar) && panels.utilities.implicitWidth > outerMargins)
            outerMargins = panels.utilities.implicitWidth;
        const maxWidth = screen.width - Config.border.rounding * 4 - (barMargins + outerMargins) * 2;

        if (maxWidth <= 0)
            return 0;

        const maxItemsOnScreen = Math.floor(maxWidth / itemWidth);
        const visible = Math.min(maxItemsOnScreen, 10, scriptModel.values.length);

        if (visible === 2)
            return 1;
        if (visible > 1 && visible % 2 === 0)
            return visible - 1;
        return visible;
    }

    function incrementCurrentIndex(): void {
        if (scriptModel.search.trim() !== "") {
            if (root.count > 0)
                root.currentIndex = (root.currentIndex + 1) % root.count;
        } else {
            Windows.triggerCycleNext();
        }
    }

    function decrementCurrentIndex(): void {
        if (scriptModel.search.trim() !== "") {
            if (root.count > 0)
                root.currentIndex = (root.currentIndex - 1 + root.count) % root.count;
        } else {
            Windows.triggerCyclePrev();
        }
    }

    implicitWidth: Math.min(numItems, count) * itemWidth

    orientation: ListView.Horizontal
    snapMode: ListView.SnapToItem
    preferredHighlightBegin: root.width / 2 - itemWidth / 2
    preferredHighlightEnd: root.width / 2 + itemWidth / 2
    highlightRangeMode: ListView.StrictlyEnforceRange
    highlightMoveDuration: Tokens.anim.durations.expressiveFastSpatial

    Component.onCompleted: {
        root.currentIndex = Qt.binding(() => Windows.selectedIndex);
    }

    model: ScriptModel {
        id: scriptModel

        readonly property string search: root.search.text.split(" ").slice(1).join(" ")

        values: {
            const _ = Windows.items;
            return Windows.query(search);
        }
        onValuesChanged: {
            if (scriptModel.search.trim() !== "") {
                root.currentIndex = 0;
            } else {
                root.currentIndex = Qt.binding(() => Windows.selectedIndex);
            }
        }
    }

    delegate: WindowSwitcherItem {
        list: root
    }

    MouseArea {
        anchors.fill: parent
        propagateComposedEvents: true
        onWheel: wheel => {
            if (wheel.angleDelta.y > 0)
                root.decrementCurrentIndex();
            else
                root.incrementCurrentIndex();
            wheel.accepted = true;
        }
    }
}

