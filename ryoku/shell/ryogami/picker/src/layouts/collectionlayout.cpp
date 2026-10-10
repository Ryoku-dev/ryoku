#include "collectionlayout.h"

#include "cardsource.h"

#include <algorithm>
#include <cmath>

using namespace handrig;

namespace {

void setF4(float *dst, float a, float b, float c, float d)
{
    dst[0] = a;
    dst[1] = b;
    dst[2] = c;
    dst[3] = d;
}

float clampf(float v, float lo, float hi) { return v < lo ? lo : v > hi ? hi : v; }

const QString kPrefix = QStringLiteral("components.wallpaperSelector.");

struct QuadWH {
    Quad quad;
    float hw;
    float hh;
};

QuadWH collectionQuad(float vh, float availW, float cx, float distance, float open, bool selected, float aspect,
                      float sizePct, float spacingPct, float tiltDeg)
{
    aspect = clampf(aspect, 0.5f, 2.4f);
    const float size = std::max(std::min(vh * sizePct / 100.0f, availW * 0.64f), 1.0f);
    const float scale = 1.0f + distance * 0.045f;
    const float width = size * scale;
    const float previewW = std::min(availW * 0.76f, vh * 0.56f * aspect);

    float targetW, targetH, targetY, targetTilt;
    if (selected) {
        targetW = previewW;
        targetH = previewW / aspect;
        targetY = vh * 0.41f;
        targetTilt = 0.0f;
    } else {
        targetW = size * 0.43f;
        targetH = size * 0.43f;
        targetY = vh * 0.86f + distance * size * 0.026f;
        targetTilt = -55.0f;
    }
    const auto mix = [open](float a, float b) { return a + (b - a) * open; };
    const float hw = mix(width, targetW) * 0.5f;
    const float hh = mix(width, targetH) * 0.5f;
    const float cy = mix(vh * 0.53f + distance * size * spacingPct / 100.0f, targetY);
    const float tiltDeg2 = mix(-tiltDeg, targetTilt);

    Camera camera;
    camera.d = size * 3.5f;
    camera.origin = QVector2D(cx, cy);
    camera.shift = QVector2D(0.0f, 0.0f);
    const Quad quad = projectQuad(camera, rotationMatrix(AXIS_X, tiltDeg2), cardCorners(hw, hh, PAD, PAD));
    return {quad, hw, hh};
}

}  // namespace

CollectionLayout::CollectionLayout()
{
    m_camera = Spring::forDuration(0.0, 250.0);
    m_open = Spring::forDuration(0.0, 250.0);
}

void CollectionLayout::readParams(const ParamSource *src, Params &out) const
{
    if (!src)
        return;
    const auto num = [&](const char *k, double fb) { return src->num(kPrefix + QLatin1String(k), fb); };
    out.size = clampf(float(num("collectionSize", 42)), 15.0f, 65.0f);
    out.spacing = clampf(float(num("collectionSpacing", 17)), 5.0f, 30.0f);
    out.count = float(std::lround(std::clamp(num("collectionCount", 7), 3.0, 11.0)));
    out.tilt = clampf(float(num("collectionTilt", 52)), 0.0f, 75.0f);
    out.corners = clampf(float(num("collectionCorners", 2)), 0.0f, 100.0f);
    out.speed = clampf(float(num("collectionSpeed", 100)), 25.0f, 300.0f);
    out.shadows = src->flag(kPrefix + QLatin1String("collectionShadows"), true);
    out.shadowStrength = clampf(float(num("shadowStrength", 100)) / 100.0f, 0.0f, 2.0f);
    out.shadowDistance = clampf(float(num("shadowDistance", 100)) / 100.0f, 0.0f, 2.0f);
}

double CollectionLayout::cameraMs(const LayoutContext &ctx) const
{
    return ctx.motion->ms(MotionProfile::Standard) * 100.0 / std::max<double>(m_params.speed, 1.0);
}

double CollectionLayout::openMs(const LayoutContext &ctx) const { return cameraMs(ctx); }

void CollectionLayout::raiseSelf(int idx)
{
    if (m_card != idx)
        m_open.snap(0.0);
    m_card = idx;
    m_open.target = 1.0;
}

void CollectionLayout::configure(const LayoutContext &ctx, bool animate)
{
    readParams(ctx.params, m_params);
    m_count = ctx.count;
    m_current = std::clamp(ctx.current, 0, std::max(ctx.count - 1, 0));
    m_camera.setDuration(cameraMs(ctx), 1.0);
    m_open.setDuration(openMs(ctx), 1.0);
    if (!animate) {
        m_camera.snap(m_current);
        m_open.snap(0.0);
        m_card = -1;
        m_anchor = true;
    }
}

