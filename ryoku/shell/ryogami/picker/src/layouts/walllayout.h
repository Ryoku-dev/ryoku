#pragma once

#include "../scene/layout.h"
#include "../scene/spring.h"

#include <QRectF>

#include <cstdint>
#include <utility>
#include <vector>

class WallLayout final : public Layout
{
public:
    WallLayout();

    QString key() const override;
    void configure(const LayoutContext &ctx, bool animate) override;
    void reset(const LayoutContext &ctx) override;
    void select(const LayoutContext &ctx, int from, int to) override;
    bool tick(const LayoutContext &ctx, double dt) override;
    void build(const LayoutContext &ctx, std::vector<CardVisual> &out) override;

    int hitTest(QPointF point) const override;
    QRectF cardRect(int row) const override;
    QRectF stageRect(const LayoutContext &ctx) const override;
    int step(const LayoutContext &ctx, int dx, int dy) const override;
    int page(const LayoutContext &ctx, int dir) const override;
    int wheel(const LayoutContext &ctx, QPointF angle, QPointF pixels) override;
    double cameraMs(const LayoutContext &ctx) const override;
    QRectF clip(const LayoutContext &ctx) const override;
    qreal scrollPosition() const override { return m_camera.x; }
    qreal scrollExtent() const override { return m_maxScroll; }
    void setScrollPosition(const LayoutContext &ctx, qreal position) override;

public:
    enum Kind { Uniform, Brick, Masonry, Justified, Editorial, Cylinder };

private:

    struct Stage {
        float offsetX = 0, offsetY = 0, scale = 1, rotation = 0;
        float perspective = 0, shearX = 0, shearY = 0, depthAngle = 0;
        void transform(float x, float y, float ox, float oy, float vw, float vh,
                       float &outX, float &outY, float &outScale) const;
        void morphToward(const Stage &t, float amt);
        bool settledTo(const Stage &t) const;
    };

    // Continuous fields lerp toward target; topology fields are hard-copied.
    struct Params {
        int cols = 6, rows = 3;
        float thumbW = 300, thumbH = 169, gapX = 8, gapY = 8;
        float cornerRadius = 6, borderWidth = 2;
        Kind layout = Uniform;
        float stagger = 0.5f, selectedScale = 1.0f, flowWave = 0, flowFrequency = 1;
        float scatter = 0, scaleVariance = 0, cylinderBend = 0.65f, cylinderRadius = 720;
        Stage stage;

        float cellW() const { return thumbW + gapX; }
        float cellH() const { return thumbH + gapY; }
        float spanW(int c) const { return thumbW * c + gapX * (c > 1 ? c - 1 : 0); }
        float spanH(int r) const { return thumbH * r + gapY * (r > 1 ? r - 1 : 0); }
        float totalW() const { return spanW(cols); }
        float totalH() const { return spanH(rows); }
        void cylinderTransform(float x, float y, float ox, float &outX, float &outY,
                               float &outScale) const;
        void morphToward(const Params &t, float amt);
        bool settledTo(const Params &t) const;
    };

    struct Cell {
        int row;
        float cx, cy, hw, hh;
    };
    struct Place {
        int idx;
        float x, y, w, h;
    };

    Params readParams(const LayoutContext &ctx) const;
    Stage readStage(const LayoutContext &ctx) const;
    void placements(const LayoutContext &ctx, float totalW);
    QPointF compositionCenter(const LayoutContext &ctx) const;
    void gridFollow(int idx);
    void emitCell(const LayoutContext &ctx, std::vector<CardVisual> &out, int idx, float px,
                  float py, float hw, float hh, float entrance);

    Params m_live;
    Params m_target;
    int m_settingsCols = 6;
    float m_preferredThumbW = 300;
    float m_preferredThumbH = 169;
    float m_viewHeight = 0;
    bool m_haveParams = false;
    bool m_snapCamera = true;
    Spring m_camera;
    float m_maxScroll = 0;
    QRectF m_frame;
    std::vector<std::pair<int, Spring>> m_hoverFades;
    std::vector<Cell> m_cells;
    std::vector<Place> m_places;
    std::vector<float> m_bottoms;
};
