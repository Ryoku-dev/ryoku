#pragma once

#include <QColor>
#include <QObject>
#include <QSizeF>
#include <QString>

class CardSourceNotifier;

class CardSource
{
public:
    enum Badge : unsigned {
        BadgeNone = 0,
        BadgeFavourite = 1u << 0,
        BadgeVideo = 1u << 1,
        BadgeScene = 1u << 2,     // Wallpaper Engine scene
        BadgeDepth = 1u << 3,     // Ryostage depth or parallax on
        BadgeDownloaded = 1u << 4,
        BadgeApplied = 1u << 5,
    };

    virtual ~CardSource() = default;

    virtual int cardCount() const = 0;
    // Stable across refilters; drives texture caching and selection restore.
    virtual QString cardKey(int row) const = 0;
    // Image decoded for the far tier and while the near image loads.
    virtual QString cardThumb(int row) const = 0;
    // The original for static images, the large thumbnail otherwise.
    virtual QString cardFullImage(int row) const = 0;
    // Source aspect (width / height); the scene cover-crops with it.
    virtual QSizeF cardImageSize(int row) const = 0;
    // Placeholder colour while nothing is decoded (the dominant colour).
    virtual QColor cardFill(int row) const = 0;
    virtual unsigned cardBadges(int row) const = 0;
    virtual QString cardPreviewVideo(int row) const = 0;
    virtual int rowOfKey(const QString &key) const = 0;
    // Bumps whenever rows change so the scene can run its filter-swap roll.
    virtual quint64 cardGeneration() const = 0;
    // Lets the scene observe row changes without knowing the concrete type.
    virtual CardSourceNotifier *cardNotifier() const = 0;
};

class CardSourceNotifier : public QObject
{
    Q_OBJECT
public:
    using QObject::QObject;
Q_SIGNALS:
    void cardsChanged();
    void cardsAppended();
    void cardUpdated(int row);
};
