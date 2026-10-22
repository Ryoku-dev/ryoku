import QtQuick
import "dash"
import QtQuick.Layouts
import Caelestia.Config
import caelestia.components
import caelestia.components.controls
import caelestia.components.filedialog
import caelestia.services

// The user and the player on top, then one card per Controls section the Hub's Controls
// page shows, in the order it keeps them. The player stands on the right over the rows
// of the smaller cards; the quick toggles span the whole width.
GridLayout {
    id: root

    required property DrawerVisibilities visibilities
    required property ScreenState screenState
    required property FileDialog facePicker

    readonly property var cards: HostState.controlsOrder.filter(id => ["connections", "powerProfile", "levels", "bottomControls"].indexOf(id) >= 0)
    // connections and the power profile share a row; either one alone takes the row
    readonly property bool paired: cards.indexOf("connections") >= 0 && cards.indexOf("powerProfile") >= 0
    // without the levels row, the quick toggles take its place beside the player
    readonly property bool togglesBeside: cards.indexOf("levels") < 0
    function span(id: string): int {
        if (id === "connections" || id === "powerProfile")
            return paired ? 1 : 2;
        if (id === "bottomControls")
            return togglesBeside ? 2 : 3;
        return 2;
    }
    // rows of the cards beside the player, two cells per row
    readonly property int sideRows: Math.max(1, Math.ceil(cards.reduce((n, id) => n + (span(id) === 3 ? 0 : span(id)), 0) / 2))

    columns: 3
    rowSpacing: Tokens.spacing.medium
    columnSpacing: Tokens.spacing.medium

    Rect {
        Layout.columnSpan: 2
        Layout.fillWidth: true
        Layout.preferredWidth: Tokens.sizes.dashboard.userWidth * 1.75
        Layout.minimumHeight: 140
        visible: HostState.element("identity")

        radius: Tokens.rounding.extraLarge

        User {
            visibilities: root.visibilities
            facePicker: root.facePicker
        }

        ColumnLayout {
            anchors.right: parent.right
            anchors.rightMargin: Tokens.padding.extraLarge
            anchors.verticalCenter: parent.verticalCenter
            spacing: 0

            StyledText {
                Layout.alignment: Qt.AlignRight
                text: `${Time.hourStr}:${Time.minuteStr}`
                color: Colours.palette.m3secondary
                font: Tokens.font.clock.size(40).weight(Font.DemiBold).build()
            }

            StyledText {
                Layout.alignment: Qt.AlignRight
                text: Time.format("dddd, d MMMM")
                color: Colours.palette.m3onSurfaceVariant
                font: Tokens.font.body.medium
            }
        }
    }

    Rect {
        Layout.row: 0
        Layout.column: 2
        Layout.rowSpan: 1 + root.sideRows
        Layout.fillHeight: true
        Layout.preferredWidth: media.implicitWidth
        // tall enough for the cover, the controls and the gif under them, however short
        // the cards beside it are
        Layout.minimumHeight: Math.round(media.implicitWidth * 1.95)
        visible: HostState.section("media")

        radius: Tokens.rounding.extraLarge * 2

        Media {
            id: media
        }
    }

    Repeater {
        model: root.cards

        delegate: Rect {
            id: card

            required property string modelData

            Layout.fillWidth: true
            Layout.fillHeight: modelData === "connections" || modelData === "powerProfile"
            Layout.columnSpan: root.span(modelData)
            Layout.preferredWidth: Tokens.sizes.dashboard.userWidth * 0.875
            Layout.preferredHeight: loader.item?.implicitHeight ?? 0

            radius: Tokens.rounding.extraLarge

            Loader {
                id: loader

                anchors.fill: parent
                sourceComponent: card.modelData === "connections" ? connections
                    : card.modelData === "powerProfile" ? power
                    : card.modelData === "levels" ? levels : toggles
            }
        }
    }

    Component {
        id: connections

        ControlsConnections {
            showPower: false
            wide: !root.paired
        }
    }

    Component {
        id: power

        ControlsConnections {
            showConnections: false
            tall: root.paired && ["wifi", "bluetooth", "ethernet", "vpn"].filter(id => HostState.element(id)).length >= 3
        }
    }

    Component {
        id: levels

        ControlsLevels {
            screenState: root.screenState
        }
    }

    Component {
        id: toggles

        ControlsToggles {
            visibilities: root.visibilities
        }
    }

    component Rect: StyledRect {
        color: Colours.tPalette.m3surfaceContainer
    }
}
