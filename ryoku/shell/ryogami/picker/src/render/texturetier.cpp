#include "texturetier.h"
#include "gpupoison.h"

#include <rhi/qrhi.h>

#include <algorithm>
#include <cmath>

TextureTier::TextureTier(QSize layerSize, QSize tileSize, int initialLayers, int maxLayers, int mipLevels)
    : m_layerSize(layerSize)
    , m_tile(tileSize)
    , m_layers(std::max(1, std::min(initialLayers, maxLayers)))
    , m_maxLayers(std::max(1, maxLayers))
    , m_mipLevels(std::max(1, mipLevels))
{
    m_tiles.resize(size_t(tileCount()));
    for (int i = tileCount() - 1; i >= 0; --i)
        m_free.push_back(i);
}

TextureTier::~TextureTier() = default;

int TextureTier::tilesPerLayer() const
{
    return std::max(1, m_layerSize.width() / m_tile.width()) * std::max(1, m_layerSize.height() / m_tile.height());
}

QRect TextureTier::tileRect(int tile) const
{
    const int perLayer = tilesPerLayer();
    const int cols = std::max(1, m_layerSize.width() / m_tile.width());
    const int idx = tile % perLayer;
    return QRect((idx % cols) * m_tile.width(), (idx / cols) * m_tile.height(), m_tile.width(), m_tile.height());
}

const TextureTier::Slot *TextureTier::find(const QString &key)
{
    const auto it = m_index.constFind(key);
    if (it == m_index.constEnd())
        return nullptr;
    Tile &t = m_tiles[size_t(*it)];
    t.lastUsed = m_frame;
    return &t.slot;
}

int TextureTier::acquireTile()
{
    if (!m_free.empty()) {
        const int tile = m_free.back();
        m_free.pop_back();
        return tile;
    }
    int victim = -1;
    quint64 oldest = m_frame - 1;
    for (size_t i = 0; i < m_tiles.size(); ++i) {
        if (m_tiles[i].lastUsed < oldest) {
            oldest = m_tiles[i].lastUsed;
            victim = int(i);
        }
    }
    if (victim >= 0) {
        m_index.remove(m_tiles[size_t(victim)].key);
        m_pending.erase(std::remove_if(m_pending.begin(), m_pending.end(),
                                       [victim](const Pending &p) { return p.tile == victim; }),
                        m_pending.end());
    }
    return victim;
}

bool TextureTier::admit(TierImage &&image)
{
    if (image.key.isEmpty() || image.levels.empty() || image.levels.front().isNull()
        || m_index.contains(image.key))
        return false;
    const int requiredLevels = m_mipLevels > 1
        ? int(std::floor(std::log2(std::max(m_tile.width(), m_tile.height())))) + 1
        : 1;
    image.levels.resize(std::max(int(image.levels.size()), requiredLevels));
    for (int level = 0; level < requiredLevels; ++level) {
        const QSize expected(std::max(1, m_tile.width() >> level),
                             std::max(1, m_tile.height() >> level));
        if (image.levels[size_t(level)].isNull()) {
            if (level == 0)
                return false;
            image.levels[size_t(level)] = image.levels[size_t(level - 1)].scaled(
                expected, Qt::IgnoreAspectRatio, Qt::SmoothTransformation);
        } else if (image.levels[size_t(level)].size() != expected) {
            image.levels[size_t(level)] = image.levels[size_t(level)].scaled(
                expected, Qt::IgnoreAspectRatio, Qt::SmoothTransformation);
        }
    }
    int tile = acquireTile();
    if (tile < 0 && m_layers < m_maxLayers) {
        const int grown = std::min(m_maxLayers, m_layers * 2);
        const int before = tileCount();
        m_layers = grown;
        m_tiles.resize(size_t(tileCount()));
        for (int i = tileCount() - 1; i >= before; --i)
            m_free.push_back(i);
        tile = acquireTile();
    }
    if (tile < 0)
        return false;

    const QRect r = tileRect(tile);
    const QSize content(std::clamp(image.content.width(), 1, r.width()),
                        std::clamp(image.content.height(), 1, r.height()));
    Tile &t = m_tiles[size_t(tile)];
    t.key = image.key;
    t.lastUsed = m_frame;
    t.slot.layer = tile / tilesPerLayer();
    // The slot covers the content inside the tile; the padding beyond it is
    // edge smear, defined for the sampler but never meant to be seen.
    t.slot.uv = QRectF((r.x() + 0.5) / m_layerSize.width(), (r.y() + 0.5) / m_layerSize.height(),
                       (content.width() - 1.0) / m_layerSize.width(),
                       (content.height() - 1.0) / m_layerSize.height());
    m_index.insert(t.key, tile);
    m_pending.push_back(Pending{tile, std::move(image.levels)});
    return true;
}

