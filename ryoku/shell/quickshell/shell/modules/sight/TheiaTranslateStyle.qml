pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Templates as T
import Lithic
import Ryoku.Ui.Singletons

Popover {
    id: root
    required property var panel
    required property var catalog
    property real s: 1
    readonly property string style: String(catalog.option("translate.style", "natural"))
    readonly property bool simpler: catalog.option("translate.simpler", false) === true
    readonly property bool back: catalog.option("translate.back", false) === true
    readonly property var styles: [
        { id: "natural", label: I18n.tr("Natural") },
        { id: "literal", label: I18n.tr("Literal") },
        { id: "formal", label: I18n.tr("Formal") },
        { id: "casual", label: I18n.tr("Casual") }
    ]
    readonly property var planes: Lithic.planes(Lithic.design)
    readonly property color plane: Lithic.design === "lithic" ? planes.pane : Lithic.design === "fluent" ? Lithic.mix(planes.card, Lithic.sel, 0.025) : Lithic.stone4
    readonly property color stroke: Lithic.design === "material" ? "transparent" : Lithic.design === "adwaita" ? Lithic.alpha(Lithic.fg, Lithic.dark ? 0.10 : 0.16) : Lithic.design === "fluent" ? planes.cardStroke : Lithic.vein
    signal optionChanged(string key, var value)
    menuFocus: false
    closePolicy: T.Popup.CloseOnEscape | T.Popup.CloseOnPressOutsideParent
    implicitWidth: Math.min(292 * s, Math.max(1, panel.usable.width
                                - LithicTokens.size.popoverAnchorGap * 2 * s))
    implicitHeight: Math.min(298 * s, Math.max(1, panel.usable.height
                                 - LithicTokens.size.popoverAnchorGap * 2 * s))

    function calculatePlacement() {
        return panel.popupPlacement(anchorItem, implicitWidth, implicitHeight, root.parent);
    }
    function setOption(key, value) {
        ShellSettings.patch("sight.translate." + key, value);
        optionChanged(key, value);
    }

    background: CutRect {
        cut: 8
        fill: root.plane
        strokeColor: root.stroke
        strokeWidth: 1
        transform: Translate { y: root.slideOffset }
    }
    contentItem: Loader {
        active: root.visible
        transform: Translate { y: root.slideOffset }
        sourceComponent: Component {
            Flickable {
                contentWidth: width
                contentHeight: options.implicitHeight
                clip: true
                boundsBehavior: Flickable.StopAtBounds
                Column {
                    id: options
                    width: parent.width
                    padding: 8 * root.s
                    spacing: 6 * root.s
                    readonly property real lithicScale: root.s
                    Label {
                        width: parent.width - 16 * root.s
                        height: 24 * root.s
                        text: I18n.tr("Translation style")
                        typeRole: "detail"
                        inkRole: "fg3"
                    }
                    Repeater {
                        model: root.styles
                        delegate: TheiaChip {
                            required property var modelData
                            width: options.width - 16 * root.s
                            text: modelData.label
                            selected: root.style === modelData.id
                            s: root.s
                            onClicked: root.setOption("style", modelData.id)
                        }
                    }
                    TheiaChip {
                        width: options.width - 16 * root.s
                        text: I18n.tr("Simpler words")
                        icon: selected ? "check_box" : "check_box_outline_blank"
                        selected: root.simpler
                        onClicked: root.setOption("simpler", !root.simpler)
                        s: root.s
                    }
                    TheiaChip {
                        width: options.width - 16 * root.s
                        text: I18n.tr("Show it translated back")
                        icon: selected ? "check_box" : "check_box_outline_blank"
                        selected: root.back
                        s: root.s
                        onClicked: root.setOption("back", !root.back)
                    }
                }
            }
        }
    }
}
