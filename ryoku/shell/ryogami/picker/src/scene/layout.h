#pragma once

#include "cardvisual.h"
#include "params.h"

#include <QColor>
#include <QPointF>
#include <QRectF>
#include <QSizeF>
#include <QString>

#include <algorithm>
#include <vector>

class CardSource;
struct SandyPass;

struct ScenePalette {
    QColor primary = QColor(255, 122, 102);
    QColor accent = QColor(255, 180, 160);
    QColor surface = QColor(18, 20, 26);
    QColor surfaceVariant = QColor(40, 44, 54);
    QColor outline = QColor(120, 126, 140);
    QColor text = QColor(236, 238, 244);
    QColor shadow = QColor(0, 0, 0);
};

struct FlipState {
    int row = -1;
    double progress = 0;   // 0 front, 1 back
    int effect = 0;        // card.frag flip effect id
    float seed = 0;
};

struct LayoutContext {
    QSizeF viewport;
    int count = 0;
    int current = 0;
    int hovered = -1;
    QPointF pointer;
    bool pointerInside = false;
    double entrance = 1.0;      // open fade, 0..1
    double time = 0;            // seconds since the scene started
    const MotionProfile *motion = nullptr;
    const ParamSource *params = nullptr;
    const CardSource *source = nullptr;
    ScenePalette palette;
    FlipState flip;
    // 0 means the wall layout reads its column count from settings.
    int columnsOverride = 0;
    // The band the masthead takes at the top of the picker (its bottom edge plus a
    // gap) while it shows; the wall and sandy modes lay their cards out below it.
    QSizeF barReserve;
};

class Layout
{
public:
    enum class Click { Select, Apply, Ignore };

    virtual ~Layout() = default;

    virtual QString key() const = 0;

    // Settings changed or the layout was just attached; animate=false snaps.
    virtual void configure(const LayoutContext &ctx, bool animate) = 0;
    virtual void reset(const LayoutContext &ctx) = 0;
    virtual void select(const LayoutContext &ctx, int from, int to) = 0;
    // Advances motion by dt seconds; returns true while anything still moves.
    virtual bool tick(const LayoutContext &ctx, double dt) = 0;
    // Appends this frame's visuals, back to front.
    virtual void build(const LayoutContext &ctx, std::vector<CardVisual> &out) = 0;

    // Row under an item-space point, from the last build, or -1.
    virtual int hitTest(QPointF point) const = 0;
    virtual QRectF cardRect(int row) const = 0;
    // Skew and edge tilt (x, y) the row's card is drawn with; overlays use it to follow its shape.
    virtual QPointF cardShear(int row) const
    {
        Q_UNUSED(row)
        return {};
    }
    // Row an arrow key leads to (dx, dy in -1..1); current row when inert.
    virtual int step(const LayoutContext &ctx, int dx, int dy) const = 0;
    virtual int page(const LayoutContext &ctx, int dir) const { return dir * 5; }
    // Wheel input (angle in eighths of a degree, pixels from touchpads);
    // returns how many rows to move now.
    virtual int wheel(const LayoutContext &ctx, QPointF angle, QPointF pixels) = 0;
    // Pointer moved over the scene; true when it started motion (tilt, edge pan).
    virtual bool pointer(const LayoutContext &ctx, QPointF pos, bool inside)
    {
        Q_UNUSED(ctx) Q_UNUSED(pos) Q_UNUSED(inside)
        return false;
    }
    virtual bool drag(const LayoutContext &ctx, QPointF delta, bool released)
    {
        Q_UNUSED(ctx) Q_UNUSED(delta) Q_UNUSED(released)
        return false;
    }
    virtual Click click(const LayoutContext &ctx, int row)
    {
        return row == ctx.current ? Click::Apply : Click::Select;
    }
    virtual bool action(const LayoutContext &ctx, const QString &name)
    {
        Q_UNUSED(ctx) Q_UNUSED(name)
        return false;
    }
    // True when the card back is drawn by the layout itself.
    virtual bool flipsInPlace() const { return false; }
    // Camera settle-time tier in milliseconds for this mode.
    virtual double cameraMs(const LayoutContext &ctx) const
    {
        return ctx.motion->ms(MotionProfile::Standard);
    }
    virtual const SandyPass *sandyPass() const { return nullptr; }
    virtual qreal scrollPosition() const { return 0; }
    virtual qreal scrollExtent() const { return 0; }
    virtual void setScrollPosition(const LayoutContext &ctx, qreal position)
    {
        Q_UNUSED(ctx) Q_UNUSED(position)
    }
    // The band the cards occupy; the search panel sits against it.
    virtual QRectF stageRect(const LayoutContext &ctx) const
    {
        const double vh = ctx.viewport.height();
        const double h = std::max(vh - 90.0, 200.0);
        return QRectF(0, (vh - h) * 0.5, ctx.viewport.width(), h);
    }
    // Empty means no clip; only the wall mode frames its scroll region.
    virtual QRectF clip(const LayoutContext &ctx) const
    {
        Q_UNUSED(ctx)
        return QRectF();
    }
};
