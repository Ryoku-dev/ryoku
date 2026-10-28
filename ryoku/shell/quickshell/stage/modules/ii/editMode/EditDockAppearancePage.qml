import QtQuick
import QtQuick.Layouts
import Quickshell
import stage.modules.common
import stage.modules.common.widgets
import stage.services
import shell.services as ShellServices

StyledFlickable {
    id: root

    contentHeight: column.implicitHeight
    clip: true

    // A deeper page of this one (the ii design's widgets & buttons).
    signal pageRequested(string page)

    readonly property string design: String(ShellServices.Dock.cfg("design", "ryoku"))
    function cfg(key, fallback) { return ShellServices.Dock.cfg(key, fallback) }
    function setCfg(key, value) { ShellServices.Dock.setCfg(key, value) }
    function designCfg(key, fallback) { return ShellServices.Dock.designCfg(root.design, key, fallback) }
    function setDesignCfg(key, value) { ShellServices.Dock.setDesignCfg(root.design, key, value) }
    function stepped(key, fallback, delta, min, max) {
        const next = Math.round((Number(root.designCfg(key, fallback)) + delta) * 100) / 100;
        root.setDesignCfg(key, Math.max(min, Math.min(max, next)));
    }

    // The ii design's saved layouts and folders live in its own services,
    // published by its host while the design is up.
    readonly property var iiActions: root.design === "ii" ? ShellServices.Dock.designActions : null
    readonly property string iiHover: root.designCfg("enablePreview", true) ? "preview"
        : root.designCfg("enableAppTooltip", false) ? "name" : "none"
    readonly property var iiShapes: ["Circle", "Square", "Slanted", "Arch", "Arrow", "SemiCircle", "Oval", "Pill",
        "Triangle", "Diamond", "ClamShell", "Pentagon", "Gem", "Sunny", "VerySunny", "Cookie4Sided", "Cookie6Sided",
        "Cookie7Sided", "Cookie9Sided", "Cookie12Sided", "Ghostish", "Clover4Leaf", "Clover8Leaf", "Burst",
        "SoftBurst", "Flower", "Puffy", "PuffyDiamond", "PixelCircle", "Bun", "Heart"]
    readonly property var iiAnimations: [
        { displayName: Translation.tr("Still"), icon: "block", value: "none" },
        { displayName: Translation.tr("Bounce"), icon: "sports_basketball", value: "bounce" },
        { displayName: Translation.tr("Hop"), icon: "north", value: "hop" },
        { displayName: Translation.tr("Pulse"), icon: "radio_button_checked", value: "pulse" },
        { displayName: Translation.tr("Wiggle"), icon: "vibration", value: "wiggle" },
        { displayName: Translation.tr("Ripple"), icon: "waves", value: "ripple" }
    ]
    readonly property int iiOnDock: {
        let n = (root.iiActions?.utilities ?? []).length;
        for (const key of ["enableMediaWidget", "enableWeatherWidget", "enableSportsWidget", "enableTasksWidget",
                "enableLivePreviewWidget", "showPhoneButton", "showOverviewButton", "showPinButton", "showTrashButton"]) {
            const fallback = key === "showOverviewButton" || key === "showPinButton";
            if (root.designCfg(key, fallback))
                n++;
        }
        return n;
    }

    ColumnLayout {
        id: column
        width: root.width
        spacing: 4

        EditDockDesignPicker {
            currentValue: root.design
            onSelected: value => root.setCfg("design", value)
        }

        EditPanelSectionLabel { text: Translation.tr("Shared placement") }

        EditPanelRow {
            Layout.fillWidth: true
            first: true
            last: false
            symbol: "dock"
            title: Translation.tr("Show dock")
            subtitle: Translation.tr("Uses this design on every monitor")
            trailingKind: "switch"
            switchChecked: root.cfg("enabled", false)
            rowEnabled: root.design !== "none"
            onActivated: root.setCfg("enabled", !root.cfg("enabled", false))
        }

        EditOptionChips {
            Layout.fillWidth: true
            label: Translation.tr("Screen edge")
            currentValue: root.cfg("edge", "auto")
            options: [
                { displayName: Translation.tr("Auto"), icon: "auto_awesome", value: "auto" },
                { displayName: Translation.tr("Bottom"), icon: "border_bottom", value: "bottom" },
                { displayName: Translation.tr("Top"), icon: "border_top", value: "top" },
                { displayName: Translation.tr("Left"), icon: "border_left", value: "left" },
                { displayName: Translation.tr("Right"), icon: "border_right", value: "right" }
            ]
            onSelected: value => root.setCfg("edge", value)
        }

        EditPanelRow {
            Layout.fillWidth: true
            first: false
            last: false
            symbol: "aspect_ratio"
            title: Translation.tr("Icon size")
            trailingKind: "stepper"
            valueText: Math.round(Number(root.cfg("size", 44))) + " px"
            stepDownEnabled: Number(root.cfg("size", 44)) > 28
            stepUpEnabled: Number(root.cfg("size", 44)) < 72
            onStepDown: root.setCfg("size", Math.max(28, Number(root.cfg("size", 44)) - 2))
            onStepUp: root.setCfg("size", Math.min(72, Number(root.cfg("size", 44)) + 2))
        }

        EditPanelRow {
            Layout.fillWidth: true
            first: false
            last: false
            symbol: "vertical_align_center"
            title: Translation.tr("Edge gap")
            subtitle: Translation.tr("Extra space from the screen edge")
            trailingKind: "stepper"
            valueText: Math.round(Number(root.cfg("edgeGap", 0))) + " px"
            stepDownEnabled: Number(root.cfg("edgeGap", 0)) > 0
            stepUpEnabled: Number(root.cfg("edgeGap", 0)) < 64
            onStepDown: root.setCfg("edgeGap", Math.max(0, Number(root.cfg("edgeGap", 0)) - 1))
            onStepUp: root.setCfg("edgeGap", Math.min(64, Number(root.cfg("edgeGap", 0)) + 1))
        }

        EditPanelRow {
            Layout.fillWidth: true
            first: false
            last: false
            symbol: "visibility_off"
            title: Translation.tr("Automatically hide")
            trailingKind: "switch"
            switchChecked: root.cfg("autohide", false)
            onActivated: root.setCfg("autohide", !root.cfg("autohide", false))
        }

        EditPanelRow {
            visible: root.design === "ryoku"
            Layout.fillWidth: true
            first: false
            last: false
            symbol: "desktop_windows"
            title: Translation.tr("Stay visible on empty workspaces")
            trailingKind: "switch"
            switchChecked: root.cfg("revealOnEmpty", true)
            onActivated: root.setCfg("revealOnEmpty", !root.cfg("revealOnEmpty", true))
        }

        EditPanelRow {
            Layout.fillWidth: true
            first: false
            last: true
            symbol: "zoom_out_map"
            title: Translation.tr("Magnify on hover")
            trailingKind: "switch"
            switchChecked: root.cfg("magnify", true)
            onActivated: root.setCfg("magnify", !root.cfg("magnify", true))
        }

        EditPanelSectionLabel {
            visible: root.design === "ryoku"
            text: Translation.tr("Ryoku design")
        }

        EditOptionChips {
            visible: root.design === "ryoku"
            Layout.fillWidth: true
            label: Translation.tr("Shape")
            currentValue: root.cfg("style", "islands")
            options: ShellServices.Dock.styleOptions.map(option => ({
                displayName: option.label, value: option.key, icon: option.key === "rail" ? "view_week" : "dock"
            }))
            onSelected: value => root.setCfg("style", value)
        }

        Repeater {
            model: root.design === "ryoku" ? [
                { key: "frost", title: Translation.tr("Frosted surface"), symbol: "blur_on", fallback: true },
                { key: "shadow", title: Translation.tr("Depth shadow"), symbol: "layers", fallback: true },
                { key: "labels", title: Translation.tr("Hover labels"), symbol: "label", fallback: true },
                { key: "media", title: Translation.tr("Now playing card"), symbol: "music_note", fallback: false }
            ] : []
            delegate: EditPanelRow {
                required property var modelData
                required property int index
                Layout.fillWidth: true
                first: index === 0
                last: index === 3
                symbol: modelData.symbol
                title: modelData.title
                trailingKind: "switch"
                switchChecked: root.cfg(modelData.key, modelData.fallback)
                onActivated: root.setCfg(modelData.key, !root.cfg(modelData.key, modelData.fallback))
            }
        }

        EditPanelSectionLabel {
            visible: root.design === "python"
            text: Translation.tr("Python design")
        }

        Repeater {
            model: root.design === "python" ? [
                { key: "floating", title: Translation.tr("Float from the edge"), symbol: "space_bar", fallback: false },
                { key: "onTop", title: Translation.tr("Stay above windows"), symbol: "vertical_align_top", fallback: true },
                { key: "exclusive", title: Translation.tr("Reserve room for windows"), symbol: "crop_free", fallback: false },
                { key: "smartAutohide", title: Translation.tr("Hide only when a workspace is busy"), symbol: "auto_awesome", fallback: true },
                { key: "cascadeScale", title: Translation.tr("Magnify neighbouring icons"), symbol: "filter_center_focus", fallback: false },
                { key: "enableScrolling", title: Translation.tr("Scroll long docks"), symbol: "swipe", fallback: false },
                { key: "overrideBoundsCorrection", title: Translation.tr("Allow edge overflow"), symbol: "open_in_full", fallback: false }
            ] : []
            delegate: EditPanelRow {
                required property var modelData
                required property int index
                Layout.fillWidth: true
                first: index === 0
                last: index === 6
                symbol: modelData.symbol
                title: modelData.title
                trailingKind: "switch"
                switchChecked: root.designCfg(modelData.key, modelData.fallback)
                onActivated: root.setDesignCfg(modelData.key, !root.designCfg(modelData.key, modelData.fallback))
            }
        }

        Repeater {
            model: root.design === "python" ? [
                { key: "opacity", title: Translation.tr("Surface opacity"), symbol: "opacity", fallback: 100, min: 20, max: 100, step: 5, unit: " %" },
                { key: "hoverScale", title: Translation.tr("Hover scale"), symbol: "zoom_in", fallback: 120, min: 100, max: 180, step: 5, unit: " %" },
                { key: "autohideTimeout", title: Translation.tr("Hide delay"), symbol: "timer", fallback: 1000, min: 250, max: 5000, step: 250, unit: " ms" },
                { key: "visibleElements", title: Translation.tr("Visible icons while scrolling"), symbol: "apps", fallback: 7, min: 3, max: 15, step: 1, unit: "" }
            ] : []
            delegate: EditPanelRow {
                required property var modelData
                required property int index
                Layout.fillWidth: true
                first: false
                last: index === 3
                symbol: modelData.symbol
                title: modelData.title
                trailingKind: "stepper"
                valueText: Math.round(Number(root.designCfg(modelData.key, modelData.fallback))) + modelData.unit
                stepDownEnabled: Number(root.designCfg(modelData.key, modelData.fallback)) > modelData.min
                stepUpEnabled: Number(root.designCfg(modelData.key, modelData.fallback)) < modelData.max
                onStepDown: root.setDesignCfg(modelData.key, Math.max(modelData.min,
                    Number(root.designCfg(modelData.key, modelData.fallback)) - modelData.step))
                onStepUp: root.setDesignCfg(modelData.key, Math.min(modelData.max,
                    Number(root.designCfg(modelData.key, modelData.fallback)) + modelData.step))
            }
        }

        EditPanelSectionLabel {
            visible: root.design === "shima"
            text: Translation.tr("Shima design")
        }

        EditOptionChips {
            visible: root.design === "shima"
            Layout.fillWidth: true
            label: Translation.tr("Shape")
            currentValue: root.designCfg("shape", "auto")
            options: [
                { displayName: Translation.tr("Auto"), icon: "auto_awesome", value: "auto" },
                { displayName: Translation.tr("Round"), icon: "circle", value: "round" },
                { displayName: Translation.tr("Squircle"), icon: "rounded_corner", value: "squircle" },
                { displayName: Translation.tr("Square"), icon: "square", value: "square" }
            ]
            onSelected: value => root.setDesignCfg("shape", value)
        }

        EditOptionChips {
            visible: root.design === "shima"
            Layout.fillWidth: true
            label: Translation.tr("Material")
            currentValue: root.designCfg("material", "inherit")
            options: [
                { displayName: Translation.tr("Shima"), icon: "auto_awesome", value: "inherit" },
                { displayName: Translation.tr("Solid"), icon: "square", value: "solid" },
                { displayName: Translation.tr("Glass"), icon: "blur_on", value: "glass" }
            ]
            onSelected: value => root.setDesignCfg("material", value)
        }

        Repeater {
            model: root.design === "shima" ? [
                { key: "notch", title: Translation.tr("Melt into the screen edge"), symbol: "line_curve", fallback: true },
                { key: "launcher", title: Translation.tr("Applications button"), symbol: "apps", fallback: true },
                { key: "reserveSpace", title: Translation.tr("Reserve room for windows"), symbol: "crop_free", fallback: true },
                { key: "revealOnEmpty", title: Translation.tr("Stay visible on empty workspaces"), symbol: "desktop_windows", fallback: true },
                { key: "badges", title: Translation.tr("Notification badges"), symbol: "notifications", fallback: true }
            ] : []
            delegate: EditPanelRow {
                required property var modelData
                required property int index
                Layout.fillWidth: true
                first: index === 0
                last: index === 4
                symbol: modelData.symbol
                title: modelData.title
                trailingKind: "switch"
                switchChecked: root.designCfg(modelData.key, modelData.fallback)
                onActivated: root.setDesignCfg(modelData.key, !root.designCfg(modelData.key, modelData.fallback))
            }
        }

        EditPanelRow {
            visible: root.design === "shima" && root.cfg("magnify", true)
            Layout.fillWidth: true
            first: false
            last: true
            symbol: "zoom_in"
            title: Translation.tr("Magnification")
            trailingKind: "stepper"
            valueText: Math.round(Number(root.designCfg("magnifySize", 150))) + " %"
            stepDownEnabled: Number(root.designCfg("magnifySize", 150)) > 110
            stepUpEnabled: Number(root.designCfg("magnifySize", 150)) < 200
            onStepDown: root.setDesignCfg("magnifySize", Math.max(110, Number(root.designCfg("magnifySize", 150)) - 5))
            onStepUp: root.setDesignCfg("magnifySize", Math.min(200, Number(root.designCfg("magnifySize", 150)) + 5))
        }

        EditPanelSectionLabel {
            visible: root.design === "ii"
            text: Translation.tr("ii design")
        }

        EditIiDockStylePicker {
            visible: root.design === "ii"
            currentValue: root.designCfg("dockStyle", "floating")
            onSelected: value => {
                root.setDesignCfg("dockStyle", value);
                root.setDesignCfg("islandsStyle", value === "islands");
            }
        }

        EditPanelRow {
            visible: root.design === "ii"
            Layout.fillWidth: true
            Layout.topMargin: Appearance.sizes.space2
            first: true
            last: false
            symbol: "rounded_corner"
            title: Translation.tr("Corner radius")
            trailingKind: "stepper"
            readonly property real radiusValue: Number(root.designCfg("dockRadius", -1))
            valueText: radiusValue < 0 ? Translation.tr("Auto") : Math.round(radiusValue) + " px"
            stepDownEnabled: radiusValue >= 0
            stepUpEnabled: radiusValue < 40
            onStepDown: root.setDesignCfg("dockRadius", radiusValue <= 0 ? -1 : Math.max(0, radiusValue - 2))
            onStepUp: root.setDesignCfg("dockRadius", radiusValue < 0 ? 0 : Math.min(40, radiusValue + 2))
        }

        EditPanelRow {
            visible: root.design === "ii" && root.designCfg("dockStyle", "floating") === "islands"
            Layout.fillWidth: true
            first: false
            last: false
            symbol: "space_bar"
            title: Translation.tr("Island spacing")
            trailingKind: "stepper"
            valueText: Math.round(Number(root.designCfg("islandSpacing", 8))) + " px"
            stepDownEnabled: Number(root.designCfg("islandSpacing", 8)) > 4
            stepUpEnabled: Number(root.designCfg("islandSpacing", 8)) < 32
            onStepDown: root.stepped("islandSpacing", 8, -2, 4, 32)
            onStepUp: root.stepped("islandSpacing", 8, 2, 4, 32)
        }

        EditPanelRow {
            visible: root.design === "ii"
            Layout.fillWidth: true
            first: false
            last: false
            symbol: "widgets"
            title: Translation.tr("Widgets & buttons")
            subtitle: root.iiOnDock === 1 ? Translation.tr("1 item on the dock besides apps")
                : Translation.tr("%1 items on the dock besides apps").arg(root.iiOnDock)
            trailingKind: "chevron"
            onActivated: root.pageRequested("iiWidgets")
        }

        EditOptionChips {
            visible: root.design === "ii"
            Layout.fillWidth: true
            Layout.topMargin: Appearance.sizes.space2
            label: Translation.tr("Over an icon")
            compact: false
            currentValue: root.iiHover
            options: [
                { displayName: Translation.tr("Nothing"), icon: "do_not_disturb_on", value: "none" },
                { displayName: Translation.tr("App name"), icon: "subtitles", value: "name" },
                { displayName: Translation.tr("Window preview"), icon: "preview", value: "preview" }
            ]
            onSelected: value => {
                root.setDesignCfg("enablePreview", value === "preview");
                root.setDesignCfg("enableAppTooltip", value === "name");
            }
        }

        EditPanelSectionLabel {
            visible: root.design === "ii" && root.cfg("magnify", true)
            text: Translation.tr("Magnification")
        }

        EditPanelRow {
            visible: root.design === "ii" && root.cfg("magnify", true)
            Layout.fillWidth: true
            first: true
            last: false
            symbol: "zoom_in"
            title: Translation.tr("Strength")
            subtitle: Translation.tr("How large an icon grows under the pointer")
            trailingKind: "stepper"
            readonly property real scaleValue: Number(root.designCfg("magnificationScale", 1.5))
            valueText: Math.round(scaleValue * 100) + " %"
            stepDownEnabled: scaleValue > 1.2
            stepUpEnabled: scaleValue < 2.2
            onStepDown: root.stepped("magnificationScale", 1.5, -0.05, 1.2, 2.2)
            onStepUp: root.stepped("magnificationScale", 1.5, 0.05, 1.2, 2.2)
        }

        EditPanelRow {
            visible: root.design === "ii" && root.cfg("magnify", true)
            Layout.fillWidth: true
            first: false
            last: false
            symbol: "blur_circular"
            title: Translation.tr("Reach")
            subtitle: Translation.tr("How many neighbours swell with it")
            trailingKind: "stepper"
            readonly property real reachValue: Number(root.designCfg("magnificationInfluenceRadius", 2.35))
            valueText: reachValue.toFixed(1)
            stepDownEnabled: reachValue > 1.2
            stepUpEnabled: reachValue < 4
            onStepDown: root.stepped("magnificationInfluenceRadius", 2.35, -0.1, 1.2, 4)
            onStepUp: root.stepped("magnificationInfluenceRadius", 2.35, 0.1, 1.2, 4)
        }

        EditPanelRow {
            visible: root.design === "ii" && root.cfg("magnify", true)
            Layout.fillWidth: true
            first: false
            last: true
            symbol: "open_in_full"
            title: Translation.tr("Make room")
            subtitle: Translation.tr("Neighbours move aside instead of overlapping")
            trailingKind: "switch"
            switchChecked: root.designCfg("magnificationDynamicSpacing", true)
            onActivated: root.setDesignCfg("magnificationDynamicSpacing", !root.designCfg("magnificationDynamicSpacing", true))
        }

        EditOptionChips {
            visible: root.design === "ii" && root.cfg("magnify", true)
            Layout.fillWidth: true
            label: Translation.tr("Motion")
            compact: false
            currentValue: root.designCfg("magnificationMotion", "balanced")
            options: [
                { displayName: Translation.tr("Fast"), icon: "fast_forward", value: "fast" },
                { displayName: Translation.tr("Balanced"), icon: "animation", value: "balanced" },
                { displayName: Translation.tr("Smooth"), icon: "slow_motion_video", value: "smooth" }
            ]
            onSelected: value => root.setDesignCfg("magnificationMotion", value)
        }

        EditPanelSectionLabel {
            visible: root.design === "ii"
            text: Translation.tr("Icons & motion")
        }

        EditOptionChips {
            visible: root.design === "ii"
            Layout.fillWidth: true
            label: Translation.tr("Icon colour")
            compact: false
            currentValue: root.designCfg("monochromeIcons", false) ? "tinted"
                : root.designCfg("dimInactiveIcons", false) ? "dimmed" : "full"
            options: [
                { displayName: Translation.tr("Full colour"), icon: "palette", value: "full" },
                { displayName: Translation.tr("Dim idle apps"), icon: "tonality", value: "dimmed" },
                { displayName: Translation.tr("Tinted"), icon: "format_color_fill", value: "tinted" }
            ]
            onSelected: value => {
                root.setDesignCfg("monochromeIcons", value === "tinted");
                root.setDesignCfg("dimInactiveIcons", value === "dimmed");
            }
        }

        EditPanelRow {
            visible: root.design === "ii"
            Layout.fillWidth: true
            Layout.topMargin: Appearance.sizes.space2
            first: true
            last: false
            symbol: "interests"
            title: Translation.tr("Adaptive icons")
            subtitle: Translation.tr("Crop every icon to one shape")
            trailingKind: "switch"
            switchChecked: root.designCfg("enableShapeMask", true)
            onActivated: root.setDesignCfg("enableShapeMask", !root.designCfg("enableShapeMask", true))
        }

        Flow {
            visible: root.design === "ii" && root.designCfg("enableShapeMask", true)
            Layout.fillWidth: true
            Layout.leftMargin: Appearance.sizes.space4
            Layout.rightMargin: Appearance.sizes.space4
            Layout.topMargin: Appearance.sizes.space1
            Layout.bottomMargin: Appearance.sizes.space2
            spacing: Appearance.sizes.space1

            Repeater {
                model: root.iiShapes
                delegate: Rectangle {
                    id: shapeChip
                    required property string modelData
                    readonly property bool current: root.designCfg("shapeMask", "Circle") === modelData
                    width: 34
                    height: 34
                    radius: Appearance.rounding.small
                    color: current ? Appearance.colors.colSecondaryContainer
                        : shapeMouse.containsMouse ? Appearance.colors.colLayer1Hover : "transparent"
                    border.width: 1
                    border.color: current ? Appearance.colors.colSecondary : Appearance.colors.colOutlineVariant

                    MaterialShape {
                        anchors.centerIn: parent
                        implicitSize: 20
                        width: 20
                        height: 20
                        shapeString: shapeChip.modelData
                        color: shapeChip.current ? Appearance.colors.colOnSecondaryContainer
                            : Appearance.colors.colOnSurfaceVariant
                    }

                    MouseArea {
                        id: shapeMouse
                        anchors.fill: parent
                        hoverEnabled: true
                        cursorShape: Qt.PointingHandCursor
                        onClicked: root.setDesignCfg("shapeMask", shapeChip.modelData)
                    }
                }
            }
        }

        EditPanelRow {
            visible: root.design === "ii"
            Layout.fillWidth: true
            first: false
            last: false
            symbol: "space_bar"
            title: Translation.tr("Icon spacing")
            trailingKind: "stepper"
            valueText: Math.round(Number(root.designCfg("iconSpacing", -1))) + " px"
            stepDownEnabled: Number(root.designCfg("iconSpacing", -1)) > -4
            stepUpEnabled: Number(root.designCfg("iconSpacing", -1)) < 16
            onStepDown: root.stepped("iconSpacing", -1, -1, -4, 16)
            onStepUp: root.stepped("iconSpacing", -1, 1, -4, 16)
        }

        EditPanelRow {
            visible: root.design === "ii"
            Layout.fillWidth: true
            first: false
            last: true
            symbol: "crop_square"
            title: Translation.tr("Widget corners")
            trailingKind: "stepper"
            readonly property real widgetRadiusValue: Number(root.designCfg("widgetRadius", -1))
            valueText: widgetRadiusValue < 0 ? Translation.tr("Auto") : Math.round(widgetRadiusValue) + " px"
            stepDownEnabled: widgetRadiusValue >= 0
            stepUpEnabled: widgetRadiusValue < 30
            onStepDown: root.setDesignCfg("widgetRadius", widgetRadiusValue <= 0 ? -1 : Math.max(0, widgetRadiusValue - 2))
            onStepUp: root.setDesignCfg("widgetRadius", widgetRadiusValue < 0 ? 0 : Math.min(30, widgetRadiusValue + 2))
        }

        EditOptionChips {
            visible: root.design === "ii"
            Layout.fillWidth: true
            Layout.topMargin: Appearance.sizes.space2
            label: Translation.tr("When an app opens")
            currentValue: root.designCfg("launchAnimation", "bounce")
            options: root.iiAnimations
            onSelected: value => root.setDesignCfg("launchAnimation", value)
        }

        EditOptionChips {
            visible: root.design === "ii"
            Layout.fillWidth: true
            label: Translation.tr("When an app calls for you")
            currentValue: root.designCfg("notificationAnimation", "bounce")
            options: root.iiAnimations
            onSelected: value => root.setDesignCfg("notificationAnimation", value)
        }

        EditPanelSectionLabel {
            visible: root.design === "ii"
            text: Translation.tr("Behaviour")
        }

        Repeater {
            model: root.design === "ii" ? [
                { key: "hoverToReveal", title: Translation.tr("Reveal at the edge"), subtitle: Translation.tr("A hidden dock comes back when the pointer reaches its edge"), symbol: "mouse", fallback: true },
                { key: "blockHoverInFullscreen", title: Translation.tr("Not over fullscreen"), subtitle: Translation.tr("The dock steps aside while a window or video is fullscreen, even when pinned"), symbol: "fullscreen", fallback: true },
                { key: "smartGrouping", title: Translation.tr("Smart grouping"), subtitle: Translation.tr("Sort apps by category; anything placed by hand stays put"), symbol: "category", fallback: false },
                { key: "enableWidgetStack", title: Translation.tr("Widget stack"), subtitle: Translation.tr("Several widgets share one slot, turned with the wheel"), symbol: "stacks", fallback: false },
                { key: "lockReorder", title: Translation.tr("Lock arrangement"), subtitle: Translation.tr("Nothing on the dock can be dragged out of place"), symbol: "lock", fallback: false },
                { key: "isolateMonitors", title: Translation.tr("Isolate monitors"), subtitle: Translation.tr("Each monitor's dock shows only its own windows"), symbol: "monitor", fallback: false },
                { key: "showOnlyOnFocusedMonitor", title: Translation.tr("Focused monitor only"), subtitle: Translation.tr("On an empty workspace, show the dock on the focused monitor alone"), symbol: "center_focus_strong", fallback: false }
            ] : []
            delegate: EditPanelRow {
                required property var modelData
                required property int index
                Layout.fillWidth: true
                first: index === 0
                last: index === 6
                symbol: modelData.symbol
                title: modelData.title
                subtitle: modelData.subtitle
                trailingKind: "switch"
                switchChecked: root.designCfg(modelData.key, modelData.fallback)
                onActivated: root.setDesignCfg(modelData.key, !root.designCfg(modelData.key, modelData.fallback))
            }
        }

        EditOptionChips {
            visible: root.design === "ii"
            Layout.fillWidth: true
            Layout.topMargin: Appearance.sizes.space2
            label: Translation.tr("Show on")
            compact: false
            currentValue: {
                const wanted = String(root.designCfg("monitor", ""));
                return Quickshell.screens.some(screen => screen.name === wanted) ? wanted : "";
            }
            options: [{ displayName: Translation.tr("All monitors"), icon: "select_all", value: "" }]
                .concat(Quickshell.screens.map(screen => ({ displayName: screen.name, icon: "monitor", value: screen.name })))
            onSelected: value => root.setDesignCfg("monitor", value)
        }

        EditPanelSectionLabel {
            visible: root.iiActions !== null
            text: Translation.tr("Saved layouts")
        }

        EditPanelNotice {
            visible: root.iiActions !== null && (root.iiActions?.presets ?? []).length === 0
            Layout.fillWidth: true
            symbol: "bookmarks"
            text: Translation.tr("Save the dock's apps, widgets and buttons under a name to switch back to them later.")
        }

        Repeater {
            model: root.iiActions?.presets ?? []
            delegate: EditPanelRow {
                required property var modelData
                required property int index
                Layout.fillWidth: true
                first: index === 0
                last: index === (root.iiActions?.presets ?? []).length - 1
                symbol: "bookmark"
                title: modelData.name ?? ""
                subtitle: index === root.iiActions?.activePreset ? Translation.tr("On the dock now") : Translation.tr("Tap to put it on the dock")
                selected: index === root.iiActions?.activePreset
                trailingKind: index === root.iiActions?.activePreset ? "check" : "none"
                onActivated: root.iiActions?.applyPreset(index)
            }
        }

        EditPanelRow {
            visible: root.iiActions !== null && (root.iiActions?.activePreset ?? -1) >= 0
            Layout.fillWidth: true
            first: true
            last: true
            destructive: true
            symbol: "bookmark_remove"
            title: Translation.tr("Forget this layout")
            subtitle: (root.iiActions?.presets ?? [])[root.iiActions?.activePreset ?? -1]?.name ?? ""
            trailingKind: "none"
            onActivated: root.iiActions?.deletePreset(root.iiActions.activePreset)
        }

        EditPanelRow {
            visible: root.iiActions !== null && (root.iiActions?.presets ?? []).length > 1
            Layout.fillWidth: true
            Layout.topMargin: Appearance.sizes.space2
            first: true
            last: true
            symbol: "swipe_vertical"
            title: Translation.tr("Switch layouts on scroll")
            subtitle: Translation.tr("The wheel over the dock turns between saved layouts")
            trailingKind: "switch"
            switchChecked: root.designCfg("switchPresetsOnScroll", true)
            onActivated: root.setDesignCfg("switchPresetsOnScroll", !root.designCfg("switchPresetsOnScroll", true))
        }

        RowLayout {
            visible: root.iiActions !== null
            Layout.fillWidth: true
            Layout.topMargin: Appearance.sizes.space2
            Layout.leftMargin: Appearance.sizes.space1
            Layout.rightMargin: Appearance.sizes.space1
            spacing: Appearance.sizes.space2

            ToolbarTextField {
                id: presetName
                Layout.fillWidth: true
                implicitHeight: 40
                leftPadding: 40
                colBackground: Appearance.colors.colLayer1
                placeholderText: Translation.tr("Name this layout")
                onAccepted: saveLayout.save()

                MaterialSymbol {
                    anchors.left: parent.left
                    anchors.leftMargin: 12
                    anchors.verticalCenter: parent.verticalCenter
                    text: "bookmark_add"
                    iconSize: 20
                    color: Appearance.colors.colPrimary
                }
            }

            Rectangle {
                id: saveLayout
                function save() {
                    const name = presetName.text.trim();
                    if (name.length === 0)
                        return;
                    root.iiActions?.savePreset(name);
                    presetName.text = "";
                }
                readonly property bool ready: presetName.text.trim().length > 0
                implicitWidth: saveLabel.implicitWidth + Appearance.sizes.space4 * 2
                implicitHeight: 40
                radius: Appearance.rounding.small
                opacity: ready ? 1 : 0.5
                color: saveMouse.containsMouse && ready ? Appearance.colors.colSecondaryHover : Appearance.colors.colSecondary

                StyledText {
                    id: saveLabel
                    anchors.centerIn: parent
                    text: Translation.tr("Save")
                    font.weight: Font.DemiBold
                    color: Appearance.colors.colOnSecondary
                }

                MouseArea {
                    id: saveMouse
                    anchors.fill: parent
                    enabled: saveLayout.ready
                    hoverEnabled: true
                    cursorShape: Qt.PointingHandCursor
                    onClicked: saveLayout.save()
                }
            }
        }

        Item {
            Layout.fillWidth: true
            implicitHeight: 8
        }
    }
}
