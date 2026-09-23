import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons

Grid {
    id: root

    required property var config
    signal changed(string key, var value)

    columns: 2
    columnSpacing: Tokens.s2
    rowSpacing: Tokens.s2

    component SliderCell: Cell {
        id: cell

        property real minimum: 0
        property real maximum: 100
        property real setting: 0
        property string key: ""
        readonly property bool ratio: ["opacity", "islandScale", "osdScale"].includes(cell.key)

        width: (root.width - Tokens.s2) / 2
        height: implicitHeight
        controlWidth: 160
        value: String(cell.ratio ? Math.round(cell.setting * 100) : cell.setting)
        unit: cell.ratio ? "%" : "px"
        source: "shell.json"

        Slid {
            width: parent.width
            from: cell.minimum
            to: cell.maximum
            value: cell.setting
            onModified: value => root.changed(cell.key, value)
        }
    }

    SliderCell {
        label: I18n.tr("Bar height")
        key: "height"
        minimum: 32
        maximum: 56
        setting: root.config.height
    }
    SliderCell {
        label: I18n.tr("Island opacity")
        key: "opacity"
        minimum: 0.45
        maximum: 1
        setting: root.config.opacity
    }
    SliderCell {
        label: I18n.tr("Island padding")
        key: "padding"
        minimum: 6
        maximum: 24
        setting: root.config.padding
    }
    SliderCell {
        label: I18n.tr("Widget spacing")
        key: "spacing"
        minimum: 2
        maximum: 18
        setting: root.config.spacing
    }
    SliderCell {
        label: I18n.tr("Island gap")
        key: "islandGap"
        minimum: 6
        maximum: 32
        setting: root.config.islandGap
    }
    SliderCell {
        objectName: "nacre-frame-size"
        label: I18n.tr("Frame size")
        key: "frameSize"
        minimum: 2
        maximum: 24
        setting: root.config.frameSize
    }
    SliderCell {
        objectName: "nacre-frame-roundness"
        label: I18n.tr("Frame roundness")
        key: "frameRoundness"
        minimum: 0
        maximum: 32
        setting: root.config.frameRoundness
    }
    SliderCell {
        objectName: "nacre-edge-melt"
        label: I18n.tr("Edge melt")
        key: "edgeMelt"
        minimum: 1
        maximum: 32
        setting: root.config.edgeMelt
    }
    SliderCell {
        objectName: "nacre-island-size"
        label: I18n.tr("Island size")
        key: "islandScale"
        minimum: 0.65
        maximum: 1.25
        setting: root.config.islandScale
    }
    SliderCell {
        objectName: "nacre-osd-size"
        label: I18n.tr("OSD / popup size")
        key: "osdScale"
        minimum: 0.65
        maximum: 1.25
        setting: root.config.osdScale
    }
    Cell {
        width: (root.width - Tokens.s2) / 2
        height: implicitHeight
        controlWidth: 54
        label: I18n.tr("Desktop frame")
        value: root.config.frame ? I18n.tr("ON") : I18n.tr("OFF")
        source: "shell.json"

        Sw {
            objectName: "nacre-frame"
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            on: root.config.frame
            onToggled: value => root.changed("frame", value)
        }
    }
    Cell {
        width: (root.width - Tokens.s2) / 2
        height: implicitHeight
        controlWidth: 54
        label: I18n.tr("Occupied workspaces")
        value: root.config.occupiedWorkspaces ? I18n.tr("ON") : I18n.tr("OFF")
        source: "shell.json"

        Sw {
            objectName: "nacre-occupied-workspaces"
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            on: root.config.occupiedWorkspaces
            onToggled: value => root.changed("occupiedWorkspaces", value)
        }
    }
    Cell {
        width: (root.width - Tokens.s2) / 2
        height: implicitHeight
        controlWidth: 174
        label: I18n.tr("Workspace style")
        value: I18n.tr(root.config.workspaceStyle.toUpperCase())
        source: "shell.json"

        Seg {
            objectName: "nacre-workspace-style"
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            options: ["DOTS", "NUMBERS", "KANJI"]
            current: root.config.workspaceStyle.toUpperCase()
            onChose: key => root.changed("workspaceStyle", key.toLowerCase())
        }
    }
    Cell {
        width: (root.width - Tokens.s2) / 2
        height: implicitHeight
        controlWidth: 174
        label: I18n.tr("Brand click opens")
        value: root.config.brandClick === "quicksettings"
            ? I18n.tr("Quick Settings") : I18n.tr("Launcher")
        source: "shell.json"

        Seg {
            objectName: "nacre-brand-click"
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            options: ["LAUNCHER", "QUICK SETTINGS"]
            current: root.config.brandClick === "quicksettings" ? "QUICK SETTINGS" : "LAUNCHER"
            onChose: key => root.changed("brandClick", key === "QUICK SETTINGS" ? "quicksettings" : "launcher")
        }
    }
}
