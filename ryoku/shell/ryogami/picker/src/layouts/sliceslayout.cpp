#include "sliceslayout.h"

#include <algorithm>
#include <cstdint>

#include <cmath>
namespace {

constexpr char kPrefix[] = "components.wallpaperSelector.";

QString skey(const char *name)
{
    return QString::fromLatin1(kPrefix) + QLatin1String(name);
}

} // namespace

void SlicesLayout::readParams(const LayoutContext &ctx, SliceParams &out) const
{
    const ParamSource &p = *ctx.params;
    const bool small = p.smallScreen();
    out.sliceW = float(std::clamp(p.num(skey("sliceWidth"), small ? 110.0 : 170.0), 24.0, 2048.0));
    out.expandedW = float(std::clamp(p.num(skey("expandedWidth"), small ? 500.0 : 720.0), 32.0, 8192.0));
    out.sliceH = float(std::clamp(p.num(skey("sliceHeight"), small ? 360.0 : 520.0), 60.0, 4096.0));
    out.spacing = float(std::clamp(p.num(skey("sliceSpacing"), 20.0), 0.0, 1024.0));
    out.skew = float(std::clamp(p.num(skey("skewOffset"), 0.0), -2048.0, 2048.0));
    out.edgeTilt = float(std::clamp(p.num(skey("sliceEdgeTilt"), 0.0), -2048.0, 2048.0));
    out.visibleCount = int(std::lround(std::clamp(
        p.num(skey("visibleCount"), small ? 8.0 : 12.0), 1.0, 128.0)));
    out.wobble = p.flag(skey("sliceWobble"), false);
    out.wobbleStrength = float(std::clamp(p.num(skey("sliceWobbleStrength"), 100.0) / 100.0, 0.0, 2.0));
    out.parallax = p.flag(skey("sliceParallax"), false);
    out.shadows = p.flag(skey("sliceShadows"), true);
    out.shadowStrength = float(std::clamp(p.num(skey("shadowStrength"), 100.0) / 100.0, 0.0, 2.0));
    out.shadowDistance = float(std::clamp(p.num(skey("shadowDistance"), 100.0) / 100.0, 0.0, 2.0));
    out.offsetX = float(std::clamp(p.num(skey("sliceStageX"), 0.0) / 100.0, -1.0, 1.0));
    out.offsetY = float(std::clamp(p.num(skey("sliceStageY"), 0.0) / 100.0, -1.0, 1.0));
    if (p.flag(skey("roundCorners"), true)) {
        out.corners = {
            float(std::clamp(p.num(skey("cornerTL"), 18.0), 0.0, 2048.0)),
            float(std::clamp(p.num(skey("cornerTR"), 18.0), 0.0, 2048.0)),
            float(std::clamp(p.num(skey("cornerBR"), 18.0), 0.0, 2048.0)),
            float(std::clamp(p.num(skey("cornerBL"), 18.0), 0.0, 2048.0)),
        };
    } else {
        out.corners = {0, 0, 0, 0};
    }
}

float SlicesLayout::widthOf(int row, int current) const
{
    const auto it = m_widths.find(row);
    if (it != m_widths.end())
        return float(it->second.x);
    return row == current ? m_live.expandedW : m_live.sliceW;
}

void SlicesLayout::configure(const LayoutContext &ctx, bool animate)
{
    SliceBase::configure(ctx, animate);
    for (auto &kv : m_widths) {
        const double target = kv.first == ctx.current ? m_target.expandedW : m_target.sliceW;
        if (!animate)
            kv.second.snap(target);
        else
            kv.second.target = target;
    }
}

void SlicesLayout::reset(const LayoutContext &ctx)
{
    Q_UNUSED(ctx)
    m_widths.clear();
    m_selection.clear();
    m_scrollAccum = 0;
    m_camera.snap(m_live.expandedW * 0.5f);
    m_anchor = true;
}

void SlicesLayout::select(const LayoutContext &ctx, int from, int to)
{
    const double ms = ctx.motion->ms(MotionProfile::Standard);
    const double zeta = m_live.wobble ? 0.5 : 1.0;
    const auto reWidth = [&](int row, double startFallback, double target) {
        const auto it = m_widths.find(row);
        const double x = it != m_widths.end() ? it->second.x : startFallback;
        Spring s = Spring::forDuration(x, ms, zeta);
        s.target = target;
        m_widths[row] = s;
    };
    reWidth(from, m_live.expandedW, m_live.sliceW);
    reWidth(to, m_live.sliceW, m_live.expandedW);
    retargetSelection(from, to, ms, 0.05);
}

