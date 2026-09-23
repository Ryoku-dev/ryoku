pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.FrameBars
import "framebars/menus"

// Finite host for menu widgets: a closed switch resolves the generic
// composition primitives Task 6 ships. A catalogued but not-yet-implemented
// widget id logs a developer-visible error and renders nothing; Tasks 7-8
// extend the switch with real content components.
Item {
    id: root

    property string widgetId: ""
    property var widgetData: null
    property bool open: false
    property real scale: 1
    property int depth: 0
    property real avail: 0
    property string initialPage: ""
    property bool incubate: false
    signal requestClose()

    implicitWidth: loader.item ? loader.item.implicitWidth : 0
    implicitHeight: loader.item ? loader.item.implicitHeight : 0
    readonly property bool loaded: loader.status === Loader.Ready && loader.item !== null

    function componentFor(id) {
        switch (id) {
        case "container": return containerComponent;
        case "divider": return dividerComponent;
        case "spacer": return spacerComponent;
        case "clock": return clockComponent;
        case "notifications": return notificationsComponent;
        case "network": return networkComponent;
        case "bluetooth": return bluetoothComponent;
        case "audio-input": return audioInputComponent;
        case "audio-output": return audioOutputComponent;
        case "power-profile": return powerProfileComponent;
        case "quick-settings": return quickSettingsComponent;
        case "quick-actions": return quickActionsComponent;
        case "layout-switcher": return layoutSwitcherComponent;
        case "theme": return themeComponent;
        case "weather": return weatherComponent;
        case "media": return mediaComponent;
        default:
            if (MenuCatalog.widget(id)) console.error("frame menus: no host component for " + id);
            return null;
        }
    }

    Connections {
        target: loader.item
        ignoreUnknownSignals: true
        function onRequestClose() { root.requestClose(); }
    }

    Loader {
        id: loader
        width: root.width
        asynchronous: root.incubate
        sourceComponent: root.componentFor(root.widgetId)
    }

    Component {
        id: containerComponent
        MenuContainer {
            width: root.width
            scale: root.scale
            open: root.open
            depth: root.depth
            orientation: root.widgetData && root.widgetData.orientation ? root.widgetData.orientation : "vertical"
            widgets: root.widgetData && root.widgetData.widgets ? root.widgetData.widgets : []
        }
    }
    Component { id: dividerComponent; MenuDivider { scale: root.scale } }
    Component { id: spacerComponent; MenuSpacer { scale: root.scale } }
    Component { id: clockComponent; MenuClock { width: root.width; s: root.scale; open: root.open } }
    Component { id: notificationsComponent; MenuNotifications { width: root.width; height: implicitHeight; s: root.scale; open: root.open } }
    Component { id: networkComponent; MenuNetwork { width: root.width; s: root.scale; open: root.open } }
    Component { id: bluetoothComponent; MenuBluetooth { width: root.width; s: root.scale; open: root.open } }
    Component { id: audioInputComponent; MenuAudioInput { width: root.width; s: root.scale; open: root.open } }
    Component { id: audioOutputComponent; MenuAudioOutput { width: root.width; s: root.scale; open: root.open } }
    Component { id: powerProfileComponent; MenuPowerProfile { width: root.width; s: root.scale; open: root.open } }
    Component { id: quickSettingsComponent; MenuQuickSettings { width: root.width; s: root.scale; open: root.open; avail: root.avail; initialPage: root.initialPage } }
    Component { id: quickActionsComponent; MenuQuickActions { width: root.width; s: root.scale; open: root.open } }
    Component { id: layoutSwitcherComponent; MenuLayoutSwitcher { width: root.width; s: root.scale; open: root.open } }
    Component { id: themeComponent; MenuTheme { width: root.width; s: root.scale; open: root.open } }
    Component { id: weatherComponent; MenuWeather { width: root.width; s: root.scale; open: root.open } }
    Component { id: mediaComponent; MenuMedia { width: root.width; s: root.scale; open: root.open } }
}
