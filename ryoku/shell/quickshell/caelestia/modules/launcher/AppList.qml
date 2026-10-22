pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Caelestia.Config
import caelestia.components
import caelestia.components.containers
import caelestia.components.controls
import caelestia.services
import caelestia.modules.launcher.items
import caelestia.modules.launcher.services

StyledListView {
    id: root

    required property StyledTextField search
    required property DrawerVisibilities visibilities

    property string displayText

    readonly property string requestedState: stateForText(search.text)
    readonly property string displayState: stateForText(displayText)

    function syncDisplayText(): void {
        if (visibilities.launcher && requestedState === displayState)
            displayText = search.text;
    }

    function stateForText(text: string): string {
        const prefix = GlobalConfig.launcher.actionPrefix;
        if (text.startsWith(prefix)) {
            for (const action of ["calc", "scheme", "variant", "emoji", "clipboard", "windows"])
                if (text.startsWith(`${prefix}${action} `))
                    return action;

            return "actions";
        }

        return "apps";
    }

    function resultsForText(text: string): var {
        switch (stateForText(text)) {
        case "actions":
            return Actions.query(text);
        case "calc":
            return [0];
        case "scheme":
            return Schemes.query(text);
        case "variant":
            return M3Variants.query(text);
        case "emoji": {
            const prefix = GlobalConfig.launcher.actionPrefix;
            const q = root._debouncedSearchText.slice((prefix + "emoji ").length).toLowerCase();
            if (!q)
                return Emojis.getSortedItems();
            return Emojis.search(q);
        }
        case "clipboard": {
            const prefix = GlobalConfig.launcher.actionPrefix;
            const q = root.search.text.slice((prefix + "clipboard ").length).toLowerCase();
            if (!q)
                return Clipboard.getSortedItems();
            return Clipboard.items.filter(function (item) {
                return item.preview.toLowerCase().includes(q);
            });
        }
        case "windows":
            return Windows.items;
        default:
            return Apps.search(text);
        }
    }

    function openContextMenu(app: DesktopEntry, targetItem: Item): void {
        contextMenu.openFor(app, targetItem);
    }

    model: ScriptModel {
        values: root.resultsForText(root.displayText)
        onValuesChanged: root.currentIndex = 0
    }

    spacing: Tokens.spacing.small
    orientation: Qt.Vertical
    implicitHeight: Math.max(0, (Tokens.sizes.launcher.itemHeight + spacing) * Math.min(Config.launcher.maxShown, count) - spacing)
    cacheBuffer: Tokens.sizes.launcher.itemHeight * 10

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
            Anim {}
        }
    }

    property string _debouncedSearchText: search.text

    Timer {
        id: searchDebounceTimer

        interval: 80
        onTriggered: root._debouncedSearchText = search.text
    }
    Connections {
        target: search

        function onTextChanged(): void {
            if (root.state === "emoji") {
                searchDebounceTimer.restart();
            } else {
                root._debouncedSearchText = search.text;
            }
        }
    }

    state: visibilities.launcher ? requestedState : displayState

    onStateChanged: {
        if (state === "scheme" || state === "variant")
            Schemes.reload();
        if (state === "emoji")
            Emojis.reload();
        if (state === "clipboard")
            Clipboard.reload();
            
        if (state !== "scheme" && state !== "variant") {
            Colours.showPreview = false;
        }
    }

    Component.onDestruction: {
        Colours.showPreview = false;
    }

    // shortcut: the desktop themes carry three swatches, not a palette, and the
    // theme daemon has no preview mode, so the scheme and variant lists do not
    // preview on hover; add one if `ryoku-shell theme catalog` grows full palettes.

    Component.onCompleted: displayText = search.text

    states: [
        State {
            name: "apps"

            PropertyChanges {
                root.delegate: appItem
            }
        },
        State {
            name: "actions"

            PropertyChanges {
                root.delegate: actionItem
            }
        },
        State {
            name: "calc"

            PropertyChanges {
                root.delegate: calcItem
            }
        },
        State {
            name: "scheme"

            PropertyChanges {
                root.delegate: schemeItem
            }
        },
        State {
            name: "variant"

            PropertyChanges {
                root.delegate: variantItem
            }
        },
        State {
            name: "emoji"

            PropertyChanges {
                root.delegate: emojiItem
            }
        },
        State {
            name: "clipboard"

            PropertyChanges {
                root.delegate: clipItem
            }
        },
        State {
            name: "windows"

            PropertyChanges {
                root.delegate: windowsItem
            }
        }
    ]

    transitions: Transition {
        SequentialAnimation {
            ParallelAnimation {
                Anim {
                    target: root
                    property: "opacity"
                    from: 1
                    to: 0
                    duration: Tokens.anim.durations.small
                    easing: Tokens.anim.standardAccel
                }
                Anim {
                    target: root
                    property: "scale"
                    from: 1
                    to: 0.9
                    duration: Tokens.anim.durations.small
                    easing: Tokens.anim.standardAccel
                }
            }
            PropertyAction {
                target: root
                property: "delegate"
                value: null
            }
            ScriptAction {
                script: root.displayText = root.search.text
            }
            PropertyAction {
                target: root
                property: "delegate"
            }
            ParallelAnimation {
                Anim {
                    target: root
                    property: "opacity"
                    from: 0
                    to: 1
                    duration: Tokens.anim.durations.small
                    easing: Tokens.anim.standardDecel
                }
                Anim {
                    target: root
                    property: "scale"
                    from: 0.9
                    to: 1
                    duration: Tokens.anim.durations.small
                    easing: Tokens.anim.standardDecel
                }
            }
            PropertyAction {
                targets: [root.add, root.remove]
                property: "enabled"
                value: true
            }
        }
    }

    StyledScrollBar.vertical: StyledScrollBar {
        flickable: root
    }

    add: Transition {
        enabled: !root.state

        Anim {
            type: Anim.DefaultEffects
            property: "opacity"
            from: 0
            to: 1
        }
    }

    remove: Transition {
        enabled: !root.state

        Anim {
            type: Anim.DefaultEffects
            property: "opacity"
            from: 1
            to: 0
        }
    }

    move: Transition {
        Anim {
            property: "y"
        }
        Anim {
            type: Anim.DefaultEffects
            property: "opacity"
            to: 1
        }
    }

    addDisplaced: Transition {
        Anim {
            property: "y"
            type: Anim.StandardSmall
        }
        Anim {
            type: Anim.DefaultEffects
            property: "opacity"
            to: 1
        }
    }

    displaced: Transition {
        Anim {
            property: "y"
        }
        Anim {
            type: Anim.DefaultEffects
            property: "opacity"
            to: 1
        }
    }

    Component {
        id: appItem

        AppItem {
            list: root
            visibilities: root.visibilities
        }
    }

    Component {
        id: actionItem

        ActionItem {
            list: root
        }
    }

    Component {
        id: calcItem

        CalcItem {
            list: root
        }
    }

    Component {
        id: schemeItem

        SchemeItem {
            list: root
        }
    }

    Component {
        id: variantItem

        VariantItem {
            list: root
        }
    }

    Component {
        id: emojiItem

        EmojiItem {
            list: root
        }
    }

    Component {
        id: clipItem

        ClipItem {
            list: root
        }
    }

    Component {
        id: windowsItem

        WindowSwitcherItem {
            list: root
        }
    }

    AppContextMenu {
        id: contextMenu

        attachTo: root
        visibilities: root.visibilities
    }

    Connections {
        function onTextChanged() {
            root.syncDisplayText();
        }

        target: root.search
    }

    Connections {
        function onLauncherChanged() {
            root.syncDisplayText();
        }

        target: root.visibilities
    }
}
