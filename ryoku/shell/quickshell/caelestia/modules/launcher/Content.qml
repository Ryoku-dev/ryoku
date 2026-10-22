pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import Quickshell
import Caelestia
import Caelestia.Config
import Caelestia.Services
import caelestia.components
import caelestia.components.controls
import caelestia.services
import caelestia.modules.launcher.services

Item {
    id: root

    required property DrawerVisibilities visibilities
    required property var panels
    required property real maxWidth
    required property real maxHeight

    readonly property int padding: Tokens.padding.large
    readonly property int rounding: Tokens.rounding.extraLarge
    readonly property bool isClipboardMode: search.text.startsWith(`${GlobalConfig.launcher.actionPrefix}clipboard `)
    readonly property int footerSpacing: Tokens.spacing.small

    function clearClipboardHistory(): void {
        Clipboard.clearHistory();
    }

    function triggerSessionCommand(command: list<string>): void {
        root.visibilities.launcher = false;
        if (!SessionManager.exec(command))
            Quickshell.execDetached(command);
    }

    implicitWidth: listWrapper.width + padding * 2
    implicitHeight: listWrapper.height + sessionFooter.height + searchWrapper.height + listWrapper.anchors.bottomMargin + sessionFooter.anchors.bottomMargin + searchWrapper.anchors.bottomMargin

    Connections {
        function onClearHistoryFinished(success: bool): void {
            if (success) {
                if (GlobalConfig.utilities.toasts.clipboardChanged)
                    Toaster.toast(qsTr("Clipboard history cleared"), "", "delete");
            } else {
                Toaster.toast(qsTr("Failed to clear clipboard history"), "", "error");
            }
        }

        target: Clipboard
    }

    Item {
        id: listWrapper

        implicitWidth: list.implicitWidth
        implicitHeight: list.implicitHeight + root.padding

        anchors.horizontalCenter: parent.horizontalCenter
        anchors.bottom: sessionFooter.top
        anchors.bottomMargin: root.footerSpacing

        ContentList {
            id: list

            content: root
            visibilities: root.visibilities
            panels: root.panels
            maxWidth: root.maxWidth - root.padding * 2
            maxHeight: root.maxHeight - searchWrapper.implicitHeight - sessionFooter.implicitHeight - root.padding * 2 - (sessionFooter.visible ? root.footerSpacing * 2 : root.footerSpacing)
            search: search
            padding: root.padding
            rounding: root.rounding
        }
    }

    StyledRect {
        id: sessionFooter

        visible: Config.launcher.showPowerMenu

        color: Colours.layer(Colours.palette.m3surfaceContainerLow, 2)
        radius: Tokens.rounding.extraLarge

        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: searchWrapper.top
        anchors.bottomMargin: visible ? root.footerSpacing : 0
        anchors.leftMargin: root.padding
        anchors.rightMargin: root.padding

        implicitHeight: visible ? (footerLayout.implicitHeight + Tokens.padding.medium * 2) : 0

        ColumnLayout {
            id: footerLayout

            anchors.fill: parent
            anchors.margins: Tokens.padding.medium
            spacing: Tokens.spacing.small

            // StyledText {
            //     Layout.fillWidth: true
            //     text: qsTr("Quick session controls")
            //     color: Colours.palette.m3onSurfaceVariant
            //     font: Tokens.font.label.large
            // }

            RowLayout {
                Layout.fillWidth: true
                spacing: Tokens.spacing.small

                IconTextButton {
                    Layout.fillWidth: true
                    type: TextButton.Tonal
                    icon: "logout"
                    text: qsTr("Log Out")
                    onClicked: root.triggerSessionCommand(["sh", "-c", "qdbus6 org.kde.Shutdown /Shutdown org.kde.Shutdown.logout 2>/dev/null"])
                }

                IconTextButton {
                    Layout.fillWidth: true
                    type: TextButton.Tonal
                    icon: "bedtime"
                    text: qsTr("Sleep")
                    onClicked: root.triggerSessionCommand(["suspendThenHibernate"])
                }

                IconTextButton {
                    Layout.fillWidth: true
                    type: TextButton.Tonal
                    icon: "restart_alt"
                    text: qsTr("Restart")
                    inactiveColour: Colours.palette.m3secondaryContainer
                    inactiveOnColour: Colours.palette.m3onSecondaryContainer
                    onClicked: root.triggerSessionCommand(Config.session.commands.reboot)
                }

                IconTextButton {
                    Layout.fillWidth: true
                    type: TextButton.Tonal
                    icon: "power_settings_new"
                    text: qsTr("Shut Down")
                    inactiveColour: Colours.palette.m3errorContainer
                    inactiveOnColour: Colours.palette.m3onErrorContainer
                    onClicked: root.triggerSessionCommand(Config.session.commands.shutdown)
                }
            }
        }
    }

    StyledRect {
        id: searchWrapper

        color: Colours.layer(Colours.palette.m3surfaceContainer, 2)
        radius: Tokens.rounding.full

        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        anchors.margins: root.padding
        anchors.bottomMargin: CUtils.clamp(root.padding - Config.border.thickness, 0, root.padding)

        implicitHeight: Math.max(searchIcon.implicitHeight, search.implicitHeight, clearClipboardIcon.implicitHeight, clearIcon.implicitHeight, commandsBtn.implicitHeight)

        MaterialIcon {
            id: searchIcon

            anchors.verticalCenter: parent.verticalCenter
            anchors.left: parent.left
            anchors.leftMargin: root.padding

            text: "search"
            color: Colours.palette.m3onSurfaceVariant
        }

        StyledTextField {
            id: search

            anchors.left: searchIcon.right
            anchors.right: clearClipboardIcon.left
            anchors.leftMargin: Tokens.spacing.small
            anchors.rightMargin: Tokens.spacing.small

            topPadding: Tokens.padding.medium
            bottomPadding: Tokens.padding.medium

            placeholderText: qsTr("Type \"%1\" for commands").arg(GlobalConfig.launcher.actionPrefix)

            onAccepted: {
                if (list.showAppsBrowser) {
                    list.currentList?.activateCurrent();
                    return;
                }

                const currentItem = list.currentList?.currentItem;
                if (currentItem) {
                    if (list.showWallpapers) {
                        if (Colours.scheme === "dynamic" && currentItem.modelData.path !== Wallpapers.actualCurrent)
                            Wallpapers.previewColourLock = true;
                        Wallpapers.setWallpaper(currentItem.modelData.path);
                        root.visibilities.launcher = false;
                    } else if (text.startsWith(GlobalConfig.launcher.actionPrefix)) {
                        if (text.startsWith(`${GlobalConfig.launcher.actionPrefix}calc `))
                            currentItem.onClicked();
                        else if (text.startsWith(`${GlobalConfig.launcher.actionPrefix}emoji `) || text.startsWith(`${GlobalConfig.launcher.actionPrefix}clipboard `) || text.startsWith(`${GlobalConfig.launcher.actionPrefix}windows `) || text.startsWith(`${GlobalConfig.launcher.actionPrefix}keybinds `) || text.startsWith(`${GlobalConfig.launcher.actionPrefix}animations `))
                            currentItem.clicked();
                        else
                            currentItem.modelData.onClicked(list.currentList);
                    } else {
                        Apps.launch(currentItem.modelData);
                        root.visibilities.launcher = false;
                    }
                }
            }

            Keys.onUpPressed: {
                if (!list.showWallpapers)
                    list.currentList?.decrementCurrentIndex();
            }
            Keys.onDownPressed: {
                if (!list.showWallpapers)
                    list.currentList?.incrementCurrentIndex();
            }
            Keys.onLeftPressed: event => {
                if (list.showWallpapers || list.showWindowSwitcher) {
                    list.currentList?.decrementCurrentIndex();
                    event.accepted = true;
                } else if (list.showAppsBrowser) {
                    list.currentList?.moveLeft();
                    event.accepted = true;
                } else {
                    event.accepted = false;
                }
            }
            Keys.onRightPressed: event => {
                if (list.showWallpapers || list.showWindowSwitcher) {
                    list.currentList?.incrementCurrentIndex();
                    event.accepted = true;
                } else if (list.showAppsBrowser) {
                    list.currentList?.moveRight();
                    event.accepted = true;
                } else {
                    event.accepted = false;
                }
            }

            Keys.onEscapePressed: {
                Windows.isSwitching = false;
                Kwin.clearHighlight();
                root.visibilities.launcher = false;
            }

            Keys.onReleased: event => {
                if (Windows.isSwitching && text.startsWith(`${GlobalConfig.launcher.actionPrefix}windows `)) {
                    const switcherKey = "Alt+Tab";
                    if (!CUtils.isShortcutModifierPressed(switcherKey)) {
                        Windows.focusSelectedWindow();
                        root.visibilities.launcher = false;
                        event.accepted = true;
                    }
                }
            }

            Keys.onPressed: event => {
                if (Windows.isSwitching && text.startsWith(`${GlobalConfig.launcher.actionPrefix}windows `)) {
                    if (event.key === Qt.Key_Tab || event.key === Qt.Key_Backtab) {
                        if (event.modifiers & Qt.ShiftModifier || event.key === Qt.Key_Backtab) {
                            Windows.triggerCyclePrev();
                        } else {
                            Windows.triggerCycleNext();
                        }
                        event.accepted = true;
                        return;
                    }
                    if (event.key === Qt.Key_Left || event.key === Qt.Key_Up) {
                        Windows.triggerCyclePrev();
                        event.accepted = true;
                        return;
                    }
                    if (event.key === Qt.Key_Right || event.key === Qt.Key_Down) {
                        Windows.triggerCycleNext();
                        event.accepted = true;
                        return;
                    }
                    if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter) {
                        Windows.focusSelectedWindow();
                        root.visibilities.launcher = false;
                        event.accepted = true;
                        return;
                    }
                    if (event.key === Qt.Key_Escape) {
                        Windows.isSwitching = false;
                        Kwin.clearHighlight();
                        root.visibilities.launcher = false;
                        event.accepted = true;
                        return;
                    }
                }

                if (list.showAppsBrowser && event.key === Qt.Key_Tab) {
                    list.currentList?.toggleFocus();
                    event.accepted = true;
                    return;
                }

                if (!GlobalConfig.launcher.vimKeybinds)
                    return;

                if (event.modifiers & Qt.ControlModifier) {
                    if (event.key === Qt.Key_J || event.key === Qt.Key_N) {
                        list.currentList?.incrementCurrentIndex();
                        event.accepted = true;
                    } else if (event.key === Qt.Key_K || event.key === Qt.Key_P) {
                        list.currentList?.decrementCurrentIndex();
                        event.accepted = true;
                    }
                } else if (event.key === Qt.Key_Backtab || (event.key === Qt.Key_Tab && (event.modifiers & Qt.ShiftModifier))) {
                    list.currentList?.decrementCurrentIndex();
                    event.accepted = true;
                } else if (event.key === Qt.Key_Tab) {
                    list.currentList?.incrementCurrentIndex();
                    event.accepted = true;
                }
            }

            onTextChanged: {
                if (!text.startsWith(`${GlobalConfig.launcher.actionPrefix}windows `)) {
                    Windows.isSwitching = false;
                    Kwin.clearHighlight();
                }
            }

            Component.onCompleted: {
                if (Visibilities.launcherInitialSearch) {
                    text = Visibilities.launcherInitialSearch;
                    Visibilities.launcherInitialSearch = "";
                }
                forceActiveFocus();
            }

            Connections {
                function onLauncherChanged(): void {
                    if (root.visibilities.launcher) {
                        // Every open decides the field: the requested prefix, or the app grid.
                        search.text = Visibilities.launcherInitialSearch;
                        Visibilities.launcherInitialSearch = "";
                        search.forceActiveFocus();
                    } else {
                        Windows.isSwitching = false;
                        Kwin.clearHighlight();
                        Wallpapers.stopPreview();
                    }
                }

                function onSessionChanged(): void {
                    if (!root.visibilities.session)
                        search.forceActiveFocus();
                }

                target: root.visibilities
            }

            Connections {
                function onClosed(): void {
                    search.text = "";
                }

                target: root.panels.launcher
            }

            Connections {
                function onModifierReleased(): void {
                    if (Windows.isSwitching && root.visibilities.launcher && search.text.startsWith(`${GlobalConfig.launcher.actionPrefix}windows `)) {
                        const switcherKey = "Alt+Tab";
                        if (!CUtils.isShortcutModifierPressed(switcherKey)) {
                            Windows.focusSelectedWindow();
                            root.visibilities.launcher = false;
                        }
                    }
                }

                target: CUtils
            }
        }

        MaterialIcon {
            id: clearClipboardIcon

            anchors.verticalCenter: parent.verticalCenter
            anchors.right: clearIcon.left
            anchors.rightMargin: (root.isClipboardMode && Clipboard.items.length > 0) ? Tokens.spacing.small : 0

            width: (root.isClipboardMode && Clipboard.items.length > 0) ? implicitWidth : 0
            opacity: (root.isClipboardMode && Clipboard.items.length > 0) ? 1 : 0

            text: "delete"
            color: clipboardMouse.containsMouse ? Colours.palette.m3error : Colours.palette.m3onSurfaceVariant

            MouseArea {
                id: clipboardMouse

                anchors.fill: parent
                hoverEnabled: true
                cursorShape: (root.isClipboardMode && Clipboard.items.length > 0) ? Qt.PointingHandCursor : undefined

                onClicked: {
                    if (!root.isClipboardMode || Clipboard.items.length === 0)
                        return;

                    if (GlobalConfig.launcher.confirmClearClipboard)
                        clearClipboardConfirmPopup.open();
                    else
                        root.clearClipboardHistory();
                }
            }

            Behavior on color {
                CAnim {}
            }

            Behavior on width {
                Anim {
                    type: Anim.StandardSmall
                }
            }

            Behavior on opacity {
                Anim {
                    type: Anim.StandardSmall
                }
            }
        }

        MaterialIcon {
            id: clearIcon

            anchors.verticalCenter: parent.verticalCenter
            anchors.right: commandsBtn.left
            anchors.rightMargin: search.text ? Tokens.spacing.small : 0

            width: search.text ? implicitWidth : 0
            opacity: search.text ? 1 : 0

            text: "close"
            color: mouse.containsMouse ? Colours.palette.m3primary : Colours.palette.m3onSurfaceVariant

            MouseArea {
                id: mouse

                anchors.fill: parent
                hoverEnabled: true
                cursorShape: search.text ? Qt.PointingHandCursor : undefined

                onClicked: search.text = ""
            }

            Behavior on color {
                CAnim {}
            }

            Behavior on width {
                Anim {
                    type: Anim.StandardSmall
                }
            }

            Behavior on opacity {
                Anim {
                    type: Anim.StandardSmall
                }
            }
        }

        IconButton {
            id: commandsBtn

            isToggle: true
            type: IconButton.Text
            implicitWidth: 32
            implicitHeight: 32
            radius: Tokens.rounding.full
            radiusMorph: false
            icon: "menu"

            anchors.verticalCenter: parent.verticalCenter
            anchors.right: parent.right
            anchors.rightMargin: root.padding
            checked: search.text.startsWith(GlobalConfig.launcher.actionPrefix)

            onClicked: {
                if (search.text.startsWith(GlobalConfig.launcher.actionPrefix)) {
                    search.text = "";
                } else {
                    search.text = GlobalConfig.launcher.actionPrefix;
                }
                search.forceActiveFocus();
                search.cursorPosition = search.text.length;
            }
        }
    }

    Popup {
        id: clearClipboardConfirmPopup

        modal: true
        focus: true
        closePolicy: Popup.CloseOnEscape | Popup.CloseOnPressOutside

        x: Math.round((root.width - width) / 2)
        y: Math.round((root.height - height) / 2)

        padding: Tokens.padding.large

        contentItem: Column {
            spacing: Tokens.spacing.medium

            StyledText {
                text: qsTr("Clear clipboard history?")
                font: Tokens.font.body.builders.large.weight(Font.Medium).build()
            }

            StyledText {
                text: qsTr("This removes all clipboard entries.")
                color: Colours.palette.m3onSurfaceVariant
                font: Tokens.font.body.small
            }

            Row {
                spacing: Tokens.spacing.small

                TextButton {
                    text: qsTr("Cancel")
                    onClicked: clearClipboardConfirmPopup.close()
                }

                TextButton {
                    text: qsTr("Clear")
                    type: TextButton.Filled
                    onClicked: {
                        clearClipboardConfirmPopup.close();
                        root.clearClipboardHistory();
                    }
                }
            }
        }

        background: StyledRect {
            radius: Tokens.rounding.large
            color: Colours.palette.m3surfaceContainer
        }
    }
}