void CollectionLayout::reset(const LayoutContext &ctx)
{
    m_count = ctx.count;
    m_current = std::clamp(ctx.current, 0, std::max(ctx.count - 1, 0));
    m_camera.snap(m_current);
    m_open.snap(0.0);
    m_card = -1;
    m_anchor = true;
}

void CollectionLayout::select(const LayoutContext &ctx, int from, int to)
{
    Q_UNUSED(from)
    m_count = ctx.count;
    if (ctx.count == 0)
        return;
    m_current = std::clamp(to, 0, ctx.count - 1);
    m_camera.target = m_current;
    if (m_raiseIntent == to) {
        raiseSelf(to);
        m_raiseIntent = -1;
    } else if (m_open.target > 0.5) {
        m_open.target = 0.0;  // any other nav lowers the shelf
    }
}

bool CollectionLayout::tick(const LayoutContext &ctx, double dt)
{
    m_count = ctx.count;
    m_current = std::clamp(ctx.current, 0, std::max(ctx.count - 1, 0));
    m_camera.setDuration(cameraMs(ctx), 1.0);
    m_open.setDuration(openMs(ctx), 1.0);
    const bool cameraMoving = m_camera.tick(dt);
    const bool openMoving = m_open.tick(dt);
    if (m_open.settled() && m_open.target <= 0.0)
        m_card = -1;
    return cameraMoving || openMoving;
}

void CollectionLayout::build(const LayoutContext &ctx, std::vector<CardVisual> &out)
{
    m_hits.clear();
    m_rects.clear();
    m_count = ctx.count;
    if (ctx.count == 0)
        return;
    m_current = std::clamp(ctx.current, 0, ctx.count - 1);

    if (m_anchor) {
        m_camera.snap(m_current);
        m_anchor = false;
    } else {
        m_camera.target = m_current;
    }

    const int count = ctx.count;
    const float radius = (std::round(m_params.count) - 1.0f) * 0.5f;
    const float last = float(count - 1);
    const float center = clampf(float(m_camera.x), std::min(radius, last * 0.5f), std::max(last - radius, last * 0.5f));
    const int visLo = std::clamp(int(std::floor(center - radius - 1.0f)), 0, count - 1);
    const int visHi = std::clamp(int(std::ceil(center + radius + 1.0f)), 0, count - 1);
    const float open = clampf(float(m_open.x), 0.0f, 1.0f);

    std::vector<int> visible;
    for (int i = visLo; i <= visHi; ++i)
        visible.push_back(i);
    const auto depthOf = [&](int idx) {
        return float(idx) - center + (idx == m_card ? open * 12.0f : 0.0f);
    };
    std::sort(visible.begin(), visible.end(), [&](int a, int b) { return depthOf(a) < depthOf(b); });

    const float vw = float(ctx.viewport.width());
    const float vh = float(ctx.viewport.height());
    const float availW = vw;
    const float cx = vw * 0.5f;
    const float entrance = float(ctx.entrance);
    const QColor &prim = ctx.palette.primary;

    for (int idx : visible) {
        const float distance = float(idx) - center;
        const float opacity = clampf(radius + 1.0f - std::abs(distance), 0.0f, 1.0f) * entrance;
        if (opacity <= 0.001f)
            continue;

        float aspect = 16.0f / 9.0f;
        if (ctx.source) {
            const QSizeF sz = ctx.source->cardImageSize(idx);
            if (sz.width() > 0 && sz.height() > 0)
                aspect = clampf(float(sz.width() / sz.height()), 0.5f, 2.4f);
        }
        const bool selected = idx == m_card;
        const QuadWH q = collectionQuad(vh, availW, cx, distance, open, selected, aspect, m_params.size,
                                        m_params.spacing, m_params.tilt);
        const auto b = q.quad.bounds();
        const auto locals = cardLocals(q.hh, PAD);

        CardVisual body;
        body.row = idx;
        CardInstance &inst = body.inst;
        setF4(inst.rect, b[0], b[1], q.hw, q.hh);
        const float rad = clampf(m_params.corners, 0.0f, std::max(std::min(q.hw, q.hh) - 1.0f, 0.0f));
        setF4(inst.radii, rad, rad, rad, rad);
        inst.params[0] = 0.0f;
        inst.params[2] = opacity;
        inst.misc[3] |= CardFlag::Projected;
        setF4(inst.quadA, q.quad.pts[0].x(), q.quad.pts[0].y(), q.quad.pts[1].x(), q.quad.pts[1].y());
        setF4(inst.quadB, q.quad.pts[2].x(), q.quad.pts[2].y(), q.quad.pts[3].x(), q.quad.pts[3].y());
        setF4(inst.quadW, q.quad.pts[0].z(), q.quad.pts[1].z(), q.quad.pts[2].z(), q.quad.pts[3].z());
        setF4(inst.quadL, locals[0], locals[1], locals[2], locals[3]);
        body.texture = CardVisual::TextureAuto;
        body.wantNear = true;
        body.cover = true;
        body.cropAspect = q.hh > 0.0f ? q.hw / q.hh : 0.0f;

        if (m_params.shadows) {
            CardVisual shadow = body;
            shadow.row = -1;
            shadow.texture = CardVisual::TextureNone;
            shadow.wantNear = false;
            shadow.cover = false;
            shadow.cropAspect = 0.0f;
            shadow.inst.misc[0] = 0;
            shadow.inst.misc[1] = 0;
            shadow.inst.misc[2] = 0;
            shadow.inst.misc[3] = CardFlag::Projected;
            setF4(shadow.inst.fill, 0.0f, 0.0f, 0.0f, 0.24f * m_params.shadowStrength);
            setF4(shadow.inst.border, 0.0f, 0.0f, 0.0f, 0.0f);
            setF4(shadow.inst.tint, 0.0f, 0.0f, 0.0f, 0.0f);
            const float shOff = 5.0f * m_params.shadowDistance;
            shadow.inst.quadA[1] += shOff;
            shadow.inst.quadA[3] += shOff;
            shadow.inst.quadB[1] += shOff;
            shadow.inst.quadB[3] += shOff;
            out.push_back(std::move(shadow));
        }

        if (idx == m_current)
            setF4(inst.border, float(prim.redF()), float(prim.greenF()), float(prim.blueF()), 0.85f * (1.0f - open));
        else
            setF4(inst.border, 1.0f, 1.0f, 1.0f, 0.16f);
        inst.params[1] = 1.0f;
        setF4(inst.tint, 0.0f, 0.0f, 0.0f, clampf(-distance * 0.035f, 0.0f, 0.16f) * (1.0f - open));
        out.push_back(std::move(body));

        m_hits.push_back(Hit{idx, q.quad.pts});
        m_rects[idx] = QRectF(b[0] - b[2], b[1] - b[3], b[2] * 2.0f, b[3] * 2.0f);
    }

    m_raiseIntent = -1;
}

