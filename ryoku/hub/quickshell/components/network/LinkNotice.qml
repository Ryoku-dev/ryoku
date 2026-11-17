import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import "../wifi"

// why the network controls are off, when they are: the desktop service or NetworkManager does not answer
Slab {
    id: notice

    property Item host: null

    visible: NetworkLink.unavailable !== ""
    width: parent ? parent.width : 0

    Column {
        width: parent.width

        NetRow {
            host: notice.host
            first: true
            title: NetworkLink.silent || !Bus.connected ? I18n.tr("Network settings are off") : I18n.tr("Waiting for NetworkManager")
            detail: NetworkLink.unavailable
        }
    }
}
