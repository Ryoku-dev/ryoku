#pragma once

#include <qobject.h>
#include <qqmlintegration.h>

namespace caelestia::config {

#define ENUM(Name, ...)                                                                                                \
    namespace Name {                                                                                                   \
                                                                                                                       \
    Q_NAMESPACE                                                                                                        \
    QML_ELEMENT                                                                                                        \
                                                                                                                       \
    enum Enum {                                                                                                        \
        __VA_ARGS__                                                                                                    \
    };                                                                                                                 \
    Q_ENUM_NS(Enum)                                                                                                    \
                                                                                                                       \
    };

ENUM(BarWorkspaceDisplay, Shapes, Text)
ENUM(BarWorkspaceCapitalisation, Preserve, Upper, Lower)
ENUM(DockWindowGrouping, Combined, PinnedFirst, Ungrouped)
ENUM(LauncherBrowseLayout, Default, Simple, Compact)
ENUM(LauncherNameOverflow, Tooltip, Shrink, Flip)
ENUM(LyricsBackend, Auto, Local, LRCLIB, NetEase)
ENUM(GpuType, Auto, Nvidia, Generic, None)
ENUM(NotifsFullscreen, On, Off)
ENUM(NotifsMonitor, All, Focused)
ENUM(TemperatureUnit, Auto, Celsius, Fahrenheit, Kelvin)
ENUM(DataUnit, Binary, Decimal)
ENUM(ClockFormat, Auto, TwelveHour, TwentyFourHour)
ENUM(VisualiserInput, Output, Input)

#undef ENUM

} // namespace caelestia::config