int CollectionLayout::hitTest(QPointF point) const
{
    for (auto it = m_hits.rbegin(); it != m_hits.rend(); ++it)
        if (pointInQuad(it->pts, point))
            return it->row;
    return -1;
}

QRectF CollectionLayout::cardRect(int row) const
{
    const auto it = m_rects.find(row);
    return it != m_rects.end() ? it->second : QRectF();
}

int CollectionLayout::step(const LayoutContext &ctx, int dx, int dy) const
{
    if (ctx.count == 0)
        return ctx.current;
    int dir = 0;
    if (dx != 0)
        dir = dx > 0 ? 1 : -1;
    else if (dy != 0)
        dir = dy > 0 ? 1 : -1;
    return std::clamp(ctx.current + dir, 0, ctx.count - 1);
}

int CollectionLayout::page(const LayoutContext &ctx, int dir) const
{
    Q_UNUSED(ctx)
    return dir * std::max(int(std::round(m_params.count)), 1);
}

int CollectionLayout::wheel(const LayoutContext &ctx, QPointF angle, QPointF pixels)
{
    Q_UNUSED(ctx)
    if (m_open.target > 0.5)
        m_open.target = 0.0;
    m_wheelAccum += float(angle.y()) + float(pixels.y());
    const int notches = int(m_wheelAccum / 120.0f);
    if (notches != 0)
        m_wheelAccum -= float(notches) * 120.0f;
    return -notches;
}

bool CollectionLayout::pointer(const LayoutContext &ctx, QPointF pos, bool inside)
{
    Q_UNUSED(ctx)
    Q_UNUSED(pos)
    Q_UNUSED(inside)
    return false;
}

Layout::Click CollectionLayout::click(const LayoutContext &ctx, int row)
{
    if (openFor(row))
        return Click::Apply;  // second click on the raised card applies it (#86)
    m_raiseIntent = row;
    if (row == ctx.current) {
        raiseSelf(row);
        m_raiseIntent = -1;
        return Click::Ignore;
    }
    return Click::Select;
}
