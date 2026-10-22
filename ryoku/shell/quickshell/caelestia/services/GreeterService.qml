pragma Singleton

import QtQuick
import QtCore
import Quickshell
import Caelestia.Config
import Caelestia.Models
import caelestia.services
import caelestia.utils

Singleton {
    id: root

    property int activePopoutCount: 0
    property var allSlideshowMedia: []
    property int currentIndex: greeterSettings.lastIndex
    property string currentMedia: greeterSettings.lastMedia
    readonly property bool isPopoutOpen: activePopoutCount > 0
    readonly property string timeOfDayMedia: {
        const hr = Time.hours;
        const mStart = Config.bar.greeter.morningStart;
        const aStart = Config.bar.greeter.afternoonStart;
        const eStart = Config.bar.greeter.eveningStart;
        const nStart = Config.bar.greeter.nightStart;

        if (hr >= mStart && hr < aStart) {
            return resolvePath(Config.bar.greeter.morningGif);
        } else if (hr >= aStart && hr < eStart) {
            return resolvePath(Config.bar.greeter.afternoonGif);
        } else if (hr >= eStart && hr < nStart) {
            return resolvePath(Config.bar.greeter.eveningGif);
        } else {
            return resolvePath(Config.bar.greeter.nightGif);
        }
    }
    readonly property string activeMedia: (Config.bar.greeter.mode === "slideshow") ? (currentMedia || (allSlideshowMedia.length > 0 ? allSlideshowMedia[0] : timeOfDayMedia)) : timeOfDayMedia

    function resolvePath(p: string): string {
        if (!p) return "";
        if (p.startsWith("file://")) {
            p = p.slice(7);
        }
        if (p.startsWith("root:/")) {
            return Quickshell.shellPath(p.slice(6));
        }
        if (p.startsWith("~")) {
            return Paths.home + p.slice(1);
        }
        return p;
    }

    function updateMediaList(): void {
        let files = [];
        const manualMedia = Config.bar.greeter.slideshowGifs || [];
        for (let i = 0; i < manualMedia.length; i++) {
            if (manualMedia[i]) {
                const resolved = resolvePath(manualMedia[i]);
                if (resolved && !files.includes(resolved)) {
                    files.push(resolved);
                }
            }
        }
        for (let i = 0; i < folderScanners.count; i++) {
            const scanner = folderScanners.objectAt(i);
            if (scanner && scanner.entries) {
                for (let j = 0; j < scanner.entries.length; j++) {
                    const entry = scanner.entries[j];
                    if (entry && entry.path && !files.includes(entry.path)) {
                        files.push(entry.path);
                    }
                }
            }
        }

        if (files.length === 0) {
            const timeGifs = [
                Config.bar.greeter.morningGif,
                Config.bar.greeter.afternoonGif,
                Config.bar.greeter.eveningGif,
                Config.bar.greeter.nightGif
            ];
            for (let i = 0; i < timeGifs.length; i++) {
                const resolved = resolvePath(timeGifs[i]);
                if (resolved) {
                    files.push(resolved);
                }
            }
        }

        allSlideshowMedia = files;

        const targetMedia = currentMedia || greeterSettings.lastMedia;
        const foundIdx = targetMedia ? allSlideshowMedia.indexOf(targetMedia) : -1;
        if (foundIdx !== -1) {
            currentIndex = foundIdx;
        } else if (currentIndex >= allSlideshowMedia.length) {
            currentIndex = Math.max(0, Math.min(greeterSettings.lastIndex, allSlideshowMedia.length - 1));
        }

        if (allSlideshowMedia.length > 0) {
            currentMedia = allSlideshowMedia[currentIndex];
            greeterSettings.lastMedia = currentMedia;
            greeterSettings.lastIndex = currentIndex;
        }
    }

    Component.onCompleted: updateMediaList()

    Settings {
        id: greeterSettings

        property string lastMedia: ""
        property int lastIndex: 0

        category: "Greeter"
    }

    Instantiator {
        id: folderScanners

        model: Config.bar.greeter.slideshowFolders
        onObjectAdded: root.updateMediaList()
        onObjectRemoved: root.updateMediaList()
        onCountChanged: root.updateMediaList()

        delegate: FileSystemModel {
            id: fsModel

            path: root.resolvePath(modelData)
            filter: FileSystemModel.Files
            nameFilters: Images.validImageExtensions.concat(Images.validVideoExtensions).map(e => `*.${e}`)
            recursive: true
            onEntriesChanged: root.updateMediaList()
            Component.onCompleted: root.updateMediaList()
        }
    }

    Connections {
        function onSlideshowGifsChanged(): void { root.updateMediaList(); }
        function onSlideshowFoldersChanged(): void { root.updateMediaList(); }
        function onMorningGifChanged(): void { root.updateMediaList(); }
        function onAfternoonGifChanged(): void { root.updateMediaList(); }
        function onEveningGifChanged(): void { root.updateMediaList(); }
        function onNightGifChanged(): void { root.updateMediaList(); }

        target: Config.bar.greeter
    }

    Timer {
        id: slideshowTimer

        interval: Math.max(2, Math.round(Config.bar.greeter.slideshowInterval)) * 1000
        running: root.isPopoutOpen && Config.bar.greeter.mode === "slideshow" && root.allSlideshowMedia.length > 1
        repeat: true
        onTriggered: {
            const list = root.allSlideshowMedia;
            if (list.length === 0) return;
            if (Config.bar.greeter.slideshowRandom && list.length > 1) {
                let nextIdx = Math.floor(Math.random() * list.length);
                if (nextIdx === root.currentIndex) nextIdx = (nextIdx + 1) % list.length;
                root.currentIndex = nextIdx;
            } else {
                root.currentIndex = (root.currentIndex + 1) % list.length;
            }
            root.currentMedia = list[root.currentIndex];
            greeterSettings.lastMedia = root.currentMedia;
            greeterSettings.lastIndex = root.currentIndex;
        }
    }
}
