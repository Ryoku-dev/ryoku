#pragma once

#include <qdir.h>
#include <qstandardpaths.h>
#include <qstring.h>

#include "../Settings/objectnode.hpp"
#include "common.hpp"

namespace caelestia::config {

using Qt::StringLiterals::operator""_s;
using settings::vmap;

// The folder ryogami (Ryoku's wallpaper daemon) reads: the localized Pictures
// subfolder when it exists, else the English ~/Pictures one that a collection made
// before localization still lives in, else the localized path for a fresh library.
inline QString picturesSub(const QString& sub) {
    const auto localized = QStandardPaths::writableLocation(QStandardPaths::PicturesLocation) + u"/"_s + sub;
    if (QDir(localized).exists())
        return localized;
    const auto english = QDir::homePath() + u"/Pictures/"_s + sub;
    return QDir(english).exists() ? english : localized;
}

class UserPaths : public settings::ObjectNode {
    CONFIG_NODE(UserPaths, settings::ObjectNode)

    CONFIG_GLOBAL_PROPERTY(QString, wallpaperDir, picturesSub(u"Wallpapers"_s))
    CONFIG_GLOBAL_PROPERTY(
        QString, screenshotsDir, QStandardPaths::writableLocation(QStandardPaths::PicturesLocation) + u"/Screenshots"_s)
    CONFIG_GLOBAL_PROPERTY(
        QString, recordingsDir, QStandardPaths::writableLocation(QStandardPaths::MoviesLocation) + u"/Recordings"_s)
    CONFIG_PROPERTY(
        QString, cacheDir, QStandardPaths::writableLocation(QStandardPaths::GenericCacheLocation) + u"/ryoku/caelestia"_s)
    CONFIG_GLOBAL_PROPERTY(
        QString, lyricsDir, QStandardPaths::writableLocation(QStandardPaths::MusicLocation) + u"/Lyrics/"_s)
    CONFIG_PROPERTY(QString, sessionGif, u"root:/assets/kurukuru.gif"_s)
    CONFIG_PROPERTY(QString, mediaGif, u"root:/assets/bongocat.gif"_s)
    CONFIG_PROPERTY(QString, noNotifsPic, u"root:/assets/dino.png"_s)
    CONFIG_PROPERTY(QString, lockNoNotifsPic, u"root:/assets/dino.png"_s)
};

} // namespace caelestia::config
