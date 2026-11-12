pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../Singletons"
import ".."

Item {
    id: page

    property var hub
    readonly property var shippedDesigns: [
        { id: "ryoku", name: "Ryoku", description: "The paper-and-ink look that ships with Ryoku." }
    ]

    function wear(setting, designId) {
        Settings.patch("designs." + setting, designId);
    }

    function tryOsd() {
        Bus.ipc("osd", "preview", []);
    }

    function tryNotification() {
        Quickshell.execDetached([
            "notify-send", "-a", I18n.tr("Ryoku"),
            I18n.tr("Ryoku notification"),
            I18n.tr("This is how your notifications look.")
        ]);
    }

    function trySidebar() {
        Bus.run("quicksettings");
    }

    SchemaPage {
        id: scaffold
        anchors {
            fill: parent
            leftMargin: Tokens.s6
            rightMargin: Tokens.s6
            topMargin: Tokens.s5
        }
        schema: []
        draft: null
        defaults: ({})
        title: I18n.tr("Overlays")
        eyebrow: I18n.tr("DESKTOP")
        blurb: I18n.tr("Choose how desktop pop-ups, notification cards and the Super+Escape panel look.")

        Column {
            width: parent.width
            spacing: Tokens.s5

            Text {
                width: Math.min(parent.width, 840)
                text: I18n.tr("Bar styles that draw their own notifications or pop-ups keep their own design.")
                color: Tokens.inkFaint
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                wrapMode: Text.WordWrap
            }

            SettingCard {
                width: parent.width
                title: I18n.tr("Pop-ups")

                StoreDesignGallery {
                    width: parent.width
                    category: "osds"
                    wornId: Settings.get("designs.osd") || "ryoku"
                    shippedDesigns: page.shippedDesigns
                    extraActionText: I18n.tr("TRY")
                    onWear: designId => page.wear("osd", designId)
                    onExtraAction: page.tryOsd()
                }
            }

            SettingCard {
                width: parent.width
                title: I18n.tr("Notifications")

                StoreDesignGallery {
                    width: parent.width
                    category: "notifications"
                    wornId: Settings.get("designs.notification") || "ryoku"
                    shippedDesigns: page.shippedDesigns
                    extraActionText: I18n.tr("TRY")
                    onWear: designId => page.wear("notification", designId)
                    onExtraAction: page.tryNotification()
                }
            }

            SettingCard {
                width: parent.width
                title: I18n.tr("Super+Esc panel")

                StoreDesignGallery {
                    width: parent.width
                    category: "sidebars"
                    wornId: Settings.get("designs.sidebar") || "ryoku"
                    shippedDesigns: page.shippedDesigns
                    extraActionText: I18n.tr("TRY")
                    onWear: designId => page.wear("sidebar", designId)
                    onExtraAction: page.trySidebar()
                }
            }
        }
    }
}
