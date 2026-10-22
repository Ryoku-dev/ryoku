import Quickshell
import Quickshell.Wayland
import Caelestia.Config

// qmllint disable uncreatable-type
PanelWindow {
    // qmllint enable uncreatable-type
    required property string name

    property bool isDesktopWidget: false
    WlrLayershell.namespace: isDesktopWidget ? "desktop" : "panel"

    color: "transparent"

    contentItem.Config.screen: screen.name
    contentItem.Tokens.screen: screen.name
}
