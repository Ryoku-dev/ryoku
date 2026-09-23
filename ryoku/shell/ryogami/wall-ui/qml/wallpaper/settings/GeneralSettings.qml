import QtQuick
import "../.."
import "../../components"
import "../../services"
import Ryoku.Ui.Singletons

// Two independent columns (masonry) so a short card doesn't leave dead space
// under it: left = General + Behaviour, right = Features + Random rotation. Each
// column packs top-down, so the next card follows immediately.
Row {
    id: root
    property var colors
    property var saveConfigKey
    property var _recolorThemes: []

    width: parent ? parent.width : 0
    spacing: 12

    Component.onCompleted: {
        DaemonClient.call("effects.list", {}, function(result, err) {
            if (err || !result || !result.effects) return
            for (var i = 0; i < result.effects.length; i++) {
                var eff = result.effects[i]
                if (eff.id !== "theme" || !eff.params) continue
                for (var j = 0; j < eff.params.length; j++) {
                    if (eff.params[j].id === "theme") {
                        root._recolorThemes = eff.params[j].options || []
                        return
                    }
                }
            }
        })
    }

    Column {
        width: (root.width - root.spacing) / 2
        spacing: 12

        SettingsCard {
            colors: root.colors
            title: I18n.tr("General")
            width: parent.width

            RowTextInput {
                colors: root.colors
                title: I18n.tr("Monitor")
                description: I18n.tr("Restrict to a specific monitor (e.g. DP-1). Leave empty for all.")
                value: Config.mainMonitor
                placeholder: I18n.tr("e.g. DP-1")
                onCommit: function(v) { if (root.saveConfigKey) root.saveConfigKey("monitor", v) }
            }

            RowTextInput {
                colors: root.colors
                title: I18n.tr("UI scale")
                description: I18n.tr("Scale the entire selector UI. Range 0.5–2.0 (below 1.0 shrinks it to fit small screens).")
                value: Config.uiScale.toFixed(2)
                placeholder: "1.00"
                onCommit: function(v) {
                    var n = parseFloat(v)
                    if (isNaN(n)) n = 1.0
                    n = Math.max(0.5, Math.min(2.0, n))
                    if (root.saveConfigKey) root.saveConfigKey("general.uiScale", n)
                }
            }
        }

        SettingsCard {
            colors: root.colors
            title: I18n.tr("Behaviour")
            width: parent.width

            RowToggle {
                colors: root.colors
                title: I18n.tr("Apply per monitor")
                description: I18n.tr("Allow picking a different wallpaper for each monitor. Video and Wallpaper Engine support is in progress.")
                checked: Config.wallpaperPerMonitor
                onToggle: function(v) { if (root.saveConfigKey) root.saveConfigKey("general.wallpaperPerMonitor", v) }
            }

            RowInput {
                colors: root.colors
                title: I18n.tr("Selector backdrop dim (%)")
                description: I18n.tr("How dark to make the area behind the wallpaper selector card. 0 disables the dim entirely.")
                value: Config.selectorBackdropOpacity
                min: 0; max: 100
                onCommit: function(v) { if (root.saveConfigKey) root.saveConfigKey("general.selectorBackdropOpacity", v) }
            }

            RowToggle {
                colors: root.colors
                title: I18n.tr("Close on apply")
                description: I18n.tr("Close the picker once a wallpaper, theme or rice is applied.")
                checked: Config.closeOnSelection
                onToggle: function(v) { if (root.saveConfigKey) root.saveConfigKey("general.closeOnSelection", v) }
            }

            RowToggle {
                colors: root.colors
                title: I18n.tr("Always show filter bar")
                description: I18n.tr("Keep the filter bar pinned visible instead of auto-hiding.")
                checked: Config.filterBarAlwaysVisible
                onToggle: function(v) { if (root.saveConfigKey) root.saveConfigKey("general.filterBarAlwaysVisible", v) }
            }

            RowToggle {
                colors: root.colors
                title: I18n.tr("Video auto-scale")
                description: I18n.tr("Auto-scale videos to fit the wallpaper resolution.")
                checked: Config.videoAutoScale
                onToggle: function(v) { if (root.saveConfigKey) root.saveConfigKey("features.videoAutoScale", v) }
            }

            RowToggle {
                colors: root.colors
                title: I18n.tr("Notify on wallpaper change")
                description: I18n.tr("Send a system notification each time the wallpaper changes.")
                checked: Config.notifyOnWallpaperChange
                onToggle: function(v) { if (root.saveConfigKey) root.saveConfigKey("general.notifyOnWallpaperChange", v) }
            }

            RowToggle {
                colors: root.colors
                title: I18n.tr("Restore wallpaper on startup")
                description: I18n.tr("Re-apply the last wallpaper when the daemon starts.")
                checked: Config.restoreOnStartup
                onToggle: function(v) { if (root.saveConfigKey) root.saveConfigKey("restoreOnStartup", v) }
            }
        }
    }

    Column {
        width: (root.width - root.spacing) / 2
        spacing: 12

        SettingsCard {
            colors: root.colors
            title: I18n.tr("Features")
            width: parent.width

            RowToggle {
                colors: root.colors
                title: I18n.tr("Matugen")
                description: I18n.tr("Generate Material 3 colour schemes from the active wallpaper.")
                checked: Config.matugenEnabled
                onToggle: function(v) { if (root.saveConfigKey) root.saveConfigKey("features.matugen", v) }
            }

            RowToggle {
                colors: root.colors
                title: I18n.tr("Steam Workshop browser")
                description: I18n.tr("Browse and install Wallpaper Engine items from Steam Workshop.")
                checked: Config.steamEnabled
                onToggle: function(v) { if (root.saveConfigKey) root.saveConfigKey("features.steam", v) }
            }

            RowToggle {
                colors: root.colors
                title: I18n.tr("Wallhaven browser")
                description: I18n.tr("Browse and download wallpapers from wallhaven.cc.")
                checked: Config.wallhavenEnabled
                onToggle: function(v) { if (root.saveConfigKey) root.saveConfigKey("features.wallhaven", v) }
            }

            RowToggle {
                colors: root.colors
                title: I18n.tr("Mute wallpaper audio")
                description: I18n.tr("Silence video and Wallpaper Engine audio output.")
                checked: Config.wallpaperMute
                onToggle: function(v) {
                    if (root.saveConfigKey) root.saveConfigKey("wallpaperMute", v)
                    DaemonClient.setAudio(v, Config.wallpaperVolume)
                }
            }

            RowInput {
                colors: root.colors
                title: I18n.tr("Wallpaper volume")
                description: I18n.tr("Playback volume for video and Wallpaper Engine audio (0–100%). Has no effect while audio is muted.")
                value: Config.wallpaperVolume
                min: 0
                max: 100
                suffix: "%"
                enabled: !Config.wallpaperMute
                onCommit: function(v) {
                    if (root.saveConfigKey) root.saveConfigKey("wallpaperVolume", v)
                    DaemonClient.setAudio(Config.wallpaperMute, v)
                }
            }

            RowToggle {
                colors: root.colors
                title: I18n.tr("Auto-recolour new wallpapers")
                description: I18n.tr("When a new wallpaper is added, save a gowall theme-recoloured copy alongside it.")
                checked: Config.autoRecolorEnabled
                onToggle: function(v) { if (root.saveConfigKey) root.saveConfigKey("effects.autoRecolor", v) }
            }

            RowDropdown {
                colors: root.colors
                title: I18n.tr("Recolour theme")
                description: I18n.tr("Palette used when auto-recolouring new wallpapers.")
                value: Config.autoRecolorTheme
                model: root._recolorThemes
                enabled: Config.autoRecolorEnabled
                opacity: enabled ? 1.0 : 0.5
                onSelect: function(v) { if (root.saveConfigKey) root.saveConfigKey("effects.autoTheme", v) }
            }
        }

        SettingsCard {
            colors: root.colors
            title: I18n.tr("Random rotation")
            width: parent.width

            RowInput {
                colors: root.colors
                title: I18n.tr("Interval")
                description: I18n.tr("Seconds between random rotations.")
                value: Config.randomInterval
                min: 1; max: 86400
                onCommit: function(v) { if (root.saveConfigKey) root.saveConfigKey("general.randomInterval", v) }
            }

            RowToggle {
                colors: root.colors
                title: I18n.tr("Include images")
                description: I18n.tr("Allow static wallpapers in the random pool.")
                checked: Config.randomIncludeStatic
                onToggle: function(v) {
                    if (!v && !Config.randomIncludeVideo && !Config.randomIncludeWE) return
                    if (root.saveConfigKey) root.saveConfigKey("general.randomIncludeStatic", v)
                }
            }

            RowToggle {
                colors: root.colors
                title: I18n.tr("Include video")
                description: I18n.tr("Allow video wallpapers in the random pool.")
                checked: Config.randomIncludeVideo
                onToggle: function(v) {
                    if (!v && !Config.randomIncludeStatic && !Config.randomIncludeWE) return
                    if (root.saveConfigKey) root.saveConfigKey("general.randomIncludeVideo", v)
                }
            }

            RowToggle {
                colors: root.colors
                title: I18n.tr("Include Wallpaper Engine")
                description: I18n.tr("Allow Wallpaper Engine items in the random pool.")
                checked: Config.randomIncludeWE
                onToggle: function(v) {
                    if (!v && !Config.randomIncludeStatic && !Config.randomIncludeVideo) return
                    if (root.saveConfigKey) root.saveConfigKey("general.randomIncludeWE", v)
                }
            }

            RowToggle {
                colors: root.colors
                title: I18n.tr("Favourites only")
                description: I18n.tr("Restrict the random pool to favourited wallpapers.")
                checked: Config.randomIncludeFavourites
                onToggle: function(v) { if (root.saveConfigKey) root.saveConfigKey("general.randomIncludeFavourites", v) }
            }
        }

        SettingsCard {
            colors: root.colors
            title: I18n.tr("Day / night rotation")
            width: parent.width

            RowToggle {
                colors: root.colors
                title: I18n.tr("Alternate day and night clips")
                description: I18n.tr("Switch between a daytime and a night video pool at real sunrise/sunset, rotating within the active one.")
                checked: Config.dayNightEnabled
                onToggle: function(v) {
                    if (root.saveConfigKey) root.saveConfigKey("daynight.enabled", v)
                    if (v) DaemonClient.dayNightStart()
                    else DaemonClient.dayNightStop()
                }
            }

            RowTextInput {
                colors: root.colors
                title: I18n.tr("Day pool")
                description: I18n.tr("Folder of videos shown between sunrise and sunset.")
                value: Config.dayNightDayDir
                placeholder: I18n.tr("~/Pictures/Wallpapers/dynamic/Day")
                enabled: Config.dayNightEnabled
                opacity: enabled ? 1.0 : 0.5
                onCommit: function(v) { if (root.saveConfigKey) root.saveConfigKey("daynight.dayDir", v) }
            }

            RowTextInput {
                colors: root.colors
                title: I18n.tr("Night pool")
                description: I18n.tr("Folder of videos shown from sunset to sunrise.")
                value: Config.dayNightNightDir
                placeholder: I18n.tr("~/Pictures/Wallpapers/dynamic/Night")
                enabled: Config.dayNightEnabled
                opacity: enabled ? 1.0 : 0.5
                onCommit: function(v) { if (root.saveConfigKey) root.saveConfigKey("daynight.nightDir", v) }
            }

            RowInput {
                colors: root.colors
                title: I18n.tr("Rotate every (minutes)")
                description: I18n.tr("Minutes between clips within the active pool.")
                value: Config.dayNightInterval
                min: 1; max: 1440
                enabled: Config.dayNightEnabled
                opacity: enabled ? 1.0 : 0.5
                onCommit: function(v) {
                    if (root.saveConfigKey) root.saveConfigKey("daynight.rotateIntervalMinutes", v)
                    if (Config.dayNightEnabled) DaemonClient.dayNightStart()
                }
            }

            RowToggle {
                colors: root.colors
                title: I18n.tr("No repeat within a day")
                description: I18n.tr("Show every clip in a pool once before repeating; the set resets at the next sunrise.")
                checked: Config.dayNightNoRepeat
                enabled: Config.dayNightEnabled
                opacity: enabled ? 1.0 : 0.5
                onToggle: function(v) { if (root.saveConfigKey) root.saveConfigKey("daynight.noRepeatWithinDay", v) }
            }
        }
    }
}
