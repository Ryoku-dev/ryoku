#include "decklayout.h"

#include <algorithm>
#include <cmath>

void DeckLayout::configure(const LayoutContext &ctx, bool animate)
{
    Q_UNUSED(ctx)
    Q_UNUSED(animate)
}

void DeckLayout::reset(const LayoutContext &ctx)
{
    Q_UNUSED(ctx)
    m_scrollAccum = 0;
}

void DeckLayout::select(const LayoutContext &ctx, int from, int to)
{
    Q_UNUSED(ctx)
    Q_UNUSED(from)
    Q_UNUSED(to)
}

bool DeckLayout::tick(const LayoutContext &ctx, double dt)
{
    Q_UNUSED(ctx)
    Q_UNUSED(dt)
    return false;
}

void DeckLayout::build(const LayoutContext &ctx, std::vector<CardVisual> &out)
{
    Q_UNUSED(ctx)
    Q_UNUSED(out)
}

int DeckLayout::hitTest(QPointF point) const
{
    Q_UNUSED(point)
    return -1;
}

QRectF DeckLayout::cardRect(int row) const
{
    return m_rects.value(row);
}

void DeckLayout::setExternalRect(int row, const QRectF &rect)
{
    if (row < 0)
        return;
    if (rect.isEmpty())
        m_rects.remove(row);
    else
        m_rects.insert(row, rect);
}

int DeckLayout::step(const LayoutContext &ctx, int dx, int dy) const
{
    Q_UNUSED(dy)
    const int last = ctx.count > 0 ? ctx.count - 1 : 0;
    const int next = ctx.current + (dx < 0 ? -1 : (dx > 0 ? 1 : 0));
    return std::clamp(next, 0, last);
}

int DeckLayout::wheel(const LayoutContext &ctx, QPointF angle, QPointF pixels)
{
    Q_UNUSED(ctx)
    const double amount = pixels.y() != 0.0 ? double(pixels.y()) / 50.0 : double(angle.y()) / 120.0;
    m_scrollAccum += int(std::lround(amount * 100.0));
    int steps = 0;
    while (m_scrollAccum >= 100) {
        ++steps;
        m_scrollAccum -= 100;
    }
    while (m_scrollAccum <= -100) {
        --steps;
        m_scrollAccum += 100;
    }
    return -steps;
}

double DeckLayout::cameraMs(const LayoutContext &ctx) const
{
    return ctx.motion->ms(MotionProfile::Standard);
}
