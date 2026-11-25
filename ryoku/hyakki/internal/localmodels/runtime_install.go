package localmodels

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// InstallRuntime begins an explicit, pinned runtime download. No runtime is
// installed when New, Models or Ensure encounters an old system binary.
func (m *Manager) InstallRuntime(ctx context.Context, backend string) error {
	if backend == "" {
		backend = protocol.BackendVulkan
	}
	asset, supported := runtimeAssets[backend]
	if !supported || len(offers()) == 0 {
		return protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("Choose Vulkan or processor for the llama.cpp runtime on this system."))
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return errors.New(i18n.T("Hyakki is shutting down."))
	}
	if m.jobs[protocol.LocalRuntimeID] != nil {
		m.mu.Unlock()
		return nil
	}
	jobCtx, cancel := context.WithCancel(context.Background())
	j := &job{cancel: cancel, done: make(chan struct{})}
	m.jobs[protocol.LocalRuntimeID] = j
	m.mu.Unlock()
	go m.installJob(jobCtx, backend, asset, j)
	m.notify()
	return nil
}

func (m *Manager) installJob(ctx context.Context, backend string, asset runtimeAsset, j *job) {
	err := m.installRuntime(ctx, backend, asset)
	m.mu.Lock()
	delete(m.jobs, protocol.LocalRuntimeID)
	close(j.done)
	m.mu.Unlock()
	phase := protocol.LocalPhaseDone
	if err != nil {
		phase = protocol.LocalPhaseError
		if errors.Is(err, context.Canceled) {
			phase = protocol.LocalPhaseCancelled
		}
	}
	p := protocol.LocalProgress{ID: protocol.LocalRuntimeID, Phase: phase}
	if err != nil && phase == protocol.LocalPhaseError {
		p.Error = err.Error()
	}
	m.progress(p)
	m.notify()
}

func (m *Manager) installRuntime(ctx context.Context, backend string, asset runtimeAsset) error {
	root := paths.LlamaRuntimeDir()
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	if err := checkSpace(root, asset.Size+(200<<20)); err != nil {
		return err
	}
	archive := filepath.Join(root, asset.Name)
	last, percent := time.Time{}, int64(-1)
	progress := func(phase string, done int64) {
		p := done * 100 / asset.Size
		if done == asset.Size || time.Since(last) >= 250*time.Millisecond || p > percent {
			last, percent = time.Now(), p
			m.progress(protocol.LocalProgress{ID: protocol.LocalRuntimeID, File: asset.Name, Done: done, Total: asset.Size, Phase: phase})
		}
	}
	_, err := fetch(ctx, m.client, fetchRequest{URL: runtimeReleaseBase + "/b12521/" + asset.Name, Dest: archive, Size: asset.Size, SHA256: asset.SHA256}, progress)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	staging := filepath.Join(root, "."+buildDir(runtimeBuild)+".part")
	if err := os.RemoveAll(staging); err != nil {
		return err
	}
	if err := extractTarGz(archive, staging, buildDir(runtimeBuild)); err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	bin := filepath.Join(staging, "llama-server")
	out, err := runQuick(ctx, bin, 5*time.Second, "--version")
	build, _, _, ok := parseVersion(out)
	if err != nil || !ok || build != runtimeBuild {
		return errors.New(i18n.T("The downloaded llama.cpp runtime did not report the pinned build."))
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	finishSwap, err := m.beginRuntimeSwap(ctx)
	if err != nil {
		return err
	}
	defer finishSwap()
	m.Stop()
	installed := filepath.Join(root, buildDir(runtimeBuild))
	backup := installed + ".previous"
	if _, err := os.Lstat(backup); err == nil {
		return errors.New(i18n.T("An earlier llama.cpp installation backup is still present."))
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	old := false
	if _, err := os.Lstat(installed); err == nil {
		if err := os.Rename(installed, backup); err != nil {
			return err
		}
		old = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(staging, installed); err != nil {
		if old {
			_ = os.Rename(backup, installed)
		}
		return err
	}
	rec := runtimeRecord{Build: runtimeBuild, Backend: backend, Dir: buildDir(runtimeBuild), Asset: asset.Name, SHA256: asset.SHA256, Installed: time.Now().UTC()}
	data, _ := json.MarshalIndent(rec, "", "  ")
	if err := writeAtomic(runtimeRecordPath(), append(data, '\n'), 0o600); err != nil {
		_ = os.RemoveAll(installed)
		if old {
			_ = os.Rename(backup, installed)
		}
		return err
	}
	_ = os.Remove(archive)
	if old {
		_ = os.RemoveAll(backup)
	}
	for _, b := range installedBuilds() {
		if b != runtimeBuild {
			_ = os.RemoveAll(filepath.Join(root, buildDir(b)))
		}
	}
	return nil
}
