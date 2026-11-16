import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import "../about"

// the sheet's colophon at the page's foot: Ryoku's motto and the page's code in bars
Item {
    id: root

    required property var hub

    implicitHeight: 26

    // the motto, set before the page's code
    Label {
        visible: UiPreferences.showJapaneseGloss
        anchors.right: code.left
        anchors.rightMargin: 16
        anchors.verticalCenter: parent.verticalCenter
        text: "力と美のために"
        typeRole: "caption"
        font.pixelSize: 11
        font.letterSpacing: 2
        inkRole: "fg3"
    }

    CardBarcode {
        id: code
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        text: root.hub.pageId || "home"
        ink: Lithic.fg3
        barHeight: 9
        unit: 1
    }
}
