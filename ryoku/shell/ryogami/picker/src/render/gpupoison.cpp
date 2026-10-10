#include "gpupoison.h"

#include <QByteArray>
#include <QImage>
#include <QtGlobal>
#include <rhi/qrhi.h>

#include <algorithm>
#include <cstdint>
#include <vector>

namespace GpuPoison {

bool enabled()
{
    static const bool value = qEnvironmentVariableIntValue("RYOGAMI_POISON_GPU") == 1;
    return value;
}

void buffer(QRhiResourceUpdateBatch *batch, QRhiBuffer *buffer)
{
    if (!enabled() || !batch || !buffer || buffer->size() <= 0)
        return;
    QByteArray bytes(buffer->size(), char(0));
    uint32_t state = 0x6d2b79f5u;
    for (qsizetype i = 0; i < bytes.size(); ++i) {
        state ^= state << 13;
        state ^= state >> 17;
        state ^= state << 5;
        bytes[i] = char(state & 0xffu);
    }
    batch->updateDynamicBuffer(buffer, 0, buffer->size(), bytes.constData());
}

void texture(QRhiResourceUpdateBatch *batch, QRhiTexture *texture, QSize size,
             int layers, int mipLevels)
{
    if (!enabled() || !batch || !texture)
        return;

    layers = std::max(layers, 1);
    mipLevels = std::max(mipLevels, 1);
    std::vector<QImage> images;
    std::vector<QRhiTextureUploadEntry> entries;
    images.reserve(std::size_t(layers * mipLevels));
    entries.reserve(std::size_t(layers * mipLevels));

    for (int layer = 0; layer < layers; ++layer) {
        for (int level = 0; level < mipLevels; ++level) {
            const QSize levelSize(std::max(1, size.width() >> level),
                                  std::max(1, size.height() >> level));
            QImage image(levelSize, QImage::Format_RGBA8888);
            for (int y = 0; y < levelSize.height(); ++y) {
                auto *row = reinterpret_cast<QRgb *>(image.scanLine(y));
                for (int x = 0; x < levelSize.width(); ++x) {
                    const bool odd = ((x >> 3) ^ (y >> 3) ^ layer ^ level) & 1;
                    row[x] = odd ? qRgba(255, 0, 193, 255) : qRgba(23, 255, 0, 255);
                }
            }
            images.push_back(std::move(image));
            QRhiTextureSubresourceUploadDescription desc(images.back());
            entries.emplace_back(layer, level, desc);
        }
    }

    QRhiTextureUploadDescription upload;
    upload.setEntries(entries.cbegin(), entries.cend());
    batch->uploadTexture(texture, upload);
}

} // namespace GpuPoison
