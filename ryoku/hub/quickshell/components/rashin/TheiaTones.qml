import QtQuick
import Ryoku.Ui.Singletons

TheiaBubblePicker {
    limit: 23
    ordered: false
    choices: [
        { id: "formal", label: I18n.tr("Formal") },
        { id: "confident", label: I18n.tr("Confident") },
        { id: "friendly", label: I18n.tr("Friendly") },
        { id: "diplomatic", label: I18n.tr("Diplomatic") },
        { id: "direct", label: I18n.tr("Direct") },
        { id: "empathetic", label: I18n.tr("Empathetic") },
        { id: "assertive", label: I18n.tr("Assertive") },
        { id: "constructive", label: I18n.tr("Constructive") },
        { id: "engaging", label: I18n.tr("Engaging") },
        { id: "exciting", label: I18n.tr("Exciting") },
        { id: "informative", label: I18n.tr("Informative") },
        { id: "inspirational", label: I18n.tr("Inspirational") },
        { id: "objective", label: I18n.tr("Objective") },
        { id: "persuasive", label: I18n.tr("Persuasive") },
        { id: "academic", label: I18n.tr("Academic") },
        { id: "casual", label: I18n.tr("Casual") },
        { id: "compassionate", label: I18n.tr("Compassionate") },
        { id: "cooperative", label: I18n.tr("Cooperative") },
        { id: "fluent", label: I18n.tr("Fluent") },
        { id: "professional", label: I18n.tr("Professional") },
        { id: "witty", label: I18n.tr("Witty") },
        { id: "personable", label: I18n.tr("Personable") },
        { id: "warm", label: I18n.tr("Warm") }
    ]
}
