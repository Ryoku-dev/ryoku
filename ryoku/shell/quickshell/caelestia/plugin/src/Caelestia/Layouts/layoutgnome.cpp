#include "layoutgnome.hpp"

#include <algorithm>
#include <cmath>

#include "layoututils.hpp"

namespace caelestia::layouts {

LayoutGnome::LayoutGnome(QObject* parent)
    : QObject(parent) {}

inline double lerp(double a, double b, double t) {
    return a + (b - a) * std::clamp(t, 0.0, 1.0);
}

double LayoutGnome::computeWindowScale(const WindowInfo& win, double areaHeight) {
    double ratio = win.height / std::max(1.0, areaHeight);
    return lerp(1.5, 1.0, ratio);
}

bool LayoutGnome::keepSameRow(const RowInfo& row, double width, double idealRowWidth) {
    if (row.fullWidth + width <= idealRowWidth)
        return true;
    double oldRatio = row.fullWidth / idealRowWidth;
    double newRatio = (row.fullWidth + width) / idealRowWidth;
    if (std::abs(1.0 - newRatio) < std::abs(1.0 - oldRatio))
        return true;
    return false;
}

bool LayoutGnome::isBetterScaleAndSpace(double oldScale, double oldSpace, double scale, double space) {
    const double LAYOUT_SCALE_WEIGHT = 1.0;
    const double LAYOUT_SPACE_WEIGHT = 0.1;
    double spacePower = (space - oldSpace) * LAYOUT_SPACE_WEIGHT;
    double scalePower = (scale - oldScale) * LAYOUT_SCALE_WEIGHT;

    if (scale > oldScale && space > oldSpace) {
        return true;
    } else if (scale > oldScale && space <= oldSpace) {
        return scalePower > -spacePower;
    } else if (scale <= oldScale && space > oldSpace) {
        return spacePower > -scalePower;
    } else {
        return false;
    }
}

QVariantMap LayoutGnome::calculateLayout(
    const QVariantList& windows, double areaWidth, double areaHeight, double columnSpacing, double rowSpacing) {
    QVariantMap result;
    if (windows.isEmpty() || areaWidth <= 0 || areaHeight <= 0)
        return result;

    QList<WindowInfo> winInfos;
    for (const QVariant& wVar : windows) {
        QVariantMap w = wVar.toMap();
        WindowInfo info;
        info.address = w.value(QStringLiteral("address")).toString();
        info.x = w.value(QStringLiteral("x")).toDouble();
        info.y = w.value(QStringLiteral("y")).toDouble();
        info.width = w.value(QStringLiteral("width"), 800).toDouble();
        info.height = w.value(QStringLiteral("height"), 600).toDouble();
        info.center_x = info.x + info.width / 2.0;
        info.center_y = info.y + info.height / 2.0;
        winInfos.append(info);
    }

    std::sort(winInfos.begin(), winInfos.end(), [](const WindowInfo& a, const WindowInfo& b) {
        return a.center_y < b.center_y;
    });

    QList<RowInfo> bestRowsData;
    double bestScale = 0.0;
    double bestSpace = 0.0;
    int lastNumColumns = -1;

    for (int numRows = 1;; ++numRows) {
        int numColumns = std::ceil((double)winInfos.size() / numRows);
        if (numColumns == lastNumColumns)
            break;

        double totalWidth = 0;
        for (const auto& win : winInfos) {
            totalWidth += win.width * computeWindowScale(win, areaHeight);
        }
        double idealRowWidth = totalWidth / numRows;

        QList<RowInfo> rows(numRows);
        int windowIdx = 0;
        for (int i = 0; i < numRows; ++i) {
            RowInfo& row = rows[i];
            for (; windowIdx < winInfos.size(); ++windowIdx) {
                const auto& win = winInfos[windowIdx];
                double s = computeWindowScale(win, areaHeight);
                double w = win.width * s;
                double h = win.height * s;
                row.fullHeight = std::max(row.fullHeight, h);

                if (keepSameRow(row, w, idealRowWidth) || i == numRows - 1) {
                    row.windows.append(win);
                    row.fullWidth += w;
                } else {
                    break;
                }
            }
        }

        double gridWidth = 0;
        double gridHeight = 0;
        int maxCols = 0;
        for (int i = 0; i < numRows; ++i) {
            RowInfo& row = rows[i];
            std::sort(row.windows.begin(), row.windows.end(), [](const WindowInfo& a, const WindowInfo& b) {
                return a.center_x < b.center_x;
            });
            gridWidth = std::max(gridWidth, row.fullWidth);
            gridHeight += row.fullHeight;
            maxCols = std::max(maxCols, (int)row.windows.size());
        }

        double hspacing = std::max(0, maxCols - 1) * columnSpacing;
        double vspacing = std::max(0, numRows - 1) * rowSpacing;

        double horizontalScale = (areaWidth - hspacing) / std::max(1.0, gridWidth);
        double verticalScale = (areaHeight - vspacing) / std::max(1.0, gridHeight);
        double scale = std::min({ horizontalScale, verticalScale, 0.95 });

        double scaledLayoutWidth = gridWidth * scale + hspacing;
        double scaledLayoutHeight = gridHeight * scale + vspacing;
        double space = (scaledLayoutWidth * scaledLayoutHeight) / (areaWidth * areaHeight);

        if (bestRowsData.size() > 0 && !isBetterScaleAndSpace(bestScale, bestSpace, scale, space)) {
            break;
        }

        bestRowsData = rows;
        bestScale = scale;
        bestSpace = space;
        lastNumColumns = numColumns;
    }

    for (RowInfo& row : bestRowsData) {
        row.width = row.fullWidth * bestScale + std::max(0, (int)row.windows.size() - 1) * columnSpacing;
        row.height = row.fullHeight * bestScale;
    }

    double heightWithoutSpacing = 0;
    for (const RowInfo& row : bestRowsData)
        heightWithoutSpacing += row.height;
    double verticalSpacing = std::max(0, (int)bestRowsData.size() - 1) * rowSpacing;
    double additionalVerticalScale =
        std::min(1.0, (areaHeight - verticalSpacing) / std::max(1.0, heightWithoutSpacing));

    double compensation = 0;
    double currentY = 0;

    for (RowInfo& row : bestRowsData) {
        double horizontalSpacing = std::max(0, (int)row.windows.size() - 1) * columnSpacing;
        double widthWithoutSpacing = row.width - horizontalSpacing;
        double additionalHorizontalScale =
            std::min(1.0, (areaWidth - horizontalSpacing) / std::max(1.0, widthWithoutSpacing));

        if (additionalHorizontalScale < additionalVerticalScale) {
            row.additionalScale = additionalHorizontalScale;
            compensation += (additionalVerticalScale - additionalHorizontalScale) * row.height;
        } else {
            row.additionalScale = additionalVerticalScale;
        }

        row.x = std::max(0.0, areaWidth - (widthWithoutSpacing * row.additionalScale + horizontalSpacing)) / 2.0;
        row.y = std::max(0.0, areaHeight - (heightWithoutSpacing + verticalSpacing)) / 2.0 + currentY;
        currentY += row.height * row.additionalScale + rowSpacing;
    }
    compensation /= 2.0;

    for (const RowInfo& row : bestRowsData) {
        double rowY = row.y + compensation;
        double rowHeight = row.height * row.additionalScale;
        double currentX = row.x;

        for (const WindowInfo& win : row.windows) {
            double s = bestScale * computeWindowScale(win, areaHeight) * row.additionalScale;
            double cellWidth = win.width * s;
            double cellHeight = win.height * s;

            s = std::min(s, 0.95);
            double cloneWidth = win.width * s;
            double cloneHeight = win.height * s;

            double cloneX = currentX + (cellWidth - cloneWidth) / 2.0;
            double cloneY = 0;
            if (bestRowsData.size() == 1) {
                cloneY = rowY + (rowHeight - cloneHeight) / 2.0;
            } else {
                cloneY = rowY + rowHeight - cellHeight;
            }

            QVariantMap props;
            props[QStringLiteral("x")] = std::floor(cloneX);
            props[QStringLiteral("y")] = std::floor(cloneY);
            props[QStringLiteral("width")] = cloneWidth;
            props[QStringLiteral("height")] = cloneHeight;
            result[win.address] = props;

            currentX += cellWidth + columnSpacing;
        }
    }

    centreLayout(result, areaWidth, areaHeight);

    return result;
}

} // namespace caelestia::layouts
