import QtQuick
import ".."
import "../.."
import "../../components"
import Ryoku.Ui.Singletons

Column {
    id: root
    property var colors
    property var saveConfigKey

    width: parent ? parent.width : 0
    spacing: 8

    // The 39 skwd shader transitions, grouped by the family taxonomy skwd-wall v2
    // carries (Fade / Wipe / Warp / Break): Fade dissolves and blends, Wipe sweeps
    // a shape or edge, Warp bends and ripples the coordinates, Break shatters or
    // pixelates. The nine gl-transition ports v2 dropped are classed by what their
    // math visibly does. Random stays first and ungrouped. The v2 Sand family is
    // absent: it is an instanced particle system the shell's ShaderEffect cannot run.
    readonly property var _shaderOptions: [
        { key: "random",             label: I18n.tr("Random"),             family: "" },
        { key: "colour-distance",    label: I18n.tr("Colour Distance"),    family: "fade" },
        { key: "crossfade",          label: I18n.tr("Crossfade"),          family: "fade" },
        { key: "fadecolor",          label: I18n.tr("Fadecolor"),          family: "fade" },
        { key: "overexposure",       label: I18n.tr("Overexposure"),       family: "fade" },
        { key: "perlin",             label: I18n.tr("Perlin"),             family: "fade" },
        { key: "pixelfade-wave",     label: I18n.tr("Pixelfade Wave"),     family: "fade" },
        { key: "soft-warp-fade",     label: I18n.tr("Soft Warp Fade"),     family: "fade" },
        { key: "static-fade",        label: I18n.tr("Static Fade"),        family: "fade" },
        { key: "bounce",             label: I18n.tr("Bounce"),             family: "wipe" },
        { key: "circle-crop",        label: I18n.tr("Circle Crop"),        family: "wipe" },
        { key: "crosshatch",         label: I18n.tr("Crosshatch"),         family: "wipe" },
        { key: "directional",        label: I18n.tr("Directional"),        family: "wipe" },
        { key: "directional-scaled", label: I18n.tr("Directional Scaled"), family: "wipe" },
        { key: "directional-wipe",   label: I18n.tr("Directional Wipe"),   family: "wipe" },
        { key: "edge-transition",    label: I18n.tr("Edge Transition"),    family: "wipe" },
        { key: "iris",               label: I18n.tr("Iris"),               family: "wipe" },
        { key: "polka-dots-curtain", label: I18n.tr("Polka Dots Curtain"), family: "wipe" },
        { key: "puzzle-right",       label: I18n.tr("Puzzle Right"),       family: "wipe" },
        { key: "randomsquares",      label: I18n.tr("Randomsquares"),      family: "wipe" },
        { key: "crazy-parametric",   label: I18n.tr("Crazy Parametric"),   family: "warp" },
        { key: "crosswarp",          label: I18n.tr("Cross Warp"),         family: "warp" },
        { key: "flyeye",             label: I18n.tr("Fly Eye"),            family: "warp" },
        { key: "heat-melt",          label: I18n.tr("Heat Melt"),          family: "warp" },
        { key: "liquid-ripple",      label: I18n.tr("Liquid Ripple"),      family: "warp" },
        { key: "morph",              label: I18n.tr("Morph"),              family: "warp" },
        { key: "polar-function",     label: I18n.tr("Polar Function"),     family: "warp" },
        { key: "wave-warp",          label: I18n.tr("Wave Warp"),          family: "warp" },
        { key: "zoom-blur-pull",     label: I18n.tr("Zoom Blur Pull"),     family: "warp" },
        { key: "chromatic-bloom",    label: I18n.tr("Chromatic Bloom"),    family: "break" },
        { key: "glitch",             label: I18n.tr("Glitch"),             family: "break" },
        { key: "glitch-displace",    label: I18n.tr("Glitch Displace"),    family: "break" },
        { key: "ink-splash",         label: I18n.tr("Ink Splash"),         family: "break" },
        { key: "inkwell-drop",       label: I18n.tr("Inkwell Drop"),       family: "break" },
        { key: "mosaic-tumble",      label: I18n.tr("Mosaic Tumble"),      family: "break" },
        { key: "parametric-glitch",  label: I18n.tr("Parametric Glitch"),  family: "break" },
        { key: "pixelate",           label: I18n.tr("Pixelate"),           family: "break" },
        { key: "plasma-flow",        label: I18n.tr("Plasma Flow"),        family: "break" },
        { key: "smoke",              label: I18n.tr("Smoke"),              family: "break" },
        { key: "voronoi-shatter",    label: I18n.tr("Voronoi Shatter"),    family: "break" }
    ]

    // Family display names in dropdown order; the picker draws a header per family.
    readonly property var _families: [
        { key: "fade",  label: I18n.tr("Fade") },
        { key: "wipe",  label: I18n.tr("Wipe") },
        { key: "warp",  label: I18n.tr("Warp") },
        { key: "break", label: I18n.tr("Break up") }
    ]

    SettingsCard {
        colors: root.colors
        title: I18n.tr("Engine")
        subtitle: I18n.tr("Pixels are painted by the built-in path: the shell renders stills with reveal transitions and plays video in-shell.")

        SettingsRow {
            colors: root.colors
            title: I18n.tr("Wallpaper engine")
            description: I18n.tr("One engine, built in. External painters from the skwd lineage are gone.")
            FilterButton {
                colors: root.colors
                label: I18n.tr("Built-in")
                skew: 8 * Config.uiScale; height: 26 * Config.uiScale
                isActive: true
            }
        }

        SettingsRow {
            colors: root.colors
            title: I18n.tr("Fill mode")
            description: I18n.tr("How the wallpaper is fitted to the screen. Applies to images and videos.")
            Row {
                spacing: 4
                Repeater {
                    model: [
                        { key: "fill",    label: I18n.tr("Fill") },
                        { key: "fit",     label: I18n.tr("Fit") },
                        { key: "stretch", label: I18n.tr("Stretch") },
                        { key: "center",  label: I18n.tr("Center") },
                        { key: "tile",    label: I18n.tr("Tile") }
                    ]
                    FilterButton {
                        colors: root.colors
                        label: I18n.tr(modelData.label)
                        skew: 8 * Config.uiScale; height: 26 * Config.uiScale
                        isActive: Config.fillMode === modelData.key
                        onClicked: Config.saveKey("display.fillMode", modelData.key)
                    }
                }
            }
        }
    }

    SettingsCard {
        colors: root.colors
        title: I18n.tr("Live wallpapers")
        subtitle: I18n.tr("How video wallpapers play: the default engine decodes a cached transcode through ryogami-live; the in-shell engine decodes the clip through the shell.")

        SettingsRow {
            colors: root.colors
            title: I18n.tr("Video engine")
            description: I18n.tr("ryogami = the C player (cached transcode, low resources); in-shell = the shell's QtMultimedia player.")
            Row {
                spacing: 4
                Repeater {
                    model: [
                        { key: "ryogami",  label: "ryogami" },
                        { key: "in_shell", label: "in-shell" }
                    ]
                    FilterButton {
                        colors: root.colors
                        label: I18n.tr(modelData.label)
                        skew: 8 * Config.uiScale; height: 26 * Config.uiScale
                        isActive: Config.engineVideoEngine === modelData.key
                        onClicked: Config._shellSet("wallpaper.video_engine", modelData.key)
                    }
                }
            }
        }

        RowToggle {
            colors: root.colors
            title: I18n.tr("Video wallpapers")
            description: I18n.tr("Off keeps only the wallpaper still and plays no clip at all, the cheapest option.")
            checked: Config.engineVideoEnabled
            onToggle: function(v) { Config._shellSet("wallpaper.video_enabled", v) }
        }

        RowToggle {
            colors: root.colors
            title: I18n.tr("Cache a re-encode")
            description: I18n.tr("Re-encode the clip once to a bite-sized cached mp4 (capped fps + width) and play that instead of decoding the full-resolution source live.")
            checked: Config.engineVideoTranscode
            onToggle: function(v) { Config._shellSet("wallpaper.video_transcode", v) }
        }

        RowInput {
            colors: root.colors
            title: I18n.tr("Re-encode fps")
            description: I18n.tr("The frame-rate cap for the cached re-encode. Lower decodes fewer frames.")
            value: Config.engineVideoTransFps
            min: 1; max: 120
            onCommit: function(v) { Config._shellSet("wallpaper.video_transcode_fps", Math.round(v)) }
        }

        RowInput {
            colors: root.colors
            title: I18n.tr("Re-encode width")
            description: I18n.tr("The width cap for the cached re-encode. The engine never decodes more pixels; a 4K clip plays at 1920 by default.")
            value: Config.engineVideoTransWidth
            min: 640; max: 7680
            onCommit: function(v) { Config._shellSet("wallpaper.video_transcode_width", Math.round(v)) }
        }
    }

    SettingsCard {
        colors: root.colors
        title: I18n.tr("Transitions")
        subtitle: I18n.tr("The 39 skwd shader transitions, grouped into Fade, Wipe, Warp and Break up and rendered by the shell on every switch. Random rotates them with no repeats; picking a shader pins it. The shell's own 22 reveal presets stay reachable by setting transition.shader to \"ryoku\".")

        RowToggle {
            colors: root.colors
            title: I18n.tr("Enable transitions")
            description: I18n.tr("Animate wallpaper switches; off means a plain cut.")
            checked: Config.transitionEnabled
            onToggle: function(v) { Config.saveKey("transition.enabled", v) }
        }

        RowInput {
            colors: root.colors
            title: I18n.tr("Duration (ms)")
            description: I18n.tr("Transition length in milliseconds.")
            value: Config.transitionDurationMs
            min: 100; max: 10000
            onCommit: function(v) { Config.saveKey("transition.durationMs", v) }
        }

        RowToggle {
            colors: root.colors
            title: I18n.tr("Random shader per transition")
            description: I18n.tr("Pick a different shader for every transition.")
            checked: Config.transitionShader === "random"
            onToggle: function(v) {
                if (v) {
                    if (Config.transitionShader !== "random" && root.saveConfigKey)
                        root.saveConfigKey("transition.lastShader", Config.transitionShader)
                    if (root.saveConfigKey) root.saveConfigKey("transition.shader", "random")
                } else {
                    var fallback = (Config._data.transition && Config._data.transition.lastShader) || "morph"
                    if (fallback === "random") fallback = "morph"
                    if (root.saveConfigKey) root.saveConfigKey("transition.shader", fallback)
                }
            }
        }

        ShaderPicker {
            colors: root.colors
            model: root._shaderOptions.filter(function(s) { return s.key !== "random" })
            families: root._families
            value: Config.transitionShader
            enabled: Config.transitionEnabled && Config.transitionShader !== "random"
            opacity: (Config.transitionEnabled && Config.transitionShader !== "random") ? 1.0 : 0.4
            onSelected: function(key) { if (root.saveConfigKey) root.saveConfigKey("transition.shader", key) }
        }
    }

}
