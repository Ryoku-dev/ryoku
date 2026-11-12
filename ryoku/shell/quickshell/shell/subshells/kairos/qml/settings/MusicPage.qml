import QtQuick
import Ryoku.Ui.Singletons
import "../kit"

Column {
    id: page

    required property var cfg

    spacing: 20

    MainSwitch {
        objectName: "music"
        width: parent.width
        text: I18n.tr("Use music island")
        checked: page.cfg.music
        onToggled: function (v) { page.cfg.set("music", v); }
    }

    Row {
        width: parent.width
        leftPadding: 8
        spacing: 16

        Icon {
            name: "info"
            size: 22
            color: Md.inkMuted
        }
        Label {
            width: parent.width - 54
            role: "body"
            color: Md.inkMuted
            wrapMode: Text.Wrap
            elide: Text.ElideNone
            text: I18n.tr("While something plays, a bubble with the cover sits beside the clock. Hover it to open the full player.")
        }
    }
}
