package gateway

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/core"
)

var _ core.Gateway = (*Gateway)(nil)

type prowlKeyRow struct {
	ID           int64  `json:"id"`
	Platform     string `json:"platform"`
	Label        string `json:"label"`
	EncryptedKey string `json:"encrypted_key"`
	IV           string `json:"iv"`
	AuthTag      string `json:"auth_tag"`
}

var prowlPlatformMap = map[string]string{
	"nvidia-nim": "nvidia", "nvidia": "nvidia",
	"modelscope":            "modelscope",
	"cloudflare-workers-ai": "cloudflare", "cloudflare": "cloudflare",
	"google-gemini": "google", "google": "google",
	"llm7-io": "llm7", "llm7": "llm7",
	"ollama-cloud": "ollama", "ollama": "ollama",
	"github-models": "github", "github": "github",
	"kilo-code": "kilo", "kilo": "kilo",
	"mistral-ai": "mistral", "mistral": "mistral",
	"ovhcloud-ai-endpoints": "ovh", "ovh": "ovh",
	"opencode-zen": "opencode", "opencode": "opencode",
	"groq": "groq", "cerebras": "cerebras", "openrouter": "openrouter", "cohere": "cohere",
	"hugging-face": "huggingface", "huggingface": "huggingface",
	"z-ai-zhipu-ai": "zhipu", "zhipu": "zhipu",
	"sambanova": "sambanova", "siliconflow": "siliconflow",
	"chutes-ai": "chutes", "chutes": "chutes",
	"pollinations": "pollinations", "aion-labs": "aion", "aion": "aion",
	"agnes-ai": "agnes", "agnes": "agnes",
	"alibaba-cloud-model-studio": "alibaba", "alibaba": "alibaba",
	"xai": "xai", "deepseek": "deepseek", "nscale": "nscale", "nebius": "nebius",
}

// ImportProwl copies supported API keys from Prowl without modifying its files.
func (g *Gateway) ImportProwl(ctx context.Context, prowlDir string) (core.ImportReport, error) {
	var report core.ImportReport
	databasePath := filepath.Join(prowlDir, "gateway.db")
	masterPath := filepath.Join(prowlDir, "master.key")
	if _, err := os.Stat(databasePath); errors.Is(err, os.ErrNotExist) {
		report.Notes = append(report.Notes, i18n.T("Prowl gateway.db was not found"))
		return report, nil
	} else if err != nil {
		return report, fmt.Errorf("inspect Prowl gateway database: %w", err)
	}
	sqlite, err := exec.LookPath("sqlite3")
	if err != nil {
		report.Notes = append(report.Notes, i18n.T("sqlite3 is not installed, so Prowl keys were not imported"))
		return report, nil
	}
	master, note, err := prowlMasterKey(masterPath)
	if errors.Is(err, os.ErrNotExist) {
		report.Notes = append(report.Notes, i18n.T("Prowl master.key and ENCRYPTION_KEY were not found"))
		return report, nil
	}
	if err != nil {
		return report, err
	}
	if note != "" {
		report.Notes = append(report.Notes, note)
	}
	query := "SELECT id, platform, label, encrypted_key, iv, auth_tag FROM api_keys ORDER BY id"
	command := exec.CommandContext(ctx, sqlite, "-readonly", "-json", databasePath, query)
	output, err := command.Output()
	if err != nil {
		return report, fmt.Errorf("read Prowl keys with sqlite3: %w", err)
	}
	var rows []prowlKeyRow
	if len(strings.TrimSpace(string(output))) != 0 {
		if err := json.Unmarshal(output, &rows); err != nil {
			return report, fmt.Errorf("decode Prowl key rows: %w", err)
		}
	}
	block, err := aes.NewCipher(master)
	if err != nil {
		return report, fmt.Errorf("initialize Prowl key decryption: %w", err)
	}
	aead, err := cipher.NewGCMWithNonceSize(block, 16)
	if err != nil {
		return report, fmt.Errorf("initialize Prowl key sealing: %w", err)
	}
	for _, row := range rows {
		providerID, ok := prowlPlatformMap[strings.ToLower(strings.TrimSpace(row.Platform))]
		if !ok {
			report.Skipped = append(report.Skipped, i18n.Tf("Key %d uses unsupported provider %s", row.ID, row.Platform))
			continue
		}
		p, exists := g.catalog.findProvider(providerID)
		if !exists || p.Keyless {
			report.Skipped = append(report.Skipped, i18n.Tf("Key %d does not need import", row.ID))
			continue
		}
		secret, err := openProwlSecret(aead, row)
		if err != nil {
			report.Skipped = append(report.Skipped, i18n.Tf("Key %d could not be decrypted", row.ID))
			continue
		}
		_, duplicate, err := g.store.addKey(providerID, row.Label, secret)
		if err != nil {
			return report, fmt.Errorf("import Prowl key %d: %w", row.ID, err)
		}
		if duplicate {
			report.Skipped = append(report.Skipped, i18n.Tf("Key %d is already present", row.ID))
			continue
		}
		report.Imported++
	}
	return report, nil
}

func prowlMasterKey(path string) ([]byte, string, error) {
	if raw := strings.TrimSpace(os.Getenv("ENCRYPTION_KEY")); raw != "" {
		key, err := hex.DecodeString(raw)
		if err != nil || len(key) != 32 {
			return nil, "", errors.New("ENCRYPTION_KEY must contain 64 hexadecimal characters")
		}
		return key, i18n.T("Prowl ENCRYPTION_KEY was used for migration"), nil
	}
	key, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	if len(key) != 32 {
		return nil, "", fmt.Errorf("Prowl master.key is %d bytes, expected 32", len(key))
	}
	return key, "", nil
}

func openProwlSecret(aead cipher.AEAD, row prowlKeyRow) (string, error) {
	iv, err := hex.DecodeString(row.IV)
	if err != nil || len(iv) != 16 {
		return "", errors.New("Prowl key IV is malformed")
	}
	ciphertext, err := hex.DecodeString(row.EncryptedKey)
	if err != nil {
		return "", errors.New("Prowl key ciphertext is malformed")
	}
	tag, err := hex.DecodeString(row.AuthTag)
	if err != nil || len(tag) != 16 {
		return "", errors.New("Prowl key authentication tag is malformed")
	}
	sealed := append(append(make([]byte, 0, len(ciphertext)+len(tag)), ciphertext...), tag...)
	plain, err := aead.Open(nil, iv, sealed, nil)
	if err != nil {
		return "", errors.New("Prowl key authentication failed")
	}
	return string(plain), nil
}
