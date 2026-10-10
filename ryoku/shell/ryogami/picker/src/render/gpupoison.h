#pragma once

#include <QSize>

class QRhiBuffer;
class QRhiResourceUpdateBatch;
class QRhiTexture;

namespace GpuPoison {

bool enabled();
void buffer(QRhiResourceUpdateBatch *batch, QRhiBuffer *buffer);
void texture(QRhiResourceUpdateBatch *batch, QRhiTexture *texture, QSize size,
             int layers = 1, int mipLevels = 1);

} // namespace GpuPoison