void TextureTier::beginFrame()
{
    ++m_frame;
}

bool TextureTier::commit(QRhi *rhi, QRhiResourceUpdateBatch *batch)
{
    bool changed = false;
    if (!m_texture || m_textureLayers < m_layers) {
        const QRhiTexture::Flags flags = m_mipLevels > 1 ? QRhiTexture::MipMapped : QRhiTexture::Flags();
        std::unique_ptr<QRhiTexture> tex(rhi->newTextureArray(QRhiTexture::RGBA8, m_layers, m_layerSize, 1, flags));
        if (!tex->create()) {
            // The array could not grow (a driver layer cap or GPU memory pressure). Fall back
            // to the size that still exists instead of leaving tiles pointing at layers the
            // live texture never got, which would show as cards that never paint until relaunch.
            if (m_textureLayers < m_layers) {
                m_layers = std::max(1, m_textureLayers);
                const int cap = tileCount();
                m_tiles.resize(size_t(cap));
                for (auto it = m_index.begin(); it != m_index.end();) {
                    if (it.value() >= cap)
                        it = m_index.erase(it);
                    else
                        ++it;
                }
                m_pending.erase(std::remove_if(m_pending.begin(), m_pending.end(),
                                               [cap](const Pending &p) { return p.tile >= cap; }),
                                m_pending.end());
                std::vector<bool> used(size_t(cap), false);
                for (auto it = m_index.constBegin(); it != m_index.constEnd(); ++it)
                    used[size_t(it.value())] = true;
                m_free.clear();
                for (int i = cap - 1; i >= 0; --i)
                    if (!used[size_t(i)])
                        m_free.push_back(i);
            }
            return changed;
        }
        const int levels = m_mipLevels > 1 ? rhi->mipLevelsForSize(m_layerSize) : 1;
        GpuPoison::texture(batch, tex.get(), m_layerSize, m_layers, levels);
        if (m_texture) {
            const int levels = m_mipLevels > 1 ? rhi->mipLevelsForSize(m_layerSize) : 1;
            for (int layer = 0; layer < m_textureLayers; ++layer) {
                for (int level = 0; level < levels; ++level) {
                    QRhiTextureCopyDescription copy;
                    copy.setSourceLayer(layer);
                    copy.setDestinationLayer(layer);
                    copy.setSourceLevel(level);
                    copy.setDestinationLevel(level);
                    batch->copyTexture(tex.get(), m_texture.get(), copy);
                }
            }
            // The old texture may still be referenced by commands in flight.
            m_texture.release()->deleteLater();
        }
        m_texture = std::move(tex);
        m_textureLayers = m_layers;
        changed = true;
    }

    if (m_pending.empty())
        return changed;

    const int levels = m_mipLevels > 1 ? rhi->mipLevelsForSize(m_layerSize) : 1;
    std::vector<QRhiTextureUploadEntry> entries;
    entries.reserve(m_pending.size() * size_t(levels));
    for (Pending &p : m_pending) {
        const QRect r = tileRect(p.tile);
        const int layer = p.tile / tilesPerLayer();
        for (int level = 0; level < levels && level < int(p.levels.size()); ++level) {
            QRhiTextureSubresourceUploadDescription desc(p.levels[size_t(level)]);
            desc.setDestinationTopLeft(QPoint(r.x() >> level, r.y() >> level));
            entries.emplace_back(layer, level, desc);
        }
    }
    QRhiTextureUploadDescription upload;
    upload.setEntries(entries.cbegin(), entries.cend());
    batch->uploadTexture(m_texture.get(), upload);
    m_pending.clear();
    return changed;
}

void TextureTier::releaseResources()
{
    m_texture.reset();
    m_textureLayers = 0;
    m_index.clear();
    m_pending.clear();
    m_free.clear();
    for (Tile &t : m_tiles)
        t = Tile{};
    for (int i = tileCount() - 1; i >= 0; --i)
        m_free.push_back(i);
}
