#pragma once

#include <qqmlintegration.h>

#include <QObject>
#include <QQmlEngine>
#include <QVariantList>
#include <QVariantMap>

namespace caelestia::layouts {

class LayoutGnome : public QObject {
    Q_OBJECT
    QML_ELEMENT
    QML_SINGLETON

public:
    explicit LayoutGnome(QObject* parent = nullptr);

    Q_INVOKABLE QVariantMap calculateLayout(
        const QVariantList& windows, double areaWidth, double areaHeight, double columnSpacing, double rowSpacing);

private:
    struct WindowInfo {
        QString address;
        double x;
        double y;
        double width;
        double height;
        double center_x;
        double center_y;
    };

    struct RowInfo {
        double x = 0;
        double y = 0;
        double width = 0;
        double height = 0;
        double fullWidth = 0;
        double fullHeight = 0;
        double additionalScale = 1.0;
        QList<WindowInfo> windows;
    };

    double computeWindowScale(const WindowInfo& win, double areaHeight);
    bool keepSameRow(const RowInfo& row, double width, double idealRowWidth);
    bool isBetterScaleAndSpace(double oldScale, double oldSpace, double scale, double space);
};

} // namespace caelestia::layouts
