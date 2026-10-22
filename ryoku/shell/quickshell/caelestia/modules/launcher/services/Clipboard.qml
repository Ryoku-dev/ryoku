pragma Singleton

import QtQuick
import Quickshell
import Caelestia
import Caelestia.Config
import Caelestia.Services

QtObject {
    id: root

    readonly property var items: ClipboardManager.items

    readonly property bool available: ClipboardManager.available

    readonly property var pinnedItems: ClipboardManager.pinnedItems

    signal imageReady(int id, string path)
    signal clearHistoryFinished(bool success)
    signal pinFailed(int id)

    readonly property string imageCacheDir: ClipboardManager.imageCacheDir

    function reload(): void {
        ClipboardManager.reload();
    }

    function clearHistory(): void {
        ClipboardManager.clearHistory();
    }

    function pin(clipId: int): void {
        ClipboardManager.pin(clipId);
    }

    function unpin(pinId: int): void {
        ClipboardManager.unpin(pinId);
    }

    function copyPinned(pinId: int): void {
        ClipboardManager.copyPinned(pinId);
    }

    /// Pinned entries carry a negative id so they never collide with a cliphist
    /// one, and so existing code that keys on `id` keeps working.
    function toPinnedEntry(pin: var): var {
        return {
            id: -pin.pinId,
            pinId: pin.pinId,
            preview: pin.preview,
            isImage: pin.isImage,
            imagePath: pin.imagePath,
            isPinned: true
        };
    }

    function getSortedItems(): var {
        const pinned = (pinnedItems || []).map(p => root.toPinnedEntry(p));

        const pinnedPreviews = new Set(pinned.map(p => p.preview));
        const rest = (items || []).filter(item => !pinnedPreviews.has(item.preview));

        return [...pinned, ...rest];
    }

    function getImagePath(clipId: int): string {
        return imageCacheDir + "/" + clipId + ".png";
    }

    function isImageCached(clipId: int): bool {
        return ClipboardManager.isImageCached(clipId);
    }

    function migrateFavourites(): void {
        const favs = GlobalConfig.launcher.favouriteClips || [];
        if (!favs.length || !items.length)
            return;

        const live = new Set(items.map(item => String(item.id)));
        for (const fav of favs)
            if (live.has(String(fav)))
                root.pin(Number(fav));

        GlobalConfig.launcher.favouriteClips = [];
    }

    property var _conn: Connections {
        target: ClipboardManager

        function onImageReady(id: int, path: string): void {
            root.imageReady(id, path);
        }

        function onClearHistoryFinished(success: bool): void {
            root.clearHistoryFinished(success);
        }

        function onPinFailed(id: int): void {
            root.pinFailed(id);
        }

        function onItemsChanged(): void {
            root.migrateFavourites();
        }
    }
}
