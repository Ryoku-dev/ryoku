import QtQuick
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../schema/performance.js" as Schema

// low power mode, the live wallpaper's hand-offs, the bar's idle drift and the parts that unload
Item {
    id: root

    required property var hub

    readonly property var modes: Changes.stores["powermode"] || null
    readonly property bool lowPower: Changes.value({ store: "performance", key: "lowPowerMode" }) === true
    readonly property bool saverEffects: Changes.value({ store: "performance", key: "powerProfileEffects" }) !== false
    readonly property bool saverOn: !!root.modes && root.modes.profile === "power-saver"

    function focusRow(rowId) {
        page.focusRow(rowId);
    }

    Component.onCompleted: {
        Changes.ensureStore("performance", Qt.resolvedUrl("../components/performance/PerformanceStore.qml"));
        Changes.ensureStore("powermode", Qt.resolvedUrl("../components/power/PowerModeStore.qml"));
    }

    readonly property string stateLine: root.lowPower
        ? I18n.tr("Low power mode is on: motion, blur and shadows are off.")
        : root.saverOn && root.saverEffects
            ? I18n.tr("Power saver is on, so motion, blur and shadows are off for now.")
            : ""

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "performance"
        schema: Schema
        stateLine: root.stateLine
    }
}
