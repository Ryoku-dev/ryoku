import QtQuick
import Lithic

Item {
    id: root

    implicitHeight: content.implicitHeight + LithicTokens.space.s5 * 2

    component RoleGroup: Column {
        required property string groupName
        required property var roles
        spacing: LithicTokens.space.s2

        Label {
            text: parent.groupName
            typeRole: "title"
        }
        Row {
            spacing: LithicTokens.space.s2
            Repeater {
                model: parent.parent.roles
                delegate: Column {
                    required property var modelData
                    spacing: LithicTokens.space.s1
                    CutRect {
                        width: 112
                        height: 46
                        fill: modelData.color
                    }
                    Label {
                        width: 112
                        text: modelData.name
                        typeRole: "monoSm"
                        inkRole: "fg2"
                    }
                }
            }
        }
    }

    component StateLabel: Column {
        required property string stateName
        spacing: LithicTokens.space.s2
        default property alias specimen: specimenHost.data
        Item {
            id: specimenHost
            width: 150
            height: 64
        }
        Label {
            width: 150
            text: parent.stateName
            typeRole: "caption"
            inkRole: "fg2"
            horizontalAlignment: Text.AlignHCenter
        }
    }

    Column {
        id: content
        x: LithicTokens.space.s5
        y: LithicTokens.space.s5
        width: root.width - LithicTokens.space.s5 * 2
        spacing: LithicTokens.space.s6

        Column {
            width: parent.width
            spacing: LithicTokens.space.s2
            Label {
                text: "Foundation"
                typeRole: "name"
            }
            Label {
                width: parent.width
                text: "Roles, type, cut geometry, state marks and motion shared by every Lithic component."
                typeRole: "body"
                inkRole: "fg2"
            }
        }

        Column {
            width: parent.width
            spacing: LithicTokens.space.s4
            Label { text: "Roles"; typeRole: "section" }
            RoleGroup {
                groupName: "Planes"
                roles: [
                    { name: "ground", color: Lithic.ground },
                    { name: "stone1", color: Lithic.stone1 },
                    { name: "stone2", color: Lithic.stone2 },
                    { name: "stone3", color: Lithic.stone3 },
                    { name: "stone4", color: Lithic.stone4 },
                    { name: "vein", color: Lithic.vein }
                ]
            }
            RoleGroup {
                groupName: "Ink"
                roles: [
                    { name: "fg", color: Lithic.fg },
                    { name: "fg2", color: Lithic.fg2 },
                    { name: "fg3", color: Lithic.fg3 }
                ]
            }
            RoleGroup {
                groupName: "Selection"
                roles: [
                    { name: "sel", color: Lithic.sel },
                    { name: "onSel", color: Lithic.onSel },
                    { name: "onSel2", color: Lithic.onSel2 }
                ]
            }
            RoleGroup {
                groupName: "Action"
                roles: [
                    { name: "gold", color: Lithic.gold },
                    { name: "onGold", color: Lithic.onGold },
                    { name: "goldInk", color: Lithic.goldInk }
                ]
            }
            RoleGroup {
                groupName: "Minerals"
                roles: [
                    { name: "jade", color: Lithic.jade },
                    { name: "lapis", color: Lithic.lapis },
                    { name: "amber", color: Lithic.amber },
                    { name: "jasper", color: Lithic.jasper }
                ]
            }
            RoleGroup {
                groupName: "Overlay"
                roles: [{ name: "scrim", color: Lithic.scrim }]
            }
        }

        Column {
            width: parent.width
            spacing: LithicTokens.space.s4
            Label { text: "Type"; typeRole: "section" }
            Flow {
                width: parent.width
                spacing: LithicTokens.space.s4
                height: childrenRect.height
                Repeater {
                    model: ["name", "sentence", "section", "title", "row", "body",
                            "proof", "label", "caption", "group", "mono", "monoSm"]
                    delegate: Column {
                        required property string modelData
                        width: modelData === "name" || modelData === "sentence" ? 620 : 280
                        spacing: LithicTokens.space.s1
                        Label {
                            width: parent.width
                            text: parent.modelData
                            typeRole: "monoSm"
                            inkRole: "fg3"
                        }
                        Label {
                            width: parent.width
                            text: parent.modelData === "mono" || parent.modelData === "monoSm"
                                  ? "192.0.2.18:7443"
                                  : "Cut stone, clear words"
                            typeRole: parent.modelData
                        }
                    }
                }
            }
        }

        Column {
            width: parent.width
            spacing: LithicTokens.space.s4
            Label { text: "Cut scale"; typeRole: "section" }
            Row {
                spacing: LithicTokens.space.s4
                Repeater {
                    model: [
                        { name: "toggle knob", value: LithicTokens.cut.toggleKnob },
                        { name: "token", value: LithicTokens.cut.token },
                        { name: "toggle track", value: LithicTokens.cut.toggleTrack },
                        { name: "tooltip", value: LithicTokens.cut.tooltip },
                        { name: "small", value: LithicTokens.cut.small },
                        { name: "mid", value: LithicTokens.cut.mid },
                        { name: "overlay", value: LithicTokens.cut.overlay },
                        { name: "notch", value: LithicTokens.cut.notch },
                        { name: "large", value: LithicTokens.cut.large }
                    ]
                    delegate: Column {
                        required property var modelData
                        spacing: LithicTokens.space.s2
                        CutRect {
                            width: 108
                            height: 48
                            fill: Lithic.stone3
                            strokeColor: Lithic.vein
                            strokeWidth: LithicTokens.size.hairline
                            cut: modelData.value
                        }
                        Label {
                            width: 108
                            text: modelData.name + " " + modelData.value
                            typeRole: "monoSm"
                            inkRole: "fg2"
                            horizontalAlignment: Text.AlignHCenter
                        }
                    }
                }
            }
        }

        Column {
            width: parent.width
            spacing: LithicTokens.space.s4
            Label { text: "Space"; typeRole: "section" }
            Row {
                spacing: LithicTokens.space.s5
                Repeater {
                    model: [
                        { name: "s1", value: LithicTokens.space.s1 },
                        { name: "s2", value: LithicTokens.space.s2 },
                        { name: "s3", value: LithicTokens.space.s3 },
                        { name: "s4", value: LithicTokens.space.s4 },
                        { name: "s5", value: LithicTokens.space.s5 },
                        { name: "s6", value: LithicTokens.space.s6 }
                    ]
                    delegate: Column {
                        required property var modelData
                        spacing: LithicTokens.space.s2
                        Item {
                            width: 150
                            height: 55
                            CutRect {
                                anchors.left: parent.left
                                anchors.verticalCenter: parent.verticalCenter
                                width: modelData.value
                                height: 16
                                cut: Math.min(LithicTokens.cut.token, width / 2)
                                fill: Lithic.fg2
                            }
                        }
                        Label {
                            text: modelData.name + " " + modelData.value
                            typeRole: "monoSm"
                            inkRole: "fg2"
                        }
                    }
                }
            }
        }

        Column {
            width: parent.width
            spacing: LithicTokens.space.s4
            Label { text: "CutRect"; typeRole: "section" }
            Row {
                spacing: LithicTokens.space.s4
                Repeater {
                    model: [
                        { name: "Filled", kind: "fill", corner: "all" },
                        { name: "Inside stroke", kind: "stroke", corner: "all" },
                        { name: "Top-left", kind: "fill", corner: "topLeft" },
                        { name: "Top-right", kind: "fill", corner: "topRight" },
                        { name: "Bottom-right", kind: "fill", corner: "bottomRight" },
                        { name: "Bottom-left", kind: "fill", corner: "bottomLeft" }
                    ]
                    delegate: StateLabel {
                        required property var modelData
                        stateName: modelData.name
                        CutRect {
                            anchors.centerIn: parent
                            width: 132
                            height: 54
                            fill: modelData.kind === "fill" ? Lithic.stone3 : "transparent"
                            strokeColor: Lithic.fg2
                            strokeWidth: modelData.kind === "stroke" ? LithicTokens.size.hairline : 0
                            cutTopLeft: modelData.corner === "all" || modelData.corner === "topLeft" ? LithicTokens.cut.mid : 0
                            cutTopRight: modelData.corner === "all" || modelData.corner === "topRight" ? LithicTokens.cut.mid : 0
                            cutBottomRight: modelData.corner === "all" || modelData.corner === "bottomRight" ? LithicTokens.cut.mid : 0
                            cutBottomLeft: modelData.corner === "all" || modelData.corner === "bottomLeft" ? LithicTokens.cut.mid : 0
                        }
                    }
                }
            }
        }

        Column {
            width: parent.width
            spacing: LithicTokens.space.s4
            Label { text: "Notch"; typeRole: "section" }
            Row {
                spacing: LithicTokens.space.s5
                Repeater {
                    model: [
                        { name: "Top-left", corner: "topLeft" },
                        { name: "Top-right", corner: "topRight" },
                        { name: "Bottom-right", corner: "bottomRight" },
                        { name: "Bottom-left", corner: "bottomLeft" }
                    ]
                    delegate: StateLabel {
                        required property var modelData
                        stateName: modelData.name
                        CutRect {
                            anchors.centerIn: parent
                            width: 92
                            height: 54
                            fill: Lithic.stone3
                            Notch {
                                anchors.fill: parent
                                corner: modelData.corner
                                fill: Lithic.amber
                            }
                        }
                    }
                }
            }
        }

        Column {
            width: parent.width
            spacing: LithicTokens.space.s4
            Label { text: "Blade"; typeRole: "section" }
            Row {
                spacing: LithicTokens.space.s5
                Repeater {
                    model: [0.25, 0.5, 0.75]
                    delegate: StateLabel {
                        required property real modelData
                        stateName: Math.round(modelData * 100) + "%"
                        Blade {
                            anchors.centerIn: parent
                            width: 150
                            height: 8
                            fraction: modelData
                            fill: Lithic.lapis
                        }
                    }
                }
            }
        }

        Column {
            width: parent.width
            spacing: LithicTokens.space.s4
            Label { text: "Flake"; typeRole: "section" }
            Row {
                spacing: LithicTokens.space.s5
                Repeater {
                    model: [
                        { name: "Idle", tone: "idle", fraction: 0 },
                        { name: "Alive", tone: "alive", fraction: 1 },
                        { name: "Becoming 40%", tone: "becoming", fraction: 0.4 },
                        { name: "Attention", tone: "attention", fraction: 1 },
                        { name: "Paused", tone: "paused", fraction: 1 },
                        { name: "Failed", tone: "failed", fraction: 1 }
                    ]
                    delegate: StateLabel {
                        required property var modelData
                        stateName: modelData.name
                        Flake {
                            anchors.centerIn: parent
                            tone: modelData.tone
                            fillFraction: modelData.fraction
                            size: 24
                        }
                    }
                }
            }
        }

        Column {
            width: parent.width
            spacing: LithicTokens.space.s4
            Label { text: "Hatch"; typeRole: "section" }
            Item {
                width: 420
                height: 96
                CutRect {
                    anchors.fill: parent
                    fill: Lithic.stone3
                    cut: LithicTokens.cut.mid
                }
                Hatch {
                    anchors.fill: parent
                    spacing: LithicTokens.size.chartHatchGap
                }
            }
        }

        Column {
            width: parent.width
            spacing: LithicTokens.space.s4
            Label { text: "FocusRing"; typeRole: "section" }
            Row {
                spacing: LithicTokens.space.s5
                StateLabel {
                    stateName: "Pointer focus"
                    Item {
                        anchors.centerIn: parent
                        width: 124
                        height: 48
                        CutRect { anchors.fill: parent; fill: Lithic.stone3; cut: LithicTokens.cut.mid }
                        FocusRing {
                            x: -outset
                            y: -outset
                            width: parent.width + outset * 2
                            height: parent.height + outset * 2
                            cut: LithicTokens.cut.mid
                            keyboardFocusVisible: false
                        }
                    }
                }
                StateLabel {
                    stateName: "Keyboard focus"
                    Item {
                        anchors.centerIn: parent
                        width: 124
                        height: 48
                        CutRect { anchors.fill: parent; fill: Lithic.stone3; cut: LithicTokens.cut.mid }
                        FocusRing {
                            x: -outset
                            y: -outset
                            width: parent.width + outset * 2
                            height: parent.height + outset * 2
                            cut: LithicTokens.cut.mid
                            keyboardFocusVisible: true
                        }
                    }
                }
            }
        }

        Column {
            width: parent.width
            spacing: LithicTokens.space.s4
            Label { text: "Glyph"; typeRole: "section" }
            Row {
                spacing: LithicTokens.space.s5
                Repeater {
                    model: ["home", "search", "settings", "close"]
                    delegate: StateLabel {
                        required property string modelData
                        stateName: modelData.charAt(0).toUpperCase() + modelData.slice(1)
                        Glyph {
                            anchors.centerIn: parent
                            name: modelData
                            size: 28
                        }
                    }
                }
            }
        }

        Column {
            width: parent.width
            spacing: LithicTokens.space.s4
            Label { text: "Motion"; typeRole: "section" }
            CutRect {
                width: parent.width
                height: 88
                fill: Lithic.stone1
                cut: LithicTokens.cut.mid
                Row {
                    anchors.left: parent.left
                    anchors.leftMargin: LithicTokens.space.s4
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: LithicTokens.space.s4
                    Label { text: "At rest"; typeRole: "title" }
                    Blade { width: 320; height: LithicTokens.size.bladeTrack; fraction: 0.62; fill: Lithic.fg2; anchors.verticalCenter: parent.verticalCenter }
                    Label { text: "cubic-bezier(.2, 0, 0, 1)"; typeRole: "mono"; inkRole: "fg2" }
                }
            }
        }
    }
}