bool SlicesLayout::tick(const LayoutContext &ctx, double dt)
{
    bool moving = morphParams(ctx, dt);
    const double ms = ctx.motion->ms(MotionProfile::Standard);
    const double zeta = m_live.wobble ? 0.5 : 1.0;
    for (auto it = m_widths.begin(); it != m_widths.end();) {
        it->second.setDuration(ms, zeta);
        const bool m = it->second.tick(dt);
        moving |= m;
        if (!m)
            it = m_widths.erase(it);
        else
            ++it;
    }
    m_camera.setDuration(cameraMs(ctx), m_live.wobble ? 0.45 : 1.0);
    moving |= m_camera.tick(dt);
    moving |= tickSelection(dt, 0.05);
    return moving;
}

void SlicesLayout::build(const LayoutContext &ctx, std::vector<CardVisual> &out)
{
    m_hits.clear();
    const SliceParams sp = m_live;
    const int count = ctx.count;
    if (count == 0)
        return;
    const int current = std::clamp(ctx.current, 0, count - 1);
    const float vw = float(ctx.viewport.width());
    const float vh = float(ctx.viewport.height());
    const geom::CenterLayout cl = geom::centerLayout(vw, 0.0f);
    const float stripW =
        std::min(sp.layoutWidth(sp.expandedW)
                     + float(std::max(0, sp.visibleCount - 1)) * sp.sliceStride(),
                 cl.availW - 40.0f);
    const float halfView = stripW * 0.5f;
    const float cx = cl.centerX + sp.offsetX * vw * 0.5f;
    const float cy = vh * 0.5f + sp.offsetY * vh * 0.5f;
    const float cardH = sp.sliceH + geom::kTopBar + 60.0f;

    m_centers.assign(size_t(count), 0.0f);
    float acc = 0.0f;
    float tacc = 0.0f;
    float targetCenter = 0.0f;
    for (int i = 0; i < count; ++i) {
        const float lw = sp.layoutWidth(widthOf(i, current));
        m_centers[size_t(i)] = acc + lw * 0.5f;
        acc += lw + sp.spacing;
        const float tlw = sp.layoutWidth(i == current ? sp.expandedW : sp.sliceW);
        if (i == current)
            targetCenter = tacc + tlw * 0.5f;
        tacc += tlw + sp.spacing;
    }

    positionCamera(targetCenter);
    const float cam = float(m_camera.x);
    const float bend = geom::wobbleBend(float(m_camera.v), sp.expandedW);
    const float top = cy - cardH * 0.5f + geom::kTopBar + 15.0f;
    const float itemCy = top + sp.sliceH * 0.5f;
    const float margin = sp.expandedW;

    m_visible.clear();
    for (int i = 0; i < count; ++i) {
        const float halfW = widthOf(i, current) * 0.5f;
        const float x0 = m_centers[size_t(i)] - halfW - (cam - halfView);
        const float x1 = m_centers[size_t(i)] + halfW - (cam - halfView);
        if (x1 >= -margin && x0 <= stripW + margin)
            m_visible.push_back(i);
    }

    const int hover = ctx.hovered;
    const auto orderKey = [&](int i) -> int64_t {
        if (i == current)
            return INT64_MAX;
        if (i == hover)
            return INT64_MAX - 1;
        return -int64_t(std::abs(i - current));
    };
    std::sort(m_visible.begin(), m_visible.end(),
              [&](int a, int b) { return orderKey(a) < orderKey(b); });

    const float expandedLayoutW = sp.layoutWidth(sp.expandedW);
    const float stride = sp.sliceStride();
    const float edgeDist = std::min(cx, vw - cx);
    for (int i : m_visible) {
        const float w = widthOf(i, current);
        const float itemCx = cx - cam + m_centers[size_t(i)];
        const float opacity = geom::sliceOpacity(itemCx, cx, halfView, expandedLayoutW, stride,
                                                 edgeDist, w * 0.5f)
                              * float(ctx.entrance);
        if (opacity <= 0.01f)
            continue;
        const float parallax =
            sp.parallax ? (itemCx - cx) / std::max(halfView * 1.2f, 1.0f) : 0.0f;
        emitCard(ctx, out, i, w, itemCx, itemCy, opacity, halfView, bend, sp, false, false,
                 sp.parallax, parallax);
    }
}
