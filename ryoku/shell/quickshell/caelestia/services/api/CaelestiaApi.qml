pragma Singleton

import "../" as Services
import "../../utils" as Utils
import QtQuick
import Caelestia as Backend

QtObject {
    id: apiRoot

    readonly property SystemApi system: SystemApi {}
    readonly property WindowsApi windows: WindowsApi {}
    readonly property NetworkApi network: NetworkApi {}
    readonly property MediaApi media: MediaApi {}
    readonly property VisualsApi visuals: VisualsApi {}
    readonly property UiApi ui: UiApi {}
    readonly property ShortcutsApi shortcuts: ShortcutsApi {}
    readonly property PluginsApi plugins: PluginsApi {}
}
