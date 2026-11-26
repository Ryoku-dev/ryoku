#pragma once

#include "../scene/layout.h"

#include <QString>

// Deck is drawn by the QML DeckScene so it can render the paint-chip cards
// exactly. This layout only keeps CardField's selection state and navigation
// contract intact while that scene is active.
class DeckLayout final : public Layout
{
public:
    QString key() const override { return QStringLiteral("deck"); }

    void configure(const LayoutContext &ctx, bool animate) override;
    void reset(const LayoutContext &ctx) override;
    void select(const LayoutContext &ctx, int from, int to) override;
    bool tick(const LayoutContext &ctx, double dt) override;
    void build(const LayoutContext &ctx, std::vector<CardVisual> &out) override;

    int hitTest(QPointF point) const override;
    QRectF cardRect(int row) const override;
    int step(const LayoutContext &ctx, int dx, int dy) const override;
    int wheel(const LayoutContext &ctx, QPointF angle, QPointF pixels) override;

    double cameraMs(const LayoutContext &ctx) const override;

private:
    int m_scrollAccum = 0;
};
