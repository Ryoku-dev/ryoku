#pragma once

#include <qstring.h>

#include "../Settings/objectnode.hpp"
#include "common.hpp"

namespace caelestia::config {

class DesktopClockBackground : public settings::ObjectNode {
    CONFIG_NODE(DesktopClockBackground, settings::ObjectNode)

    CONFIG_PROPERTY(bool, enabled, false)
    CONFIG_PROPERTY(qreal, opacity, 0.7)
    CONFIG_PROPERTY(bool, blur, true)
};

class DesktopClockShadow : public settings::ObjectNode {
    CONFIG_NODE(DesktopClockShadow, settings::ObjectNode)

    CONFIG_PROPERTY(bool, enabled, true)
    CONFIG_PROPERTY(qreal, opacity, 0.7)
    CONFIG_PROPERTY(qreal, blur, 0.4)
};

class DesktopClock : public settings::ObjectNode {
    CONFIG_NODE(DesktopClock, settings::ObjectNode)

    CONFIG_PROPERTY(bool, invertColors, false)
    CONFIG_SUBOBJECT(DesktopClockBackground, background)
    CONFIG_SUBOBJECT(DesktopClockShadow, shadow)
};

class BackgroundVisualiser : public settings::ObjectNode {
    CONFIG_NODE(BackgroundVisualiser, settings::ObjectNode)

    CONFIG_PROPERTY(bool, enabled, true)
    CONFIG_PROPERTY(bool, autoHide, true)
    CONFIG_PROPERTY(bool, hideOnAllMonitors, false)
    CONFIG_PROPERTY(bool, blur, false)
    CONFIG_PROPERTY(qreal, rounding, 1)
    CONFIG_PROPERTY(qreal, spacing, 1)
    CONFIG_PROPERTY(qreal, size, 1)
};

class DesktopLyricsBackground : public settings::ObjectNode {
    CONFIG_NODE(DesktopLyricsBackground, settings::ObjectNode)

    CONFIG_PROPERTY(bool, enabled, false)
    CONFIG_PROPERTY(qreal, opacity, 0.7)
    CONFIG_PROPERTY(bool, blur, true)
};

class DesktopLyricsShadow : public settings::ObjectNode {
    CONFIG_NODE(DesktopLyricsShadow, settings::ObjectNode)

    CONFIG_PROPERTY(bool, enabled, true)
    CONFIG_PROPERTY(qreal, opacity, 0.7)
    CONFIG_PROPERTY(qreal, blur, 0.4)
};

class DesktopLyrics : public settings::ObjectNode {
    CONFIG_NODE(DesktopLyrics, settings::ObjectNode)

    CONFIG_PROPERTY(bool, autoHide, true)
    CONFIG_PROPERTY(int, alignment, 0)
    CONFIG_PROPERTY(bool, invertColors, false)
    CONFIG_SUBOBJECT(DesktopLyricsBackground, background)
    CONFIG_SUBOBJECT(DesktopLyricsShadow, shadow)
};

class DesktopShapes : public settings::ObjectNode {
    CONFIG_NODE(DesktopShapes, settings::ObjectNode)

    CONFIG_PROPERTY(bool, autoHide, true)
};

class BackgroundConfig : public settings::ObjectNode {
    CONFIG_NODE(BackgroundConfig, settings::ObjectNode)

    // The Ryoku shell owns the desktop (wallpaper, widgets, plugins); the Caelestia
    // background would cover them, so it stays off until asked for.
    CONFIG_PROPERTY(bool, enabled, false)
    CONFIG_PROPERTY(bool, wallpaperEnabled, true)
    CONFIG_PROPERTY(int, wallpaperFillMode, 2)
    CONFIG_PROPERTY(bool, wallpaperRecolor, false)
    CONFIG_PROPERTY(qreal, wallpaperRecolorStrength, 0.5)
    CONFIG_PROPERTY(bool, slideshowEnabled, false)
    CONFIG_PROPERTY(qreal, slideshowInterval, 0.16)
    CONFIG_PROPERTY(bool, slideshowRandom, true)
    CONFIG_PROPERTY(bool, videoWallpaperPaused, false)
    CONFIG_PROPERTY(bool, videoWallpaperSoundEnabled, false)
    CONFIG_PROPERTY(bool, videoWallpaperPauseOnFullscreen, false)
    CONFIG_PROPERTY(bool, videoWallpaperPauseOnTiled, false)
    CONFIG_PROPERTY(bool, videoWallpaperPauseOnAllDisplays, false)
    CONFIG_PROPERTY(bool, videoWallpaperMuteOnMedia, false)
    CONFIG_PROPERTY(bool, desktopIconsEnabled, true)
    CONFIG_PROPERTY(bool, materialYouIconsEnabled, true)
    CONFIG_PROPERTY(bool, materialYouIconsVibrant, true)
    CONFIG_SUBOBJECT(DesktopClock, desktopClock)
    CONFIG_SUBOBJECT(DesktopLyrics, desktopLyrics)
    CONFIG_SUBOBJECT(DesktopShapes, desktopShapes)
    CONFIG_SUBOBJECT(BackgroundVisualiser, visualiser)
};

} // namespace caelestia::config
