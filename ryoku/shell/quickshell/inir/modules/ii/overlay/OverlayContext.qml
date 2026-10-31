pragma Singleton
pragma ComponentBehavior: Bound

import Quickshell

Singleton {
    id: root

    readonly property list<var> availableWidgets: [
        { identifier: "crosshair", materialSymbol: "point_scan" },
        { identifier: "fpsLimiter", materialSymbol: "animation" },
        { identifier: "floatingImage", materialSymbol: "imagesmode" },
        { identifier: "resources", materialSymbol: "browse_activity" },
        { identifier: "gamePerformance", materialSymbol: "monitor_heart" },
        { identifier: "notes", materialSymbol: "note_stack" },
        { identifier: "discord", materialSymbol: "forum" },
        { identifier: "volumeMixer", materialSymbol: "volume_up" },
        { identifier: "notifications", materialSymbol: "notifications" }
    ]

    readonly property bool hasPinnedWidgets: root.pinnedWidgetIdentifiers.length > 0
    readonly property bool nativeDialogOpen: Object.keys(root._nativeDialogs).length > 0

    property list<string> pinnedWidgetIdentifiers: []
    property list<var> clickableWidgets: []
    property var _nativeDialogs: ({})

    function setNativeDialogVisible(dialogKey: string, visible: bool): void {
        const key = String(dialogKey ?? "").trim()
        if (!key) return
        const next = Object.assign({}, root._nativeDialogs)
        if (visible) next[key] = true
        else delete next[key]
        root._nativeDialogs = next
    }

    function pin(identifier: string, pinned = true): void {
        const values = root.pinnedWidgetIdentifiers.filter(id => id !== identifier)
        if (pinned) values.push(identifier)
        root.pinnedWidgetIdentifiers = values
    }

    function registerClickableWidget(widget: var, clickable = true): void {
        const values = root.clickableWidgets.filter(item => item !== widget)
        if (clickable && widget) values.push(widget)
        root.clickableWidgets = values
    }
}
