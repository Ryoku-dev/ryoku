package exec

import (
	"context"
	"strings"
	"testing"
)

func TestHostCommandsDoNotInheritDaemonSecrets(t *testing.T) {
	keys := []string{"OPENAI_API_KEY", "ENCRYPTION_KEY", "SESSION_TOKEN", "DB_PASSWORD", "HYAKKI_DAEMON", "RASHIN_DAEMON", "SUDO_ASKPASS", "SSH_AUTH_SOCK", "GPG_AGENT_INFO", "GOOGLE_APPLICATION_CREDENTIALS", "AWS_ACCESS_KEY_ID", "DATABASE_URL", "custom_secret"}
	for _, key := range keys {
		t.Setenv(key, "test-secret-"+key)
	}
	t.Setenv("DISPLAY", ":99")
	t.Setenv("LANG", "C.UTF-8")
	h := Host(HostOptions{})
	defer h.Close()
	ctx := context.Background()
	id, err := h.Start(ctx, StartRequest{Argv: []string{"env"}, Dir: t.TempDir()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	exit, err := h.Wait(ctx, id)
	if err != nil || exit.Code != 0 {
		t.Fatalf("env exit = %+v, %v", exit, err)
	}
	for _, key := range keys {
		if strings.Contains(exit.Output, key+"=") || strings.Contains(exit.Output, "test-secret-"+key) {
			t.Errorf("%s leaked to model command", key)
		}
	}
	for _, keep := range []string{"PATH=", "DISPLAY=:99", "LANG=C.UTF-8"} {
		if !strings.Contains(exit.Output, keep) {
			t.Errorf("ordinary environment %q missing", keep)
		}
	}
}
