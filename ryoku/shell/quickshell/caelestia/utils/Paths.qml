pragma Singleton

import QtQuick
import Quickshell
import Caelestia
import Caelestia.Config

Singleton {
    id: root

    readonly property string home: Quickshell.env("HOME")
    // the port's own folder: "root:" paths must find its assets from any shell
    // that imports it, not only from `qs -c caelestia`
    readonly property string caelestiaDir: toLocalFile(Qt.resolvedUrl("..")).replace(/\/$/, "")
    // loaded by another shell (the Ryoku bar style) rather than run as `qs -c caelestia`:
    // process-wide services that shell already owns, like the notification server, stay off
    readonly property bool hosted: Quickshell.shellDir.replace(/\/$/, "") !== caelestiaDir
    readonly property string pictures: Quickshell.env("XDG_PICTURES_DIR") || `${home}/Pictures`
    readonly property string videos: Quickshell.env("XDG_VIDEOS_DIR") || `${home}/Videos`

    readonly property string data: `${Quickshell.env("XDG_DATA_HOME") || `${home}/.local/share`}/caelestia`
    readonly property string state: `${Quickshell.env("XDG_STATE_HOME") || `${home}/.local/state`}/caelestia`
    readonly property string cache: `${Quickshell.env("XDG_CACHE_HOME") || `${home}/.cache`}/caelestia`
    readonly property string config: `${Quickshell.env("XDG_CONFIG_HOME") || `${home}/.config`}/caelestia`
    
    readonly property string runtimeDir: Quickshell.env("XDG_RUNTIME_DIR") || `/tmp/caelestia-${Quickshell.env("USER")}`
    
    function runtimeTemp(name: string): string {
        return `${runtimeDir}/caelestia-${name}`;
    }

    readonly property string imagecache: `${cache}/imagecache`

    readonly property string notifimagecache: `${imagecache}/notifs`

    readonly property string wallsdir: Quickshell.env("CAELESTIA_WALLPAPERS_DIR") || absolutePath(GlobalConfig.paths.wallpaperDir)

    readonly property string recsdir: Quickshell.env("CAELESTIA_RECORDINGS_DIR") || `${videos}/Recordings`

    readonly property string libdir: Quickshell.env("CAELESTIA_LIB_DIR") || "/usr/lib/caelestia"

    // Where the caelestia commands are installed: /usr/bin when a package owns
    // them, ~/.local/bin for a source install. Both autostart scripts export
    // CAELESTIA_BIN_DIR, and the fallback covers a shell started by hand.
    readonly property string binDir: Quickshell.env("CAELESTIA_BIN_DIR") || `${home}/.local/bin`

    function bin(name: string): string {
        return absolutePath(`${binDir}/${name}`);
    }

    function toLocalFile(path: url): string {
        path = Qt.resolvedUrl(path);
        return path.toString() ? CUtils.toLocalFile(path) : "";
    }

    function absolutePath(path: string): string {
        return toLocalFile(path.replace(/~|(\$({?)HOME(}?))+/, home).replace(/^root:/, root.caelestiaDir + "/"));
    }

    function shortenHome(path: string): string {
        return path.replace(home, "~");
    }
}
