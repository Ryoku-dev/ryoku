package gateway

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestImportProwlSQLiteFixture(t *testing.T) {
	sqlite, err := exec.LookPath("sqlite3")
	if err != nil {
		t.Skip("sqlite3 is not installed")
	}
	t.Setenv("ENCRYPTION_KEY", "")
	prowlDir := t.TempDir()
	master := make([]byte, 32)
	for i := range master {
		master[i] = byte(i + 1)
	}
	if err := os.WriteFile(filepath.Join(prowlDir, "master.key"), master, 0600); err != nil {
		t.Fatal(err)
	}
	block, err := aes.NewCipher(master)
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCMWithNonceSize(block, 16)
	if err != nil {
		t.Fatal(err)
	}
	seal := func(secret string, fill byte) (string, string, string) {
		nonce := make([]byte, 16)
		for i := range nonce {
			nonce[i] = fill
		}
		sealed := aead.Seal(nil, nonce, []byte(secret), nil)
		return hex.EncodeToString(sealed[:len(sealed)-16]), hex.EncodeToString(nonce), hex.EncodeToString(sealed[len(sealed)-16:])
	}
	groqCipher, groqIV, groqTag := seal("gsk_imported_secret", 7)
	anthropicCipher, anthropicIV, anthropicTag := seal("sk-ant-not-imported", 8)
	database := filepath.Join(prowlDir, "gateway.db")
	script := fmt.Sprintf(`
CREATE TABLE api_keys (id INTEGER PRIMARY KEY, platform TEXT, label TEXT, encrypted_key TEXT, iv TEXT, auth_tag TEXT);
INSERT INTO api_keys VALUES (1, 'groq', 'main', '%s', '%s', '%s');
INSERT INTO api_keys VALUES (2, 'anthropic', 'subscription', '%s', '%s', '%s');
`, groqCipher, groqIV, groqTag, anthropicCipher, anthropicIV, anthropicTag)
	if output, err := exec.Command(sqlite, database, script).CombinedOutput(); err != nil {
		t.Fatalf("create sqlite fixture: %v: %s", err, output)
	}
	gateway := newTestGateway(t)
	report, err := gateway.ImportProwl(context.Background(), prowlDir)
	if err != nil {
		t.Fatal(err)
	}
	if report.Imported != 1 || len(report.Skipped) != 1 {
		t.Fatalf("report = %#v", report)
	}
	records, err := gateway.store.listKeys("groq")
	if err != nil || len(records) != 1 {
		t.Fatalf("groq keys = %#v, %v", records, err)
	}
	secret, _, err := gateway.store.reveal(records[0].ID)
	if err != nil || secret != "gsk_imported_secret" {
		t.Fatalf("imported secret = %q, %v", secret, err)
	}
	report, err = gateway.ImportProwl(context.Background(), prowlDir)
	if err != nil || report.Imported != 0 {
		t.Fatalf("second import = %#v, %v", report, err)
	}
}
