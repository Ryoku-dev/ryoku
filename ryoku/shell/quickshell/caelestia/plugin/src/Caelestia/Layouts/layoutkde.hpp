#pragma once

#include <qqmlintegration.h>

#include <QMarginsF>
#include <QObject>
#include <QQmlEngine>
#include <QRectF>
#include <QVariantList>
#include <QVariantMap>

namespace caelestia::layouts {

class LayoutKde : public QObject {
    Q_OBJECT
    QML_ELEMENT
    QML_SINGLETON

public:
    explicit LayoutKde(QObject* parent = nullptr);

    Q_INVOKABLE QVariantMap calculateLayout(
        const QVariantList& windows, double areaWidth, double areaHeight, double columnSpacing, double rowSpacing);

private:
    struct LayeredPacking {
        struct Layer {
            qreal maxWidth;
            qreal maxHeight;
            qreal remainingWidth;
            QList<size_t> ids;

            Layer(qreal mw, const QList<QRectF>& windowSizes, const QList<size_t>& windowIds, size_t startPos,
                size_t endPos);

            qreal width() const { return maxWidth - remainingWidth; }
        };

        qreal maxWidth;
        qreal width;
        qreal height;
        QList<Layer> layers;

        LayeredPacking(
            qreal mw, const QList<QRectF>& windowSizes, const QList<size_t>& ids, const QList<size_t>& layerStartPos);
    };

    static bool isDominated(size_t candidate, size_t alternativeSmall, size_t alternativeBig, size_t length,
        const std::function<qreal(size_t, size_t)>& leastWeightCandidate);
    static QList<size_t> getLayerStartPos(
        qreal maxWidth, qreal idealWidth, size_t length, const QList<qreal>& cumWidths);

    LayeredPacking findGoodPacking(const QRectF& area, const QList<QRectF>& windowSizes, const QList<QPointF>& centers,
        qreal idealWidthRatio, qreal tol);
    QList<QRectF> refineAndApplyPacking(const QRectF& area, const QMarginsF& margins, const LayeredPacking& packing,
        const QList<QRectF>& windowSizes, const QList<QPointF>& centers, qreal maxScale, qreal maxGapRatio);
};

} // namespace caelestia::layouts
