#include "font.hpp"

#include "appearanceconfig.hpp"

namespace caelestia::config {

QFont FontStyleBase::large() const {
    return m_large;
}

QFont FontStyleBase::medium() const {
    return m_medium;
}

QFont FontStyleBase::small() const {
    return m_small;
}

QFont FontStyleBase::buildFont(const FontConfigNode* cfg, const QString& fallbackFamily, qreal scale) {
    QFont font;
    // Both empty: leave the default-constructed family alone so the font follows the
    // system instead of carrying an empty family string around.
    const auto family = cfg->family().isEmpty() ? fallbackFamily : cfg->family();
    if (!family.isEmpty())
        font.setFamily(family);
    const int scaledSize = static_cast<int>(cfg->size() * scale);
    const int cappedSize = scaledSize > 0 ? scaledSize : 1;
    font.setPointSize(cappedSize);
    font.setVariableAxis("opsz", static_cast<float>(cappedSize));
    font.setWeight(static_cast<QFont::Weight>(cfg->weight()));
    font.setVariableAxis("wght", font.weight());
    font.setItalic(cfg->italic());

    const auto vaxes = cfg->vaxes();
    for (auto it = vaxes.constBegin(); it != vaxes.constEnd(); ++it) {
        if (auto tag = QFont::Tag::fromString(it.key()))
            font.setVariableAxis(*tag, it.value().toFloat());
    }

    return font;
}

void FontStyleBase::bind(FontStyleNode* cfg) {
    if (m_cfg == cfg)
        return;

    if (m_cfg) {
        disconnect(m_cfg, nullptr, this, nullptr);
        disconnect(m_cfg->large(), nullptr, this, nullptr);
        disconnect(m_cfg->medium(), nullptr, this, nullptr);
        disconnect(m_cfg->small(), nullptr, this, nullptr);
    }

    m_cfg = cfg;

    if (cfg) {
        connect(cfg, &settings::Node::optionChanged, this, &FontStyleBase::rebuild);
        connect(cfg->large(), &settings::Node::optionChanged, this, &FontStyleBase::rebuild);
        connect(cfg->medium(), &settings::Node::optionChanged, this, &FontStyleBase::rebuild);
        connect(cfg->small(), &settings::Node::optionChanged, this, &FontStyleBase::rebuild);
    }

    rebuild();
}

void FontStyleBase::rebuild() {
    QFont large;
    QFont medium;
    QFont small;

    if (m_cfg) {
        const auto family = m_cfg->family();
        large = buildFont(m_cfg->large(), family, m_scale);
        medium = buildFont(m_cfg->medium(), family, m_scale);
        small = buildFont(m_cfg->small(), family, m_scale);
    }

    // Emitting unconditionally invalidates every binding that reads a font (directly or
    // through the builder chain), even when the built fonts are identical. Only notify
    // when something actually changed; QFont::operator== compares the variable axes too.
    if (large == m_large && medium == m_medium && small == m_small)
        return;

    m_large = large;
    m_medium = medium;
    m_small = small;
    emit fontsChanged();
}

void FontStyleBase::setScale(qreal scale) {
    if (qFuzzyCompare(m_scale + 1.0, scale + 1.0))
        return;
    m_scale = scale;
    rebuild();
}

FontStyle::FontStyle(QObject* parent)
    : FontStyleBase(parent)
    , m_builders(new FontBuilders(this, this)) {}

FontBuilders* FontStyle::builders() const {
    return m_builders;
}

IconFontStyle::IconFontStyle(QObject* parent)
    : FontStyleBase(parent)
    , m_builders(new IconFontBuilders(this, this)) {}

FontBuilder IconFontStyle::size(int pointSize) {
    return FontBuilder(m_small).size(pointSize);
}

void IconFontStyle::bind(FontStyleNode* cfg) {
    if (m_cfg == cfg)
        return;

    if (auto* previous = qobject_cast<FontStyleIconNode*>(m_cfg))
        disconnect(previous->extraLarge(), nullptr, this, nullptr);

    FontStyleBase::bind(cfg);

    if (auto* icon = qobject_cast<FontStyleIconNode*>(cfg))
        connect(icon->extraLarge(), &settings::Node::optionChanged, this, &IconFontStyle::rebuild);
}

QFont IconFontStyle::extraLarge() const {
    return m_extraLarge;
}

IconFontBuilders* IconFontStyle::builders() const {
    return m_builders;
}

void IconFontStyle::rebuild() {
    QFont large;
    QFont medium;
    QFont small;
    QFont extraLarge;

    if (m_cfg) {
        const auto family = m_cfg->family();
        large = buildFont(m_cfg->large(), family, m_scale);
        medium = buildFont(m_cfg->medium(), family, m_scale);
        small = buildFont(m_cfg->small(), family, m_scale);

        // Only the icon node carries the fourth size; the cast is the one place the icon
        // style needs more than FontStyleNode offers.
        const auto* icon = qobject_cast<FontStyleIconNode*>(m_cfg);
        extraLarge = icon ? buildFont(icon->extraLarge(), family, m_scale) : QFont();
    }

    if (large == m_large && medium == m_medium && small == m_small && extraLarge == m_extraLarge)
        return;

    m_large = large;
    m_medium = medium;
    m_small = small;
    m_extraLarge = extraLarge;
    emit fontsChanged();
}

FontBuilders::FontBuilders(const FontStyleBase* style, QObject* parent)
    : QObject(parent)
    , m_style(style) {
    connect(style, &FontStyleBase::fontsChanged, this, &FontBuilders::buildersChanged);
}

FontBuilder FontBuilders::large() const {
    return FontBuilder(m_style->large());
}

FontBuilder FontBuilders::medium() const {
    return FontBuilder(m_style->medium());
}

FontBuilder FontBuilders::small() const {
    return FontBuilder(m_style->small());
}

IconFontBuilders::IconFontBuilders(const IconFontStyle* style, QObject* parent)
    : FontBuilders(style, parent) {}

FontBuilder IconFontBuilders::extraLarge() const {
    return FontBuilder(static_cast<const IconFontStyle*>(m_style)->extraLarge());
}

FontTokens::FontTokens(QObject* parent)
    : QObject(parent)
    , m_headline(new FontStyle(this))
    , m_title(new FontStyle(this))
    , m_body(new FontStyle(this))
    , m_label(new FontStyle(this))
    , m_mono(new FontStyle(this))
    , m_icon(new IconFontStyle(this)) {}

FontStyle* FontTokens::headline() const {
    return m_headline;
}

FontStyle* FontTokens::title() const {
    return m_title;
}

FontStyle* FontTokens::body() const {
    return m_body;
}

FontStyle* FontTokens::label() const {
    return m_label;
}

FontStyle* FontTokens::mono() const {
    return m_mono;
}

IconFontStyle* FontTokens::icon() const {
    return m_icon;
}

FontBuilder FontTokens::clock() const {
    return FontBuilder(m_clock);
}

QString FontTokens::workspaces() const {
    return m_font ? m_font->workspaces() : QString();
}

void FontTokens::bindFont(AppearanceFont* font) {
    if (m_font == font)
        return;

    if (m_font)
        disconnect(m_font, nullptr, this, nullptr);

    m_font = font;

    if (font) {
        rebuildScale();
        m_headline->bind(font->headline());
        m_title->bind(font->title());
        m_body->bind(font->body());
        m_label->bind(font->label());
        m_mono->bind(font->mono());
        m_icon->bind(font->icon());

        connect(font, &AppearanceFont::clockChanged, this, &FontTokens::rebuildClock);
        connect(font, &AppearanceFont::scaleChanged, this, &FontTokens::rebuildScale);
        connect(font, &AppearanceFont::workspacesChanged, this, &FontTokens::workspacesChanged);
    } else {
        rebuildScale();
        m_headline->bind(nullptr);
        m_title->bind(nullptr);
        m_body->bind(nullptr);
        m_label->bind(nullptr);
        m_mono->bind(nullptr);
        m_icon->bind(nullptr);
    }

    rebuildClock();
    emit workspacesChanged();
}

void FontTokens::rebuildScale() {
    const qreal s = m_font ? m_font->scale() : 1;
    m_headline->setScale(s);
    m_title->setScale(s);
    m_body->setScale(s);
    m_label->setScale(s);
    m_mono->setScale(s);
    m_icon->setScale(s);
}

void FontTokens::rebuildClock() {
    QFont f;
    if (m_font)
        f.setFamily(m_font->clock());
    m_clock = f;
    emit clockChanged();
}

void FontTokens::bindTokens(AppearanceTokens* tokens) {
    m_tokens = tokens;
}

} // namespace caelestia::config
