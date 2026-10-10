#include "depthlayout.h"

#include <algorithm>
#include <cmath>

namespace {

constexpr char kPrefix[] = "components.wallpaperSelector.";

QString skey(const char *name)
{
    return QString::fromLatin1(kPrefix) + QLatin1String(name);
}

// Monotonic and odd, compressing far cards.
float depthOffset(float distance, float falloff)
{
    if (falloff < 1e-4f)
        return distance;
    const float sign = distance > 0.0f ? 1.0f : (distance < 0.0f ? -1.0f : 0.0f);
    return sign * std::log(1.0f + falloff * std::abs(distance)) / falloff;
}

} // namespace

void DepthLayout::readParams(const LayoutContext &ctx, SliceParams &out) const
{
    const ParamSource &p = *ctx.params;
    out.sliceH = float(std::clamp(p.num(skey("depthHeight"), 520.0), 60.0, 4096.0));
    out.visibleCount = int(std::lround(std::clamp(p.num(skey("depthCount"), 5.0), 3.0, 21.0)));
    out.skew = float(std::clamp(p.num(skey("depthSkew"), 0.0), -2048.0, 2048.0));
    const float c = float(std::clamp(p.num(skey("depthCorners"), 0.0), 0.0, 2048.0));
    out.corners = {c, c, c, c};
    out.edgeTilt = 0.0f;
    out.shadows = p.flag(skey("depthShadows"), false);
    out.shadowStrength = float(std::clamp(p.num(skey("shadowStrength"), 100.0) / 100.0, 0.0, 2.0));
    out.shadowDistance = float(std::clamp(p.num(skey("shadowDistance"), 100.0) / 100.0, 0.0, 2.0));
    out.wobble = false;
    out.parallax = false;
    out.offsetX = 0.0f;
    out.offsetY = 0.0f;
}

void DepthLayout::readDepthParams(const LayoutContext &ctx)
{
    const ParamSource &p = *ctx.params;
    m_depth.width = float(std::clamp(p.num(skey("depthWidthPx"), 280.0), 24.0, 4096.0));
    m_depth.spacing = float(std::clamp(p.num(skey("depthSpacingPx"), 280.0), 0.0, 4096.0));
    m_depth.falloff = float(std::clamp(p.num(skey("depthFalloffFactor"), 0.05), 0.0, 10.0));
    m_depth.navigationMs = float(std::clamp(p.num(skey("depthNavigationMs"), 1000.0), 1.0, 10000.0));
    m_depth.selectionFrame = p.flag(skey("depthSelectionFrame"), false);
}

double DepthLayout::navMs(const LayoutContext &ctx) const
{
    if (ctx.motion->reduced)
        return 1.0;
    return double(m_depth.navigationMs) * ctx.motion->scale;
}

void DepthLayout::configure(const LayoutContext &ctx, bool animate)
{
    readDepthParams(ctx);
    SliceBase::configure(ctx, animate);
}

void DepthLayout::reset(const LayoutContext &ctx)
{
    Q_UNUSED(ctx)
    m_selection.clear();
    m_scrollAccum = 0;
    m_camera.snap(0.0);
    m_anchor = true;
}

void DepthLayout::select(const LayoutContext &ctx, int from, int to)
{
    retargetSelection(from, to, navMs(ctx), 0.001);
}

bool DepthLayout::tick(const LayoutContext &ctx, double dt)
{
    bool moving = morphParams(ctx, dt);
    m_camera.setDuration(navMs(ctx), 1.0);
    // Tiny, so the slow push settles exactly rather than one card short.
    const float target = float(m_camera.target);
    const float scaleDenom = std::max({m_depth.spacing, m_depth.width, 1.0f});
    m_camera.epsilon = std::max(0.05 / double(scaleDenom), double(std::abs(target)) * 1.1920929e-7);
    moving |= m_camera.tick(dt);
    moving |= tickSelection(dt, 0.001);
    return moving;
}

void DepthLayout::build(const LayoutContext &ctx, std::vector<CardVisual> &out)
{
    m_hits.clear();
    const int count = ctx.count;
    if (count == 0)
        return;
    const int current = std::clamp(ctx.current, 0, count - 1);

    if (m_anchor) {
        m_camera.snap(current);
        m_anchor = false;
    } else {
        m_camera.target = current;
    }
    const float center = std::clamp(float(m_camera.x), 0.0f, float(count - 1));
    const float falloff = m_depth.falloff;
    const float spacing = m_depth.spacing;
    const float radius = (std::clamp(m_live.visibleCount, 3, 21) - 1) * 0.5f;
    const float parallaxExtent = depthOffset(radius + 1.0f, falloff);
    const float vw = float(ctx.viewport.width());
    const float vh = float(ctx.viewport.height());
    const geom::CenterLayout cl = geom::centerLayout(vw, 0.0f);
    float width = m_depth.width;
    float height = std::max(m_live.sliceH, 1.0f);
    const float naturalSpan = std::max(
        depthOffset(radius, falloff) * spacing + width * 0.5f, 1.0f);
    const float fit = std::min({std::max(cl.availW * 0.5f - 36.0f, 1.0f) / naturalSpan,
                                vh * 0.7f / height, 1.0f});
    width *= fit;
    height *= fit;
    const float cx = cl.centerX;
    const float cy = vh * 0.5f;

    const int low = std::clamp(int(std::floor(center - radius - 1.0f)), 0, count - 1);
    const int high = std::min(int(std::ceil(center + radius + 1.0f)), count - 1);
    m_visible.clear();
    for (int i = low; i <= high; ++i)
        m_visible.push_back(i);
    std::sort(m_visible.begin(), m_visible.end(), [center](int a, int b) {
        return std::abs(float(b) - center) < std::abs(float(a) - center);
    });

    const float halfView = cl.availW * 0.5f;
    for (int i : m_visible) {
        const float n = float(i) - center;
        const float scale = 1.0f / (1.0f + falloff * std::abs(n));
        const float opacity =
            geom::smoothstep(std::clamp(radius + 1.0f - std::abs(n), 0.0f, 1.0f)) * float(ctx.entrance);
        if (opacity <= 0.0001f)
            continue;
        const float offset = depthOffset(n, falloff);
        const float itemCx = cx + offset * spacing * fit;

        SliceParams sp = m_live;
        sp.sliceH = height * scale;
        sp.skew = std::clamp(m_live.skew * fit, -width * 0.3f, width * 0.3f) * scale;
        sp.edgeTilt = 0.0f;
        for (int k = 0; k < 4; ++k)
            sp.corners[size_t(k)] = m_live.corners[size_t(k)] * fit * scale;
        sp.wobble = false;

        emitCard(ctx, out, i, width * scale, itemCx, cy, opacity, halfView, 0.0f, sp, true,
                 m_depth.selectionFrame, true, offset / parallaxExtent);
    }
}
