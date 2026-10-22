import QtQuick
import QtQuick.Layouts
import Caelestia
import Caelestia.Config
import caelestia.components
import caelestia.components.controls
import caelestia.services

Item {
    id: root

    required property Props props
    required property DrawerVisibilities visibilities

    property var popouts
    property var utilities
    property string activeTab: "notifications"

    readonly property bool isBarHorizontal: Config.bar.position === "top" || Config.bar.position === "bottom"
    readonly property bool showPopoutSeparator: isBarHorizontal && root.visibilities.sidebar && popouts && popouts.hasCurrent && popouts.currentName !== "dockhover" && popouts.currentName !== "dockcontext" && popouts.currentName !== "activewindow" && popouts.currentName !== "greeter" && popouts.currentName !== "greetercontext" && popouts.currentName !== "github"
    // Keeps the sidebar loaded after it is closed: the assistant is still replying,
    // or one of the shell's file dialogs is open (clicking into a dialog can close
    // the sidebar, and unloading it would destroy the dialog's owner with it).
    readonly property bool keepLoaded: (aiLoader.item?.busy ?? false) || Visibilities.openDialogs > 0
    readonly property bool aiEnabled: GlobalConfig.ai.enableAiAssistant && (GlobalConfig.ai.enableOllama || GlobalConfig.ai.enableClaudeCode || GlobalConfig.ai.enableClaude || GlobalConfig.ai.enableOpenai || GlobalConfig.ai.enableGemini || GlobalConfig.ai.enableOpenrouter || GlobalConfig.ai.enableOpencode || GlobalConfig.ai.enableOpencodeGo)

    function checkAiTab(): void {
        if (!root.aiEnabled && root.activeTab === "ai") {
            root.activeTab = "notifications";
        }
        if (!GlobalConfig.ai.showNews && root.activeTab === "news") {
            root.activeTab = "notifications";
        }
    }

    Component.onCompleted: {
        if (root.visibilities.sidebar) {
            root.activeTab = Visibilities.initialSidebarTab || Visibilities.sidebarOpenTab();
            Visibilities.initialSidebarTab = "";
        }
        checkAiTab();
    }
    onActiveTabChanged: {
        if (root.visibilities.sidebar)
            Visibilities.lastSidebarTab = activeTab;
    }

    Connections {
        function onEnableAiAssistantChanged(): void { checkAiTab(); }
        function onEnableOllamaChanged(): void { checkAiTab(); }
        function onEnableClaudeCodeChanged(): void { checkAiTab(); }
        function onEnableClaudeChanged(): void { checkAiTab(); }
        function onEnableOpenaiChanged(): void { checkAiTab(); }
        function onEnableGeminiChanged(): void { checkAiTab(); }
        function onEnableOpenrouterChanged(): void { checkAiTab(); }
        function onEnableOpencodeChanged(): void { checkAiTab(); }
        function onEnableOpencodeGoChanged(): void { checkAiTab(); }
        function onShowNewsChanged(): void { checkAiTab(); }

        target: GlobalConfig.ai
    }

    Connections {
        function onSidebarChanged(): void {
            if (root.visibilities.sidebar) {
                root.activeTab = Visibilities.initialSidebarTab || Visibilities.sidebarOpenTab();
                Visibilities.initialSidebarTab = "";
                checkAiTab();
            }
        }

        target: root.visibilities
    }

    GridLayout {
        id: layout

        anchors.fill: parent
        columns: 1
        rowSpacing: Tokens.spacing.medium

        StyledRect {
            Layout.fillWidth: true
            Layout.fillHeight: true
            Layout.row: isBarHorizontal ? 1 : 0

            radius: Tokens.rounding.large
            color: Colours.tPalette.m3surfaceContainerLow

            ColumnLayout {
                anchors.fill: parent
                spacing: 0

                Item {
                    id: headerContainer

                    Layout.fillWidth: true
                    implicitHeight: (!root.aiEnabled && !GlobalConfig.ai.showNews) ? 0 : 64
                    visible: root.aiEnabled || GlobalConfig.ai.showNews
                    clip: true

                    RowLayout {
                        anchors.fill: parent
                        anchors.leftMargin: Tokens.padding.medium
                        anchors.rightMargin: Tokens.padding.medium
                        spacing: 0

                        Repeater {
                            id: tabRepeater

                            model: {
                                var tabs = [
                                    { id: "notifications", label: qsTr("Notifications"), icon: "notifications" }
                                ];
                                if (root.aiEnabled) {
                                    tabs.push({ id: "ai", label: qsTr("AI Assistant"), icon: "smart_toy" });
                                }
                                if (GlobalConfig.ai.showNews) {
                                    tabs.push({ id: "news", label: qsTr("News"), icon: "newspaper" });
                                }
                                return tabs;
                            }

                            delegate: Item {
                                id: tabBtn

                                required property var modelData
                                readonly property bool active: root.activeTab === modelData.id

                                Layout.fillWidth: true
                                Layout.fillHeight: true

                                StateLayer {
                                    id: stateLayer

                                    anchors.fill: parent
                                    anchors.margins: 4
                                    radius: Tokens.rounding.medium
                                    color: Colours.palette.m3onSurface
                                    onClicked: root.activeTab = tabBtn.modelData.id
                                }

                                ColumnLayout {
                                    anchors.centerIn: parent
                                    spacing: Tokens.spacing.extraSmall - 2

                                    MaterialIcon {
                                        Layout.alignment: Qt.AlignHCenter
                                        text: tabBtn.modelData.icon
                                        color: tabBtn.active ? Colours.palette.m3primary : stateLayer.containsMouse ? Colours.palette.m3onSurface : Colours.palette.m3onSurfaceVariant
                                        fontStyle: Tokens.font.icon.small
                                        fill: tabBtn.active ? 1 : 0

                                        Behavior on color { CAnim {} }
                                        Behavior on fill { Anim { type: Anim.DefaultEffects } }
                                    }

                                    StyledText {
                                        Layout.alignment: Qt.AlignHCenter
                                        text: tabBtn.modelData.label
                                        color: tabBtn.active ? Colours.palette.m3primary : stateLayer.containsMouse ? Colours.palette.m3onSurface : Colours.palette.m3onSurfaceVariant
                                        font: Tokens.font.label.medium

                                        Behavior on color { CAnim {} }
                                    }
                                }
                            }
                        }
                    }

                    Item {
                        id: indicator

                        property int activeIndex: {
                            var arr = tabRepeater.model;
                            for (var i = 0; i < arr.length; i++) {
                                if (arr[i].id === root.activeTab) return i;
                            }
                            return 0;
                        }
                        readonly property real tabWidth: (headerContainer.width - Tokens.padding.medium * 2) / tabRepeater.count

                        anchors.verticalCenter: parent.bottom
                        implicitHeight: 6
                        width: tabWidth - Tokens.padding.medium * 2
                        x: Tokens.padding.medium + activeIndex * tabWidth + (tabWidth - width) / 2
                        clip: true

                        StyledRect {
                            anchors.fill: parent
                            color: Colours.palette.m3primary
                            radius: Tokens.rounding.full
                        }

                        Behavior on x {
                            Anim {}
                        }
                    }

                    // Pin: keep the sidebar open until it is closed explicitly.
                    Item {
                        anchors.top: parent.top
                        anchors.right: parent.right
                        anchors.topMargin: 2
                        anchors.rightMargin: 2
                        z: 1
                        implicitWidth: 26
                        implicitHeight: 26

                        StateLayer {
                            id: pinStateLayer

                            radius: Tokens.rounding.full
                            color: Colours.palette.m3onSurface
                            onClicked: GlobalConfig.sidebar.pinned = !GlobalConfig.sidebar.pinned
                        }

                        MaterialIcon {
                            anchors.centerIn: parent
                            text: "push_pin"
                            rotation: Visibilities.sidebarPinned ? 0 : 45
                            fill: Visibilities.sidebarPinned ? 1 : 0
                            color: Visibilities.sidebarPinned ? Colours.palette.m3primary : pinStateLayer.containsMouse ? Colours.palette.m3onSurface : Colours.palette.m3outline
                            fontStyle: Tokens.font.icon.small

                            Behavior on rotation { Anim { type: Anim.DefaultEffects } }
                            Behavior on fill { Anim { type: Anim.DefaultEffects } }
                            Behavior on color { CAnim {} }
                        }
                    }
                }

                StyledRect {
                    Layout.fillWidth: true
                    implicitHeight: 1
                    visible: root.aiEnabled || GlobalConfig.ai.showNews
                    color: Colours.palette.m3outlineVariant
                }

                Item {
                    property int activeIndex: indicator.activeIndex

                    Layout.fillWidth: true
                    Layout.fillHeight: true
                    clip: true

                    NotifDock {
                        anchors.top: parent.top
                        anchors.bottom: parent.bottom
                        width: parent.width
                        x: root.activeTab === "notifications" ? 0 : -width
                        opacity: root.activeTab === "notifications" ? 1 : 0
                        visible: opacity > 0
                        props: root.props
                        visibilities: root.visibilities

                        Behavior on x { Anim { type: Anim.DefaultSpatial } }
                        Behavior on opacity { Anim { type: Anim.DefaultSpatial } }
                    }

                    Loader {
                        id: aiLoader

                        property bool hasBeenActive: false

                        anchors.top: parent.top
                        anchors.bottom: parent.bottom
                        width: parent.width
                        x: root.activeTab === "ai" ? 0 : (indicator.activeIndex < 1 ? width : -width)
                        opacity: root.activeTab === "ai" ? 1 : 0
                        visible: opacity > 0
                        active: hasBeenActive || root.activeTab === "ai"
                        sourceComponent: AiAssistant {
                            anchors.fill: parent
                        }
                        onActiveChanged: if (active) hasBeenActive = true

                        Behavior on x { Anim { type: Anim.DefaultSpatial } }
                        Behavior on opacity { Anim { type: Anim.DefaultSpatial } }
                    }

                    News {
                        anchors.top: parent.top
                        anchors.bottom: parent.bottom
                        width: parent.width
                        x: root.activeTab === "news" ? 0 : width
                        opacity: root.activeTab === "news" ? 1 : 0
                        visible: opacity > 0

                        Behavior on x { Anim { type: Anim.DefaultSpatial } }
                        Behavior on opacity { Anim { type: Anim.DefaultSpatial } }
                    }
                }
            }
        }

        StyledRect {
            visible: utilities && utilities.offsetScale < 1
            Layout.row: Config.bar.position === "bottom" ? 0 : (Config.bar.position === "top" ? 2 : 1)
            Layout.topMargin: Config.bar.position === "bottom" ? 0 : (visible ? 18 : 0)
            Layout.bottomMargin: Config.bar.position === "bottom" ? (visible ? 18 : 0) : 0
            Layout.fillWidth: true
            implicitHeight: 1

            color: Colours.tPalette.m3outlineVariant
        }

        StyledRect {
            visible: showPopoutSeparator
            Layout.row: Config.bar.position === "bottom" ? 2 : 0
            Layout.topMargin: Config.bar.position === "top" ? 0 : 12
            Layout.bottomMargin: Config.bar.position === "top" ? 12 : 0
            Layout.fillWidth: true
            implicitHeight: 1

            color: Colours.tPalette.m3outlineVariant
        }
    }
}
