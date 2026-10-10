import QtQuick
import QtQuick.Layouts
import stage.modules.common
import stage.modules.common.widgets
import stage.services
import shell.services as ShellServices

StyledFlickable {
    id: root

    contentHeight: column.implicitHeight
    clip: true

    readonly property string design: String(ShellServices.Dock.cfg("design", "ryoku"))
    function cfg(key, fallback) { return ShellServices.Dock.cfg(key, fallback) }
    function setCfg(key, value) { ShellServices.Dock.setCfg(key, value) }
    function designCfg(key, fallback) { return ShellServices.Dock.designCfg(root.design, key, fallback) }
    function setDesignCfg(key, value) { ShellServices.Dock.setDesignCfg(root.design, key, value) }

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
            symbol: "visibility_off"
            title: Translation.tr("Automatically hide")
            trailingKind: "switch"
            switchChecked: root.cfg("autohide", true)
            onActivated: root.setCfg("autohide", !root.cfg("autohide", true))
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

        Item {
            Layout.fillWidth: true
            implicitHeight: 8
        }
    }
}
