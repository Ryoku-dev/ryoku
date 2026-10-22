#pragma once

#include <qstring.h>

#include "../Settings/objectnode.hpp"
#include "common.hpp"

namespace caelestia::config {

using Qt::StringLiterals::operator""_s;
using settings::vmap;

class AiConfig : public settings::ObjectNode {
    CONFIG_NODE(AiConfig, settings::ObjectNode)

    CONFIG_PROPERTY(QString, ollamaUrl, u"http://localhost:11434"_s)
    CONFIG_PROPERTY(QString, ollamaModel, u"llama3"_s)

    CONFIG_PROPERTY(bool, saveChatHistory, true)
    CONFIG_PROPERTY(QString, ollamaHistoryJson, u"[]"_s)

    CONFIG_PROPERTY(bool, snapToDefaultOllama, true)
    CONFIG_PROPERTY(QString, defaultOllamaModel, u"llama3"_s)

    CONFIG_PROPERTY(QString, defaultProvider, u"ollama"_s)
    CONFIG_PROPERTY(bool, enableOllama, true)
    CONFIG_PROPERTY(bool, enableCelestialMode, true)
    CONFIG_PROPERTY(bool, showNews, true)
    CONFIG_PROPERTY(bool, showCaelestiaMode, true)
    CONFIG_PROPERTY(QString, orionModel, u"qwen3.5:9b"_s)

    CONFIG_PROPERTY(QString, activeProvider, u"ollama"_s)
    CONFIG_PROPERTY(QString, activeOllamaModel, u"llama3"_s)

    // Ryoku has its own assistant, Rashin, so the sidebar tab for this one starts off.
    CONFIG_PROPERTY(bool, enableAiAssistant, false)

    CONFIG_PROPERTY(bool, enableClaudeCode, true)
    CONFIG_PROPERTY(QString, claudeCodeBin, u"claude"_s)
    CONFIG_PROPERTY(QString, defaultClaudeCodeModel, u"default"_s)
    CONFIG_PROPERTY(QString, claudeCodeEffort, u"default"_s)
    CONFIG_PROPERTY(bool, claudeCodeSkipPermissions, false)

    CONFIG_PROPERTY(QString, claudeAccountsJson, u"[]"_s)
    CONFIG_PROPERTY(QString, activeClaudeAccount, u""_s)
    CONFIG_PROPERTY(QString, loginTerminal, u"konsole"_s)

    CONFIG_PROPERTY(bool, enableClaude, false)
    CONFIG_PROPERTY(QString, anthropicApiKey, u""_s)
    CONFIG_PROPERTY(QString, anthropicUrl, u"https://api.anthropic.com"_s)
    CONFIG_PROPERTY(QString, defaultClaudeModel, u""_s)

    CONFIG_PROPERTY(bool, enableOpenai, false)
    CONFIG_PROPERTY(QString, openaiApiKey, u""_s)
    CONFIG_PROPERTY(QString, openaiUrl, u"https://api.openai.com/v1"_s)
    CONFIG_PROPERTY(QString, defaultOpenaiModel, u""_s)

    CONFIG_PROPERTY(bool, enableGemini, false)
    CONFIG_PROPERTY(QString, geminiApiKey, u""_s)
    CONFIG_PROPERTY(QString, geminiUrl, u"https://generativelanguage.googleapis.com/v1beta/openai"_s)
    CONFIG_PROPERTY(QString, defaultGeminiModel, u""_s)

    CONFIG_PROPERTY(bool, enableOpenrouter, false)
    CONFIG_PROPERTY(QString, openrouterApiKey, u""_s)
    CONFIG_PROPERTY(QString, openrouterUrl, u"https://openrouter.ai/api/v1"_s)
    CONFIG_PROPERTY(QString, defaultOpenrouterModel, u""_s)

    CONFIG_PROPERTY(bool, enableOpencode, false)
    CONFIG_PROPERTY(QString, opencodeApiKey, u""_s)
    CONFIG_PROPERTY(QString, opencodeUrl, u"https://opencode.ai/zen/v1"_s)
    CONFIG_PROPERTY(QString, defaultOpencodeModel, u""_s)

    CONFIG_PROPERTY(bool, enableOpencodeGo, false)
    CONFIG_PROPERTY(QString, opencodeGoUrl, u"https://opencode.ai/zen/go/v1"_s)
    CONFIG_PROPERTY(QString, defaultOpencodeGoModel, u""_s)
};

} // namespace caelestia::config
