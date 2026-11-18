import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"

Slab {
    id: root
    readonly property bool wide: true

    required property var store
    property var row: null
    property var value: true
    property var commit: null
    property var featureRows: []
    readonly property var planes: Lithic.planes(Lithic.design)
    readonly property color plane: Lithic.design === "lithic" ? planes.pane : Lithic.design === "fluent" ? Lithic.mix(planes.card, Lithic.sel, 0.025) : Lithic.stone4
    readonly property bool lookOn: root.store && root.store.read("enabled") !== false
    readonly property bool highlightOn: root.store && root.store.read("highlight") !== false
    readonly property bool inplaceOn: root.store && root.store.read("features.inplace") === true
    readonly property string lookKey: root.store ? root.store.tapLabel : ""
    readonly property string highlightKey: root.store ? root.store.highlightLabel : ""
    readonly property string inplaceKey: root.store ? root.store.keyLabel("inplace.key", "Control_R", "Right Ctrl") : ""
    readonly property int featureCount: {
        if (!root.visible)
            return 0;
        return root.featureRows.filter(r => Changes.value(r.bind) === true).length;
    }
    readonly property string status: {
        if (root.value !== true)
            return I18n.tr("Off. Your gestures stay saved.");
        var gestures = [];
        if (root.lookOn)
            gestures.push(I18n.tr("%1 three times").arg(root.lookKey));
        if (root.highlightOn)
            gestures.push(I18n.tr("%1 while you highlight").arg(root.highlightKey));
        if (root.inplaceOn)
            gestures.push(I18n.tr("%1 three times with text selected").arg(root.inplaceKey));
        if (gestures.length === 0)
            return I18n.tr("On. Choose a gesture below to start.");
        var last = gestures.pop();
        var keys = gestures.length ? I18n.tr("%1 and %2").arg(gestures.join(", ")).arg(last) : last;
        return I18n.tr("Listening for %1.").arg(keys);
    }

    width: LithicTokens.size.settingsControlMax
    implicitHeight: heroBody.implicitHeight + inset * 2
    inset: LithicTokens.space.s3
    fill: Lithic.mix(root.plane, Lithic.sel, root.value === true ? 0.10 : 0.025)
    Accessible.role: Accessible.Grouping
    Accessible.name: I18n.tr("Theia gestures")
    Accessible.description: root.status

    Behavior on fill { ColorAnimation { duration: Lithic.duration(160) } }

    function activate() {
        if (root.enabled && root.visible && root.commit)
            root.commit(root.value !== true);
    }

    Column {
        id: heroBody
        width: parent.width
        spacing: LithicTokens.space.s3

        Item {
            width: parent.width
            height: Math.max(count.implicitHeight, master.implicitHeight)
            Label {
                id: count
                anchors.left: parent.left
                anchors.right: master.left
                anchors.rightMargin: LithicTokens.space.s3
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("%1 of %2 on").arg(root.featureCount).arg(root.featureRows.length)
                typeRole: "label"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Toggle {
                id: master
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                checked: root.value === true
                Accessible.name: I18n.tr("Use Theia")
                onToggled: {
                    var want = master.checked;
                    master.checked = Qt.binding(() => root.value === true);
                    if (root.enabled && root.visible && root.commit)
                        root.commit(want);
                }
            }
        }

        Label {
            width: parent.width
            text: root.status
            typeRole: "detail"
            inkRole: "fg2"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }

        Column {
            width: parent.width
            spacing: LithicTokens.space.s2
            Repeater {
                model: [
                    { label: I18n.tr("Look"), key: I18n.tr("%1 ×3").arg(root.lookKey), on: root.lookOn },
                    { label: I18n.tr("Highlight"), key: I18n.tr("Hold %1").arg(root.highlightKey), on: root.highlightOn },
                    { label: I18n.tr("In place"), key: I18n.tr("%1 ×3").arg(root.inplaceKey), on: root.inplaceOn }
                ]
                delegate: Item {
                    id: gesture
                    required property var modelData
                    width: parent.width
                    height: Math.max(gestureLabel.implicitHeight, cap.implicitHeight)
                    opacity: root.value === true && modelData.on ? 1 : Lithic.disabledOpacity
                    Behavior on opacity { NumberAnimation { duration: Lithic.duration(120) } }
                    Label {
                        id: gestureLabel
                        anchors.left: parent.left
                        anchors.right: cap.left
                        anchors.rightMargin: LithicTokens.space.s2
                        anchors.verticalCenter: parent.verticalCenter
                        text: gesture.modelData.label
                        typeRole: "label"
                    }
                    CutRect {
                        id: cap
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        implicitWidth: Math.min(180, capLabel.implicitWidth + LithicTokens.space.s3 * 2)
                        implicitHeight: capLabel.implicitHeight + LithicTokens.space.s2
                        cut: LithicTokens.cut.small
                        fill: Lithic.field
                        strokeColor: Lithic.vein
                        strokeWidth: LithicTokens.size.hairline
                        Label {
                            id: capLabel
                            anchors.centerIn: parent
                            width: parent.width - LithicTokens.space.s3 * 2
                            text: gesture.modelData.key
                            typeRole: "monoSm"
                            horizontalAlignment: Text.AlignHCenter
                        }
                    }
                }
            }
        }
    }
}
