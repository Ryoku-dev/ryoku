#include "layoutkde.hpp"

#include <algorithm>
#include <cmath>
#include <deque>

#include "layoututils.hpp"

namespace caelestia::layouts {

LayoutKde::LayoutKde(QObject* parent)
    : QObject(parent) {}

LayoutKde::LayeredPacking::Layer::Layer(
    qreal mw, const QList<QRectF>& windowSizes, const QList<size_t>& windowIds, size_t startPos, size_t endPos)
    : maxWidth(mw)
    , ids(windowIds.begin() + startPos, windowIds.begin() + endPos) {
    maxHeight = windowSizes[windowIds[endPos - 1]].height();
    remainingWidth = maxWidth;
    for (auto id : ids) {
        remainingWidth -= windowSizes[id].width();
    }
}

LayoutKde::LayeredPacking::LayeredPacking(
    qreal mw, const QList<QRectF>& windowSizes, const QList<size_t>& ids, const QList<size_t>& layerStartPos)
    : maxWidth(mw)
    , width(0)
    , height(0) {
    for (int i = 1; i < layerStartPos.size(); ++i) {
        layers.emplace_back(maxWidth, windowSizes, ids, layerStartPos[i - 1], layerStartPos[i]);
        width = std::max(width, layers.back().width());
        height += layers.back().maxHeight;
    }
}

bool LayoutKde::isDominated(size_t candidate, size_t alternativeSmall, size_t alternativeBig, size_t length,
    const std::function<qreal(size_t, size_t)>& leastWeightCandidate) {
    if (alternativeBig == length)
        return true;
    if (leastWeightCandidate(alternativeSmall, length) <= leastWeightCandidate(candidate, length)) {
        return true;
    }
    size_t low = alternativeBig;
    size_t high = length;
    while (high - low >= 2) {
        size_t mid = (low + high) / 2;
        if (leastWeightCandidate(alternativeSmall, mid) <= leastWeightCandidate(candidate, mid)) {
            low = mid;
        } else {
            high = mid;
        }
    }
    return (leastWeightCandidate(alternativeBig, high) <= leastWeightCandidate(candidate, high));
}

QList<size_t> LayoutKde::getLayerStartPos(
    qreal maxWidth, qreal idealWidth, size_t length, const QList<qreal>& cumWidths) {
    auto weight = [maxWidth, idealWidth, &cumWidths](size_t start, size_t end) {
        qreal width = cumWidths[end] - cumWidths[start];
        if (width < idealWidth) {
            return (width - idealWidth) * (width - idealWidth) / idealWidth / idealWidth;
        } else {
            qreal penaltyFactor = cumWidths.size();
            return penaltyFactor * (width - idealWidth) * (width - idealWidth) / (maxWidth - idealWidth) /
                   (maxWidth - idealWidth);
        }
    };

    QList<size_t> layerStart(length + 1);
    QList<qreal> leastWeight(length + 1);
    std::deque<size_t> layerStartCandidates;

    leastWeight[0] = 0;

    auto leastWeightCandidate = [&leastWeight, &weight](size_t lastRowStartPos, size_t num) {
        return leastWeight[lastRowStartPos] + weight(lastRowStartPos, num);
    };

    layerStartCandidates.push_back(0);
    for (size_t currentIndex = 1; currentIndex < length; ++currentIndex) {
        leastWeight[currentIndex] = leastWeightCandidate(layerStartCandidates.front(), currentIndex);
        layerStart[currentIndex] = layerStartCandidates.front();
        layerStartCandidates.push_back(currentIndex);
        while (
            layerStartCandidates.size() >= 2 && leastWeightCandidate(layerStartCandidates[1], currentIndex + 1) <=
                                                    leastWeightCandidate(layerStartCandidates[0], currentIndex + 1)) {
            layerStartCandidates.pop_front();
        }
        layerStartCandidates.pop_back();
        while (layerStartCandidates.size() >= 2 &&
               isDominated(layerStartCandidates.back(), layerStartCandidates[layerStartCandidates.size() - 2],
                   currentIndex, length, leastWeightCandidate)) {
            layerStartCandidates.pop_back();
        }
        if (layerStartCandidates.empty()) {
            layerStartCandidates.push_back(currentIndex);
            continue;
        }
        if (leastWeightCandidate(currentIndex, length) < leastWeightCandidate(layerStartCandidates.back(), length)) {
            layerStartCandidates.push_back(currentIndex);
        }
    }

    leastWeight[length] = leastWeightCandidate(layerStartCandidates.front(), length);
    layerStart[length] = layerStartCandidates.front();

    QList<size_t> layerStartPosReversed;
    layerStartPosReversed.push_back(length);
    size_t currentIndex = length;
    while (currentIndex > 0) {
        currentIndex = layerStart[currentIndex];
        layerStartPosReversed.push_back(currentIndex);
    }

    QList<size_t> result;
    for (auto it = layerStartPosReversed.rbegin(); it != layerStartPosReversed.rend(); ++it) {
        result.push_back(*it);
    }
    return result;
}

LayoutKde::LayeredPacking LayoutKde::findGoodPacking(const QRectF& area, const QList<QRectF>& windowSizes,
    const QList<QPointF>& centers, qreal idealWidthRatio, qreal tol) {
    QList<std::tuple<size_t, QRectF, QPointF>> windowSizesWithIds;
    for (int i = 0; i < windowSizes.size(); ++i) {
        windowSizesWithIds.emplace_back(i, windowSizes[i], centers[i]);
    }

    std::stable_sort(windowSizesWithIds.begin(), windowSizesWithIds.end(), [](const auto& a, const auto& b) {
        if (std::abs(std::get<1>(a).height() - std::get<1>(b).height()) > 0.1) {
            return std::get<1>(a).height() < std::get<1>(b).height();
        }
        return std::get<2>(a).y() < std::get<2>(b).y();
    });

    QList<size_t> ids;
    QList<qreal> cumWidths;
    qreal stripWidthMin = 0;
    qreal stripWidthMax = 0;

    cumWidths.push_back(0);
    for (const auto& w : windowSizesWithIds) {
        ids.push_back(std::get<0>(w));
        qreal width = std::get<1>(w).width();
        cumWidths.push_back(cumWidths.back() + width);
        stripWidthMin = std::max(stripWidthMin, width);
        stripWidthMax += width;
    }
    stripWidthMin /= idealWidthRatio;
    stripWidthMax /= idealWidthRatio;

    qreal targetRatio = area.height() / area.width();

    auto findPacking = [&](qreal stripWidth) {
        QList<size_t> layerStartPos = getLayerStartPos(stripWidth, stripWidth * idealWidthRatio, ids.size(), cumWidths);
        return LayeredPacking(stripWidth, windowSizes, ids, layerStartPos);
    };

    LayeredPacking placementWidthMin = findPacking(stripWidthMin);
    qreal ratioHigh = placementWidthMin.height / placementWidthMin.width;
    if (ratioHigh <= targetRatio)
        return placementWidthMin;

    LayeredPacking placementWidthMax = findPacking(stripWidthMax);
    qreal ratioLow = placementWidthMax.height / placementWidthMax.width;
    if (ratioLow >= targetRatio)
        return placementWidthMax;

    while (stripWidthMax / stripWidthMin > 1 + tol) {
        qreal stripWidthMid = std::sqrt(stripWidthMin * stripWidthMax);
        LayeredPacking placementMid = findPacking(stripWidthMid);
        qreal ratioMid = placementMid.height / placementMid.width;

        if (ratioMid > targetRatio) {
            stripWidthMin = stripWidthMid;
            placementWidthMin = placementMid;
            ratioHigh = ratioMid;
        } else {
            stripWidthMax = placementMid.width;
            placementWidthMax = placementMid;
            ratioLow = ratioMid;
        }
    }

    qreal scaleWidthMin = std::min(area.width() / placementWidthMin.width, area.height() / placementWidthMin.height);
    qreal scaleWidthMax = std::min(area.width() / placementWidthMax.width, area.height() / placementWidthMax.height);

    if (scaleWidthMin > scaleWidthMax) {
        return placementWidthMin;
    }
    return placementWidthMax;
}

QList<QRectF> LayoutKde::refineAndApplyPacking(const QRectF& area, const QMarginsF& margins,
    const LayeredPacking& packing, const QList<QRectF>& windowSizes, const QList<QPointF>& centers, qreal maxScale,
    qreal maxGapRatio) {
    qreal scale = std::min(area.width() / packing.width, area.height() / packing.height);
    scale = std::min(scale, maxScale);

    QMarginsF scaledMargins(
        margins.left() * scale, margins.top() * scale, margins.right() * scale, margins.bottom() * scale);

    qreal maxGapY = maxGapRatio * (scaledMargins.top() + scaledMargins.bottom());
    qreal maxGapX = maxGapRatio * (scaledMargins.left() + scaledMargins.right());

    qreal extraY = area.height() - packing.height * scale;
    qreal gapY = std::min(maxGapY, extraY / (packing.layers.size() + 1));
    qreal y = area.y() + (extraY - gapY * (packing.layers.size() - 1)) / 2;

    QList<QRectF> finalWindowLayouts = windowSizes;
    for (const auto& layer : packing.layers) {
        qreal extraX = area.width() - layer.width() * scale;
        qreal gapX = std::min(maxGapX, extraX / (layer.ids.size() + 1));
        qreal x = area.x() + (extraX - gapX * (layer.ids.size() - 1)) / 2;

        QList<size_t> ids = layer.ids;
        std::stable_sort(ids.begin(), ids.end(), [&centers](size_t a, size_t b) {
            return centers[a].x() < centers[b].x();
        });
        for (auto id : ids) {
            QRectF& windowLayout = finalWindowLayouts[id];
            qreal newY = y + (layer.maxHeight - windowLayout.height()) * scale / 2;
            windowLayout = QRectF(x, newY, windowLayout.width() * scale, windowLayout.height() * scale);
            x += windowLayout.width() + gapX;
            windowLayout = windowLayout.marginsRemoved(scaledMargins);
        }
        y += layer.maxHeight * scale + gapY;
    }
    return finalWindowLayouts;
}

QVariantMap LayoutKde::calculateLayout(
    const QVariantList& windows, double areaWidth, double areaHeight, double columnSpacing, double rowSpacing) {
    QVariantMap result;
    if (windows.isEmpty() || areaWidth <= 0 || areaHeight <= 0)
        return result;

    QList<QString> addresses;
    QList<QRectF> windowSizes;
    QList<QPointF> centers;

    QMarginsF margins(columnSpacing / 2.0, rowSpacing / 2.0, columnSpacing / 2.0, rowSpacing / 2.0);

    for (const QVariant& wVar : windows) {
        QVariantMap w = wVar.toMap();
        QString addr = w.value(QStringLiteral("address")).toString();
        qreal wx = w.value(QStringLiteral("x")).toDouble();
        qreal wy = w.value(QStringLiteral("y")).toDouble();
        qreal ww = w.value(QStringLiteral("width"), 800).toDouble();
        qreal wh = w.value(QStringLiteral("height"), 600).toDouble();

        addresses.append(addr);

        QRectF rect(0, 0, ww, wh);
        rect = rect.marginsAdded(margins);
        windowSizes.append(rect);
        centers.append(QPointF(wx + ww / 2.0, wy + wh / 2.0));
    }

    QRectF area(0, 0, areaWidth, areaHeight);

    qreal idealWidthRatio = 0.5;
    qreal tol = 0.05;
    qreal maxScale = 1.0;
    qreal maxGapRatio = 2.0;

    LayeredPacking bestPacking = findGoodPacking(area, windowSizes, centers, idealWidthRatio, tol);
    QList<QRectF> layouts =
        refineAndApplyPacking(area, margins, bestPacking, windowSizes, centers, maxScale, maxGapRatio);

    for (int i = 0; i < layouts.size(); ++i) {
        QVariantMap props;
        props[QStringLiteral("x")] = layouts[i].x();
        props[QStringLiteral("y")] = layouts[i].y();
        props[QStringLiteral("width")] = layouts[i].width();
        props[QStringLiteral("height")] = layouts[i].height();
        result[addresses[i]] = props;
    }

    centreLayout(result, areaWidth, areaHeight);

    return result;
}

} // namespace caelestia::layouts
