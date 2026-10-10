#pragma once

#include "sandypass.h"
#include "texturetier.h"

#include <QMatrix4x4>
#include <QRectF>
#include <QSize>

#include <cstdint>
#include <memory>

class QRhi;
class QRhiBuffer;
class QRhiCommandBuffer;
class QRhiGraphicsPipeline;
class QRhiRenderPassDescriptor;
class QRhiRenderTarget;
class QRhiResourceUpdateBatch;
class QRhiSampler;
class QRhiShaderResourceBindings;
class QRhiTexture;
class QRhiTextureRenderTarget;

// Drawn inline in the card pass, or offscreen at a lower resolution and blitted up.
class SandyRenderer
{
public:
    SandyRenderer();
    ~SandyRenderer();

    void prepare(QRhi *rhi, QRhiCommandBuffer *cb, QRhiRenderTarget *mainTarget, const SandyPass &pass,
                 TextureTier &near, TextureTier &far, const QMatrix4x4 &mvp, float opacity);
    void render(QRhiCommandBuffer *cb, QSize pixelSize);
    void releaseResources();
    bool active() const { return m_hasPass; }

    // Not owned; nullptr clears and binds a transparent dummy. incomingUv is
    // the part of the incoming texture a frame covers, in texture space.
    void setPreviewTextures(QRhiTexture *incoming, QRhiTexture *outgoing, const QRectF &incomingUv);

private:
    struct Resolved {
        float layer = 0;
        float tier = 0;   // 0 near, 1 far
        float uv[4] = {0, 0, 1, 1};
        bool ok = false;
    };

    Resolved resolve(TextureTier &near, TextureTier &far, const QString &key) const;
    bool ensureStatics(QRhi *rhi, QRhiResourceUpdateBatch *batch);
    void ensureFieldBindings(QRhi *rhi, QRhiTexture *nearTex, QRhiTexture *farTex, QRhiTexture *prevTex,
                             QRhiTexture *prevOutTex);
    void ensureInlinePipeline(QRhi *rhi, QRhiRenderPassDescriptor *rp, int samples);
    void ensureBlitBindings(QRhi *rhi, QRhiTexture *tex);
    void ensureBlitPipeline(QRhi *rhi, QRhiRenderPassDescriptor *rp, int samples);
    bool ensureOffscreen(QRhi *rhi, QRhiResourceUpdateBatch *batch, QSize size);
    void buildFieldPipeline(QRhi *rhi, QRhiRenderPassDescriptor *rp, int samples,
                            std::unique_ptr<QRhiGraphicsPipeline> &out);
    void releaseOffscreen();
    void drawField(QRhiCommandBuffer *cb) const;

    bool m_hasPass = false;
    bool m_scaled = false;
    uint32_t m_vertexCount = 0;
    bool m_halfDraw = false;

    QRhiTexture *m_prevIncoming = nullptr;  // not owned
    QRhiTexture *m_prevOutgoing = nullptr;  // not owned
    QRectF m_prevIncomingUv{0, 0, 1, 1};

    std::unique_ptr<QRhiBuffer> m_uniform;
    std::unique_ptr<QRhiSampler> m_sampler;
    std::unique_ptr<QRhiTexture> m_dummy;

    std::unique_ptr<QRhiShaderResourceBindings> m_fieldBindings;
    QRhiTexture *m_boundNear = nullptr;
    QRhiTexture *m_boundFar = nullptr;
    QRhiTexture *m_boundPrev = nullptr;
    QRhiTexture *m_boundPrevOut = nullptr;

    std::unique_ptr<QRhiGraphicsPipeline> m_inlinePipeline;
    std::unique_ptr<QRhiRenderPassDescriptor> m_inlineRpOwned;
    int m_inlineSamples = -1;

    std::unique_ptr<QRhiGraphicsPipeline> m_offscreenPipeline;
    std::unique_ptr<QRhiTexture> m_offscreen;
    std::unique_ptr<QRhiTextureRenderTarget> m_offRT;
    std::unique_ptr<QRhiRenderPassDescriptor> m_offRp;
    QSize m_offSize;

    std::unique_ptr<QRhiGraphicsPipeline> m_blitPipeline;
    std::unique_ptr<QRhiRenderPassDescriptor> m_blitRpOwned;
    std::unique_ptr<QRhiShaderResourceBindings> m_blitBindings;
    QRhiTexture *m_blitBound = nullptr;
    int m_blitSamples = -1;
};
