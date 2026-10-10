#include "../render/previewyuv.h"

#include <cstdlib>
#include <vector>

namespace {

// A 4x2 YUV420P frame: left half pure limited-range red, right half pure blue.
struct Frame {
    std::vector<unsigned char> y, u, v;
    PreviewYuvPlanes planes;
    Frame()
        : y{81, 81, 41, 41, 81, 81, 41, 41}, u{90, 240}, v{240, 110}
    {
        planes.size = QSize(4, 2);
        planes.y = y.data();
        planes.u = u.data();
        planes.v = v.data();
        planes.yStride = 4;
        planes.uStride = planes.vStride = 2;
    }
};

bool near(QRgb px, int r, int g, int b)
{
    return std::abs(qRed(px) - r) <= 6 && std::abs(qGreen(px) - g) <= 6 && std::abs(qBlue(px) - b) <= 6;
}

} // namespace

int main()
{
    Frame f;
    const QImage img = previewImageFromYuv(f.planes, QSize(640, 360));
    if (img.size() != QSize(4, 2) || img.format() != QImage::Format_RGBA8888)
        return 1;
    if (!near(img.pixel(0, 0), 255, 0, 0) || !near(img.pixel(3, 1), 0, 0, 255))
        return 2;

    // NV12: the same chroma interleaved in one plane, read through uvStep.
    std::vector<unsigned char> uv{90, 240, 240, 110};
    PreviewYuvPlanes nv = f.planes;
    nv.u = uv.data();
    nv.v = uv.data() + 1;
    nv.uStride = nv.vStride = 4;
    nv.uvStep = 2;
    const QImage nvImg = previewImageFromYuv(nv, QSize(640, 360));
    if (!near(nvImg.pixel(1, 1), 255, 0, 0) || !near(nvImg.pixel(2, 0), 0, 0, 255))
        return 3;

    // A frame wider than the cap is fitted, keeping its aspect; a smaller one is never enlarged.
    PreviewYuvPlanes wide = f.planes;
    std::vector<unsigned char> y(1280 * 720, 81), c(640 * 360, 128);
    wide.size = QSize(1280, 720);
    wide.y = y.data();
    wide.u = wide.v = c.data();
    wide.yStride = 1280;
    wide.uStride = wide.vStride = 640;
    if (previewImageFromYuv(wide, QSize(640, 360)).size() != QSize(640, 360))
        return 4;
    if (previewImageFromYuv(f.planes, QSize(640, 360)).size() != QSize(4, 2))
        return 5;

    // Planes that do not describe a frame give nothing rather than reading past them.
    PreviewYuvPlanes bad = f.planes;
    bad.v = nullptr;
    if (!previewImageFromYuv(bad, QSize(640, 360)).isNull())
        return 6;
    bad = f.planes;
    bad.size = QSize();
    if (!previewImageFromYuv(bad, QSize(640, 360)).isNull())
        return 7;
    return 0;
}
