#pragma once

#include <QImage>
#include <QSize>

// A decoded video frame's planes, as the preview converter reads them. Chroma
// is one sample per (1 << subX) x (1 << subY) luma block; uvStep is the byte
// distance between consecutive chroma samples in a row (1 planar, 2 interleaved
// NV12/NV21, where u and v point into the same plane one byte apart).
struct PreviewYuvPlanes {
    QSize size;
    const unsigned char *y = nullptr;
    const unsigned char *u = nullptr;
    const unsigned char *v = nullptr;
    int yStride = 0;
    int uStride = 0;
    int vStride = 0;
    int uvStep = 1;
    int subX = 1;
    int subY = 1;
    bool fullRange = false;
    bool bt709 = false;
};

// The frame as RGBA8888, downscaled to fit within cap (aspect kept, never
// enlarged). Null when the planes or sizes do not describe a frame.
QImage previewImageFromYuv(const PreviewYuvPlanes &planes, QSize cap);
