#include "geometry.h"

#include <QQuaternion>

#include <cmath>

namespace geom {

float signedHash(unsigned index, uint32_t salt)
{
    uint32_t value = index + salt;
    value ^= value >> 16;
    value *= 0x7feb352du;
    value ^= value >> 15;
    value *= 0x846ca68bu;
    value ^= value >> 16;
    return float(value) / float(0xffffffffu) * 2.0f - 1.0f;
}

float Bezier::at(float x) const
{
    if (x <= 0.0f)
        return 0.0f;
    if (x >= 1.0f)
        return 1.0f;
    const auto sample = [](float p1, float p2, float t) {
        const float mt = 1.0f - t;
        return 3.0f * mt * mt * t * p1 + 3.0f * mt * t * t * p2 + t * t * t;
    };
    float t = x;
    for (int i = 0; i < 8; ++i) {
        const float fx = sample(x1, x2, t) - x;
        const float mt = 1.0f - t;
        const float dx = 3.0f * mt * mt * x1 + 6.0f * mt * t * (x2 - x1) + 3.0f * t * t * (1.0f - x2);
        if (std::abs(dx) < 1e-6f)
            break;
        t -= fx / dx;
        t = t < 0.0f ? 0.0f : (t > 1.0f ? 1.0f : t);
    }
    return sample(y1, y2, t);
}

std::array<float, 4> cornerClamp(std::array<float, 4> corners, float flatW, float slantLen)
{
    float rcMax = std::min(flatW / 2.0f - 1.0f, slantLen / 2.0f - 1.0f);
    if (rcMax < 0.0f)
        rcMax = 0.0f;
    for (float &r : corners)
        r = r < 0.0f ? 0.0f : (r > rcMax ? rcMax : r);
    return corners;
}

std::array<float, 4> sliceClampedCorners(std::array<float, 4> corners, float w, float h,
                                         float skew, float edgeTilt)
{
    const float skAbs = std::abs(skew);
    const float tiltAbs = std::abs(edgeTilt);
    const float flatBase = std::max(w - skAbs, 0.001f);
    const float horizontalEdge = std::sqrt(flatBase * flatBase + edgeTilt * edgeTilt);
    const float slantBase = std::max(h - tiltAbs, 0.001f);
    const float verticalEdge = std::sqrt(skew * skew + slantBase * slantBase);
    return cornerClamp(corners, horizontalEdge, verticalEdge);
}

bool shearedContains(float cx, float cy, float hw, float hh, float skew, float edgeTilt,
                     float px, float py)
{
    if (hw <= 0.0f || hh <= 0.0f)
        return false;
    const float sx = skew * 0.5f;
    const float ty = edgeTilt * 0.5f;
    const float bx = std::max(hw - std::abs(sx), 1.0f);
    const float by = std::max(hh - std::abs(ty), 1.0f);
    const float det = bx * by - sx * ty;
    if (std::abs(det) < 1.0f)
        return false;
    const float x = px - cx;
    const float y = py - cy;
    const float u = (by * x + sx * y) / det;
    const float v = (ty * x + bx * y) / det;
    return std::abs(u) <= 1.0f && std::abs(v) <= 1.0f;
}

bool pointInQuad(const std::array<QPointF, 4> &quad, float px, float py)
{
    bool positive = false;
    bool negative = false;
    for (int i = 0; i < 4; ++i) {
        const QPointF &a = quad[i];
        const QPointF &b = quad[(i + 1) % 4];
        const double cross = (b.x() - a.x()) * (py - a.y()) - (b.y() - a.y()) * (px - a.x());
        positive |= cross > 0.0;
        negative |= cross < 0.0;
    }
    return positive != negative;
}

float sliceOpacity(float itemCenterX, float viewCenterX, float halfView,
                   float expandedLayoutW, float sliceStride, float edgeDist, float halfWidth)
{
    if (halfView <= 0.0f)
        return 1.0f;
    const float fullZone = std::min(0.6f, (expandedLayoutW / 2.0f + 2.0f * sliceStride) / halfView);
    // The fade must finish before a card's outer edge reaches the viewport
    // edge, or the screen slices the card off at partial opacity.
    const float fit = edgeDist - halfWidth;
    float zeroAt = 1.2f * halfView;
    if (fit > 0.0f)
        zeroAt = std::min(zeroAt, fit);
    const float dist = std::abs(itemCenterX - viewCenterX);
    const float start = std::min(fullZone * halfView, zeroAt);
    if (dist <= start)
        return 1.0f;
    if (dist >= zeroAt)
        return 0.0f;
    const float span = zeroAt - start;
    if (span <= 1e-4f)
        return 0.0f;
    return std::clamp(1.0f - (dist - start) / span, 0.0f, 1.0f);
}

void rollInCut(CardInstance &body, float fraction)
{
    if (fraction >= 1.0f)
        return;
    const float halfWidth = body.rect[2];
    body.rect[0] += halfWidth * (1.0f - fraction);
    body.rect[2] = halfWidth * fraction;
    body.crop[2] *= fraction;
    for (float &r : body.radii)
        r = std::min(r, std::min(body.rect[2], body.rect[3]));
}

int sliceScrollSteps(float &accumulator, float amount)
{
    if (std::abs(amount) >= 1.0f) {
        accumulator = 0.0f;
        return int(std::lround(amount));
    }
    accumulator += amount;
    const int steps = int(std::trunc(accumulator));
    accumulator -= float(steps);
    return steps;
}

void rollOutCut(CardInstance &body, float fraction)
{
    const float halfWidth = body.rect[2];
    body.rect[0] -= halfWidth * fraction;
    body.rect[2] = halfWidth * (1.0f - fraction);
    body.crop[0] += body.crop[2] * fraction;
    body.crop[2] *= 1.0f - fraction;
    for (float &r : body.radii)
        r = std::min(r, std::min(body.rect[2], body.rect[3]));
}

QVector3D Camera::project(const QVector3D &p) const
{
    const float denom = d < 1.0f ? 1.0f : d;
    float w = (d - p.z()) / denom;
    if (w < 0.05f)
        w = 0.05f;
    return QVector3D(origin.x() + shift.x() + (p.x() - shift.x()) / w,
                     origin.y() + shift.y() + (p.y() - shift.y()) / w, w);
}

std::array<QVector3D, 4> cardCorners(float hw, float hh, float padX, float padY)
{
    return {QVector3D(-hw - padX, -hh - padY, 0.0f), QVector3D(hw + padX, -hh - padY, 0.0f),
            QVector3D(hw + padX, hh + padY, 0.0f), QVector3D(-hw - padX, hh + padY, 0.0f)};
}

std::array<float, 4> cardLocals(float hh, float pad)
{
    return {-hh - pad, -hh - pad, hh + pad, hh + pad};
}

Pose Pose::make(const QVector3D &t, std::initializer_list<Rot> rots, float s)
{
    Pose p;
    p.t = t;
    p.s = s;
    int i = 0;
    for (const Rot &r : rots) {
        if (i >= 3)
            break;
        p.rots[size_t(i++)] = r;
    }
    p.n = i;
    return p;
}

QMatrix4x4 Pose::matrix() const
{
    QMatrix4x4 m;
    m.translate(t);
    for (int i = 0; i < n; ++i) {
        const Rot &r = rots[size_t(i)];
        if (!r.axis.isNull() && std::abs(r.deg) > 1e-6f)
            m.rotate(r.deg, r.axis);
    }
    if (std::abs(s - 1.0f) > 1e-6f)
        m.scale(s);
    return m;
}

namespace {

bool axisEq(const QVector3D &a, const QVector3D &b)
{
    return std::abs(a.x() - b.x()) < 1e-4f && std::abs(a.y() - b.y()) < 1e-4f
        && std::abs(a.z() - b.z()) < 1e-4f;
}

bool ordersMatch(const Pose &a, const Pose &b)
{
    const int m = std::min(a.n, b.n);
    for (int i = 0; i < m; ++i)
        if (!axisEq(a.rots[size_t(i)].axis, b.rots[size_t(i)].axis))
            return false;
    return true;
}

QQuaternion poseQuat(const Pose &p)
{
    QQuaternion q;
    for (int i = 0; i < p.n; ++i) {
        QVector3D axis = p.rots[size_t(i)].axis.normalized();
        if (!axis.isNull())
            q *= QQuaternion::fromAxisAndAngle(axis, p.rots[size_t(i)].deg);
    }
    return q;
}

} // namespace

Pose Pose::mix(const Pose &a, const Pose &b, float u)
{
    const QVector3D t(lerp(a.t.x(), b.t.x(), u), lerp(a.t.y(), b.t.y(), u),
                      lerp(a.t.z(), b.t.z(), u));
    const float s = lerp(a.s, b.s, u);
    if (ordersMatch(a, b)) {
        Pose out;
        out.t = t;
        out.s = s;
        out.n = std::max(a.n, b.n);
        for (int i = 0; i < out.n; ++i) {
            const QVector3D axis = i < b.n ? b.rots[size_t(i)].axis : a.rots[size_t(i)].axis;
            const float da = i < a.n ? a.rots[size_t(i)].deg : 0.0f;
            const float db = i < b.n ? b.rots[size_t(i)].deg : 0.0f;
            out.rots[size_t(i)] = {axis, lerp(da, db, u)};
        }
        return out;
    }
    const QQuaternion q = QQuaternion::slerp(poseQuat(a), poseQuat(b), u);
    QVector3D axis;
    float angle = 0.0f;
    q.getAxisAndAngle(&axis, &angle);
    if (std::abs(angle) < 1e-5f)
        return Pose::make(t, {Rot{QVector3D(0, 0, 1), 0.0f}}, s);
    return Pose::make(t, {Rot{axis, angle}}, s);
}

ProjectedQuad projectQuad(const Camera &cam, const QMatrix4x4 &m,
                          const std::array<QVector3D, 4> &corners)
{
    ProjectedQuad q;
    q.depth = 0.0f;
    for (int i = 0; i < 4; ++i) {
        const QVector3D world = m.map(corners[size_t(i)]);
        q.depth += world.z() * 0.25f;
        const QVector3D s = cam.project(world);
        q.screen[size_t(i)] = QPointF(s.x(), s.y());
        q.w[size_t(i)] = s.z();
    }
    return q;
}

bool ProjectedQuad::facing() const
{
    const QPointF &tl = screen[0];
    const QPointF &tr = screen[1];
    const QPointF &bl = screen[3];
    const double ax = tr.x() - tl.x();
    const double ay = tr.y() - tl.y();
    const double bx = bl.x() - tl.x();
    const double by = bl.y() - tl.y();
    return ax * by - ay * bx > 0.0;
}

QRectF ProjectedQuad::bounds() const
{
    double minX = screen[0].x(), minY = screen[0].y();
    double maxX = minX, maxY = minY;
    for (int i = 1; i < 4; ++i) {
        minX = std::min(minX, screen[size_t(i)].x());
        minY = std::min(minY, screen[size_t(i)].y());
        maxX = std::max(maxX, screen[size_t(i)].x());
        maxY = std::max(maxY, screen[size_t(i)].y());
    }
    return QRectF(minX, minY, maxX - minX, maxY - minY);
}

bool ProjectedQuad::visible(float viewportW, float viewportH) const
{
    const QRectF b = bounds();
    return b.right() >= -8.0 && b.left() <= viewportW + 8.0 && b.bottom() >= -8.0
        && b.top() <= viewportH + 8.0;
}

} // namespace geom
