pragma Singleton

import QtQuick
import Caelestia
import Caelestia.Config
import Caelestia.Services

QtObject {
    id: root

    readonly property bool _loaded: EmojiDb.loaded
    readonly property int itemCount: EmojiDb.count

    property Connections favConnections: Connections {
        target: GlobalConfig.launcher

        function onFavouriteEmojisChanged(): void {
        }
    }

    function reload(): void {
        if (!EmojiDb.loaded) {
            console.warn("EmojiDb not loaded yet");
        }
    }

    function recordUsage(ch: string): void {
        EmojiDb.recordUsage(ch);
    }

    function getSortedItems(): var {
        const favEmojis = GlobalConfig.launcher.favouriteEmojis || [];
        return EmojiDb.getSortedItems(favEmojis);
    }

    function search(text: string): var {
        return EmojiDb.search(text, 500);
    }
}