import QtQuick
import QtQuick.Templates as T

T.Control {
    id: root

    property var sections: []
    property int currentIndex: 0
    property string sizeClass: "expanded"
    property Item overlayParent: null
    property string previewState: ""
    readonly property bool expanded: sizeClass === "expanded"

    signal activated(int index)

    implicitWidth: expanded ? LithicTokens.size.sectionIndexWidth
                            : picker.implicitWidth
    implicitHeight: expanded
                    ? sections.length * LithicTokens.size.sectionIndexRow
                      + Math.max(0, sections.length - 1)
                        * LithicTokens.size.sectionIndexRowGap
                    : picker.implicitHeight
    padding: 0
    Accessible.role: Accessible.List

    function sectionLabel(index) {
        const entry = sections[index]
        return typeof entry === "string" ? entry : entry.label || ""
    }

    function choose(index) {
        currentIndex = index
        activated(index)
    }

    background: null

    contentItem: Item {
        Item {
            anchors.fill: parent
            visible: root.expanded

            CutRect {
                y: Math.max(0, Math.min(root.currentIndex, root.sections.length - 1))
                   * (LithicTokens.size.sectionIndexRow
                      + LithicTokens.size.sectionIndexRowGap)
                width: parent.width
                height: LithicTokens.size.sectionIndexRow
                fill: Lithic.sel
                cut: LithicTokens.cut.small
                ChiselNumberBehavior on y { duration: LithicTokens.motion.select }
            }

            Repeater {
                model: root.sections
                delegate: T.AbstractButton {
                    id: sectionButton
                    required property var modelData
                    required property int index
                    y: index * (LithicTokens.size.sectionIndexRow
                                + LithicTokens.size.sectionIndexRowGap)
                    width: parent.width
                    height: LithicTokens.size.sectionIndexRow
                    text: typeof modelData === "string" ? modelData : modelData.label || ""
                    hoverEnabled: true
                    activeFocusOnTab: true
                    Accessible.name: text
                    Accessible.role: Accessible.Button
                    onClicked: root.choose(index)

                    background: CutRect {
                        fill: parent.hovered && root.currentIndex !== index
                              || root.previewState === "hover"
                              ? Lithic.stone3 : "transparent"
                        cut: LithicTokens.cut.small
                        ChiselColorBehavior on fill {
                            duration: parent.hovered || root.previewState === "hover"
                                      ? LithicTokens.motion.hoverIn
                                      : LithicTokens.motion.hoverOut
                        }
                        FocusRing {
                            x: -outset
                            y: -outset
                            width: parent.width + outset * 2
                            height: parent.height + outset * 2
                            keyboardFocusVisible: sectionButton.visualFocus
                            cut: LithicTokens.cut.small
                        }
                    }

                    contentItem: Label {
                        leftPadding: LithicTokens.size.sectionIndexInset
                        text: parent.text
                        typeRole: "label"
                        ink: root.currentIndex === index ? Lithic.onSel : Lithic.fg2
                        verticalAlignment: Text.AlignVCenter
                    }
                }
            }
        }

        Picker {
            id: picker
            visible: !root.expanded
            width: parent.width
            options: root.sections
            overlayParent: root.overlayParent
            previewState: root.previewState
            onActivated: index => root.choose(index)
        }

        Binding {
            target: picker
            property: "currentIndex"
            value: root.currentIndex
        }
    }
}
