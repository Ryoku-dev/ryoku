import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Io
import stage.modules.common
import stage.modules.common.widgets
import stage.services

ColumnLayout {
    id: root

    property string currentValue: "ryoku"
    signal selected(string value)

    // Docks from RyoStore, installed or not. Install stays in RyoStore; this
    // picker wears what is yours, offers its updates, and points to the Store
    // for the rest.
    property var storeDocks: []
    property string updatingId: ""

    readonly property var designs: [
        { value: "ryoku", title: Translation.tr("Ryoku"), detail: Translation.tr("Paper islands"), glyph: "dock" },
        { value: "python", title: Translation.tr("Python"), detail: Translation.tr("Flowing glass"), glyph: "blur_on" },
        { value: "shima", title: Translation.tr("Shima"), detail: Translation.tr("Notched capsule"), glyph: "line_curve" },
        { value: "caelestia", title: Translation.tr("Caelestia"), detail: Translation.tr("Inside the Caelestia bar"), glyph: "rounded_corner" },
        { value: "ii", title: Translation.tr("ii"), detail: Translation.tr("Islands, widgets, magnification"), glyph: "view_carousel" },
        { value: "none", title: Translation.tr("None"), detail: Translation.tr("No dock surface"), glyph: "block" }
    ].concat(root.storeDocks.map(dock => ({
        value: dock.id,
        title: dock.name,
        detail: dock.installed ? dock.summary : Translation.tr("Install from RyoStore"),
        glyph: "storefront",
        store: true,
        installed: dock.installed,
        updateAvailable: dock.updateAvailable
    })))

    function browseStore() {
        Quickshell.execDetached(["ryostore", "open", "docks"]);
    }
    // Re-running the install replaces the dock in place, and the shell swaps
    // the new view in without a reload.
    function updateDock(id) {
        if (!id || root.updatingId !== "")
            return;
        root.updatingId = id;
        dockUpdate.command = ["ryostore", "install", "docks", id];
        dockUpdate.running = true;
    }

    Layout.fillWidth: true
    spacing: Appearance.sizes.space2

    Process {
        id: storeCatalog
        command: ["ryostore", "catalog", "--category", "docks"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    const catalog = JSON.parse(this.text || "{}");
                    // A dock written for another window manager that you have
                    // not installed can neither run nor be fetched here.
                    root.storeDocks = (catalog.items || [])
                        .filter(item => item.category === "docks"
                            && !(item.unavailable === true && item.installed !== true))
                        .map(item => ({
                            id: item.id,
                            name: item.name || item.id,
                            summary: item.summary || "",
                            installed: item.installed === true,
                            updateAvailable: item.updateAvailable === true
                        }));
                } catch (e) {
                    root.storeDocks = [];
                }
            }
        }
    }

    // Any outcome re-reads the shelf, so a failed update keeps its button.
    Process {
        id: dockUpdate
        onExited: {
            root.updatingId = "";
            storeCatalog.running = true;
        }
    }

    EditPanelSectionLabel { text: Translation.tr("Design") }

    GridLayout {
        Layout.fillWidth: true
        columns: 2
        columnSpacing: Appearance.sizes.space2
        rowSpacing: Appearance.sizes.space2

        Repeater {
            model: root.designs

            // The selected card is the inverted plate, so EVERYTHING on it -
            // glyph, title, detail - has to read through the inverse pair. A
            // plain colOnSurface is the ink for the paper cards, which is the
            // same family as the bone plate on a light palette and the opposite
            // family on a dark one: the chosen design then painted itself
            // invisible, which is the "text and colour are bad on some
            // wallpapers" report.
            delegate: Rectangle {
                id: card
                required property var modelData
                required property int index

                readonly property bool chosen: root.currentValue === modelData.value
                readonly property bool store: modelData.store === true
                // A Store dock you have not installed opens RyoStore instead.
                readonly property bool wearable: !card.store || modelData.installed === true
                readonly property color plate: chosen
                    ? Appearance.colors.colSecondaryContainer : Appearance.colors.colLayer1
                readonly property color ink: chosen
                    ? Appearance.colors.colOnSecondaryContainer : Appearance.colors.colOnSurface
                readonly property color inkDim: chosen
                    ? Appearance.colors.colOnSecondaryContainer : Appearance.colors.colOnSurfaceVariant

                Layout.fillWidth: true
                implicitHeight: 108
                radius: Appearance.rounding.small
                color: card.plate
                opacity: card.wearable ? 1 : 0.6
                border.width: 1
                border.color: chosen
                    ? Appearance.colors.colSecondary : Appearance.colors.colOutlineVariant

                // Under the content, so the update pill keeps its own clicks.
                MouseArea {
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: Qt.PointingHandCursor
                    onClicked: card.wearable ? root.selected(card.modelData.value) : root.browseStore()
                }

                ColumnLayout {
                    anchors.fill: parent
                    anchors.margins: Appearance.sizes.space3
                    spacing: Appearance.sizes.space2

                    Item {
                        Layout.fillWidth: true
                        implicitHeight: 38

                        // The dock's own preview plate: an ink wash on paper, the
                        // inverse wash on the selected plate, so the miniature
                        // reads on both.
                        Rectangle {
                            anchors.horizontalCenter: parent.horizontalCenter
                            anchors.bottom: parent.bottom
                            width: card.modelData.value === "none" ? 42 : card.modelData.value === "python" ? 94 : 82
                            height: card.modelData.value === "none" ? 2 : 30
                            radius: card.modelData.value === "ryoku" ? 7 : height / 2
                            color: card.modelData.value === "none"
                                ? Appearance.colors.colOutlineVariant
                                : card.chosen
                                    ? Appearance.withAlpha(card.ink, 0.14)
                                    : Appearance.colors.colSurfaceContainerHighest

                            Row {
                                anchors.centerIn: parent
                                spacing: card.modelData.value === "ryoku" ? 7 : 4
                                visible: card.modelData.value !== "none"
                                Repeater {
                                    model: 4
                                    Rectangle {
                                        width: 12
                                        height: 12
                                        radius: card.modelData.value === "shima" ? 6 : 4
                                        color: index === 1
                                            ? (card.chosen ? Appearance.colors.colLayer0
                                                : Appearance.colors.colSecondary)
                                            : card.ink
                                    }
                                }
                            }
                        }
                    }

                    RowLayout {
                        Layout.fillWidth: true
                        MaterialSymbol {
                            text: card.modelData.glyph
                            iconSize: 18
                            color: card.ink
                        }
                        StyledText {
                            Layout.fillWidth: true
                            text: card.modelData.title
                            color: card.ink
                            font.weight: Font.DemiBold
                            elide: Text.ElideRight
                        }
                        StyledText {
                            visible: card.store && !update.visible
                            text: Translation.tr("STORE")
                            color: card.inkDim
                            font.pixelSize: Appearance.font.pixelSize.smaller
                        }
                        // A newer version in RyoStore: the pill takes the STORE
                        // tag's place, in the card's own ink so it reads on the
                        // chosen plate as well as on paper.
                        Rectangle {
                            id: update
                            visible: card.store && card.wearable && card.modelData.updateAvailable === true
                            implicitWidth: updateLabel.implicitWidth + Appearance.sizes.space2 * 2
                            implicitHeight: updateLabel.implicitHeight + Appearance.sizes.space1 * 2
                            radius: height / 2
                            color: card.ink
                            StyledText {
                                id: updateLabel
                                anchors.centerIn: parent
                                text: root.updatingId === card.modelData.value
                                    ? Translation.tr("Updating") : Translation.tr("Update")
                                color: card.plate
                                font.pixelSize: Appearance.font.pixelSize.smaller
                                font.weight: Font.DemiBold
                            }
                            MouseArea {
                                anchors.fill: parent
                                cursorShape: Qt.PointingHandCursor
                                onClicked: root.updateDock(card.modelData.value)
                            }
                        }
                        MaterialSymbol {
                            visible: card.chosen
                            text: "check_circle"
                            iconSize: 18
                            color: card.chosen ? card.ink : Appearance.colors.colSecondary
                        }
                    }
                    StyledText {
                        Layout.fillWidth: true
                        text: card.modelData.detail
                        color: card.inkDim
                        opacity: card.chosen ? 0.78 : 1
                        font.pixelSize: Appearance.font.pixelSize.small
                        elide: Text.ElideRight
                    }
                }
            }
        }
    }

    EditPanelRow {
        Layout.fillWidth: true
        symbol: "storefront"
        title: Translation.tr("Browse RyoStore")
        subtitle: Translation.tr("Docks made by the community")
        onActivated: root.browseStore()
    }
}
