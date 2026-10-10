#include "previewyuv.h"

#include <algorithm>
#include <cmath>

namespace {

struct Coefficients {
    float yScale, yOffset, rv, gu, gv, bu;
};

// BT.601 and BT.709 matrices, limited (16..235) and full range.
Coefficients coefficientsFor(bool fullRange, bool bt709)
{
    if (fullRange)
        return bt709 ? Coefficients{1.0f, 0.0f, 1.5748f, 0.1873f, 0.4681f, 1.8556f}
                     : Coefficients{1.0f, 0.0f, 1.402f, 0.344f, 0.714f, 1.772f};
    return bt709 ? Coefficients{1.164f, 16.0f, 1.793f, 0.213f, 0.533f, 2.112f}
                 : Coefficients{1.164f, 16.0f, 1.596f, 0.392f, 0.813f, 2.017f};
}

inline unsigned char clampByte(float v)
{
    return static_cast<unsigned char>(std::clamp(v, 0.0f, 255.0f) + 0.5f);
}

QSize fitWithin(QSize size, QSize cap)
{
    if (cap.isEmpty() || (size.width() <= cap.width() && size.height() <= cap.height()))
        return size;
    QSize fitted = size.scaled(cap, Qt::KeepAspectRatio);
    return QSize(std::max(1, fitted.width()), std::max(1, fitted.height()));
}

} // namespace

QImage previewImageFromYuv(const PreviewYuvPlanes &p, QSize cap)
{
    if (p.size.isEmpty() || !p.y || !p.u || !p.v || p.yStride <= 0 || p.uStride <= 0 || p.vStride <= 0
        || p.uvStep <= 0 || p.subX < 0 || p.subY < 0)
        return QImage();

    const QSize out = fitWithin(p.size, cap);
    QImage image(out, QImage::Format_RGBA8888);
    if (image.isNull())
        return image;

    const Coefficients c = coefficientsFor(p.fullRange, p.bt709);
    const int sw = p.size.width(), sh = p.size.height();
    const int ow = out.width(), oh = out.height();
    // Nearest sampling: a preview sits far smaller than its clip, and the
    // conversion runs for every frame the decoder delivers.
    for (int yo = 0; yo < oh; ++yo) {
        const int ys = static_cast<int>((static_cast<long long>(yo) * sh) / oh);
        const unsigned char *yRow = p.y + static_cast<std::ptrdiff_t>(ys) * p.yStride;
        const unsigned char *uRow = p.u + static_cast<std::ptrdiff_t>(ys >> p.subY) * p.uStride;
        const unsigned char *vRow = p.v + static_cast<std::ptrdiff_t>(ys >> p.subY) * p.vStride;
        unsigned char *dst = image.scanLine(yo);
        for (int xo = 0; xo < ow; ++xo) {
            const int xs = static_cast<int>((static_cast<long long>(xo) * sw) / ow);
            const int xc = (xs >> p.subX) * p.uvStep;
            const float yv = (static_cast<float>(yRow[xs]) - c.yOffset) * c.yScale;
            const float d = static_cast<float>(uRow[xc]) - 128.0f;
            const float e = static_cast<float>(vRow[xc]) - 128.0f;
            dst[0] = clampByte(yv + c.rv * e);
            dst[1] = clampByte(yv - c.gu * d - c.gv * e);
            dst[2] = clampByte(yv + c.bu * d);
            dst[3] = 255;
            dst += 4;
        }
    }
    return image;
}
