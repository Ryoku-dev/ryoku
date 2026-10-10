#pragma once

#include "cardinstance.h"
#include "sandyrenderer.h"
#include "texturetier.h"
#include "transitionpass.h"

#include <QColor>
#include <QImage>
#include <QPointF>
#include <QRectF>
#include <QSGRenderNode>
#include <QSize>

#include <memory>
#include <vector>

class QQuickWindow;
class QRhi;
class QRhiBuffer;
class QRhiGraphicsPipeline;
class QRhiRenderPassDescriptor;
class QRhiResourceUpdateBatch;
class QRhiSampler;
class QRhiShaderResourceBindings;
class QRhiTexture;
struct SandyPass;

class CardRenderNode : public QSGRenderNode
{
public:
    explicit CardRenderNode(QQuickWindow *window, QSize nearLayer = QSize(1024, 576));
    ~CardRenderNode() override;

    // Sync-time API: the GUI thread is blocked while these run.
    TextureTier &nearTier() { return m_near; }
    TextureTier &farTier() { return m_far; }
    QSize nearTileSize() const { return m_near.tileSize(); }
    void beginFrame();
    void setInstances(std::vector<CardInstance> &&instances);
    // An empty set means no transition.
    void setTransition(std::vector<CardInstance> &&from, float progress, int kind);
    // Epicenter (kind 1) and glow accent (kinds 1-2); kind 0 needs neither.
    void setTransitionParams(QPointF origin, const QColor &accent);
    // The pass is copied, not referenced: the layout that produced it can be
    // destroyed on the GUI thread while this node still renders the frame.
    void setSandyPass(const SandyPass *pass) {
        if (pass) {
            m_sandyPassData = *pass;
            m_sandyPassActive = true;
        } else {
            m_sandyPassActive = false;
        }
    }
    void setScene(const QRectF &bounds, const QRectF &clip, float time, float vis);
    void setPreviewImage(const QImage &image);
    bool hasPreview() const { return !m_previewImage.isNull() || m_previewUploaded; }
    // The part of the preview texture the latest frame covers, in texture space.
    QRectF previewUv() const { return m_previewUv; }

    void prepare() override;
    void render(const RenderState *state) override;
    void releaseResources() override;
    RenderingFlags flags() const override;
    QRectF rect() const override;

private:
    bool ensurePreview(QRhi *rhi, QRhiResourceUpdateBatch *batch);
    bool ensureBindings(QRhi *rhi, bool texturesChanged);
    void sanitize(std::vector<CardInstance> &instances, int nearLayers, int farLayers);
    void configureCardPipeline(QRhiGraphicsPipeline *pipeline);
    void buildPipeline(QRhi *rhi);
    void buildScenePipeline(QRhi *rhi);
    quint32 updateInstanceBuffer(std::unique_ptr<QRhiBuffer> &buffer, QRhi *rhi,
                                 QRhiResourceUpdateBatch *batch,
                                 const std::vector<CardInstance> &data);
    bool transitionActive() const { return !m_transitionFrom.empty(); }

    QQuickWindow *m_window;
    TextureTier m_near;
    TextureTier m_far;
    std::vector<CardInstance> m_instances;
    QRectF m_bounds;
    QRectF m_clip;
    float m_time = 0;
    float m_vis = 1;

    std::vector<CardInstance> m_transitionFrom;
    float m_transitionProgress = 0;
    int m_transitionKind = 0;
    QPointF m_transitionOrigin{0.5, 0.5};
    QColor m_transitionAccent{255, 122, 102};
    bool m_didTransition = false;

    QImage m_previewImage;
    QRectF m_previewUv{0, 0, 1, 1};
    bool m_previewUploaded = false;

    std::unique_ptr<QRhiBuffer> m_instanceBuffer;
    std::unique_ptr<QRhiBuffer> m_fromBuffer;
    std::unique_ptr<QRhiBuffer> m_uniformBuffer;
    quint32 m_uploadedInstanceCount = 0;
    quint32 m_uploadedFromCount = 0;
    std::unique_ptr<QRhiSampler> m_mipSampler;
    std::unique_ptr<QRhiSampler> m_flatSampler;
    std::unique_ptr<QRhiTexture> m_preview;
    std::unique_ptr<QRhiShaderResourceBindings> m_bindings;
    std::unique_ptr<QRhiGraphicsPipeline> m_pipeline;
    std::unique_ptr<QRhiGraphicsPipeline> m_scenePipeline;
    std::unique_ptr<QRhiRenderPassDescriptor> m_pipelinePass;
    int m_pipelineSamples = 0;
    TransitionPass m_transition;
    SandyRenderer m_sandy;
    SandyPass m_sandyPassData;
    bool m_sandyPassActive = false;
};
