#include "decklayout.h"

#include "../scene/params.h"

#include <cmath>
#include <cstdio>
#include <vector>

namespace {

bool near(double a, double b)
{
    return std::abs(a - b) < 0.001;
}

} // namespace

int main()
{
    DeckLayout layout;
    if (layout.key() != QLatin1String("deck")) {
        std::fprintf(stderr, "deck layout has the wrong key\n");
        return 1;
    }

    MotionProfile motion;
    LayoutContext ctx;
    ctx.count = 12;
    ctx.current = 5;
    ctx.motion = &motion;

    layout.configure(ctx, false);
    layout.reset(ctx);
    layout.select(ctx, 2, 5);

    std::vector<CardVisual> cards;
    layout.build(ctx, cards);
    if (!cards.empty()) {
        std::fprintf(stderr, "deck layout emitted GPU cards; the QML scene owns them\n");
        return 1;
    }
    if (layout.tick(ctx, 1.0 / 60.0)) {
        std::fprintf(stderr, "deck layout still reports motion\n");
        return 1;
    }
    if (layout.hitTest(QPointF(10, 10)) != -1) {
        std::fprintf(stderr, "deck layout claimed a hit outside the QML scene\n");
        return 1;
    }
    if (!layout.cardRect(3).isEmpty()) {
        std::fprintf(stderr, "deck layout returned a card rectangle\n");
        return 1;
    }
    if (layout.step(ctx, -1, 0) != 4 || layout.step(ctx, 1, 0) != 6) {
        std::fprintf(stderr, "deck navigation did not step one row\n");
        return 1;
    }
    if (layout.step(ctx, -1, 0) != 0) {
        ctx.current = 0;
        if (layout.step(ctx, -1, 0) != 0) {
            std::fprintf(stderr, "deck navigation did not clamp\n");
            return 1;
        }
    }

    layout.reset(ctx);
    if (layout.wheel(ctx, QPointF(0, 120), QPointF()) != -1
        || layout.wheel(ctx, QPointF(0, -120), QPointF()) != 1) {
        std::fprintf(stderr, "deck wheel did not step one row per notch\n");
        return 1;
    }
    if (!near(layout.cameraMs(ctx), motion.ms(MotionProfile::Standard))) {
        std::fprintf(stderr, "deck camera timing left the motion profile\n");
        return 1;
    }
    return 0;
}
