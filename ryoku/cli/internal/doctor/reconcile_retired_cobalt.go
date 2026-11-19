package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// ---- reconciler: sweep the retired cobalt download engine -------------------
//
// The old Stash download queue could run through a local cobalt container that
// a polkit helper provisioned in docker. ryoget replaced it, so the per-user
// engine switch (stash.json) and the format-probe cache are dead, and boxes that
// turned the engine on still carry the container and its image.
//
// The sweep is gated on those per-user files: they are the evidence the engine
// ever existed here, so a clean box never runs docker and the "not reachable"
// warning is said once rather than on every doctor run. Docker is only driven
// through the user's own CLI when `docker info` already works: doctor never
// escalates, never touches docker.service, and only ever names the one
// container and image Ryoku created.

const (
	retiredCobaltContainer = "ryoku-cobalt"
	retiredCobaltImage     = "ghcr.io/imputnet/cobalt:11"
)

var cobaltDockerTimeout = 20 * time.Second

func retiredCobaltFiles() []string {
	return []string{
		filepath.Join(sys.ConfigHome(), "ryoku", "stash.json"),
		filepath.Join(sys.Xdg("XDG_CACHE_HOME", ".cache"), "ryoku", "stash-probe"),
	}
}

type cobaltDockerState int

const (
	cobaltDockerAbsent cobaltDockerState = iota
	cobaltDockerUnreachable
	cobaltDockerNoContainer
	cobaltDockerContainer
)

func cobaltDocker(args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cobaltDockerTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if err != nil {
		if msg := firstLine(strings.TrimSpace(string(out))); msg != "" {
			return fmt.Errorf("%s", msg)
		}
		return err
	}
	return nil
}

func probeCobaltDocker() cobaltDockerState {
	if _, err := exec.LookPath("docker"); err != nil {
		return cobaltDockerAbsent
	}
	if cobaltDocker("info") != nil {
		return cobaltDockerUnreachable
	}
	if cobaltDocker("container", "inspect", "--format", "{{.Name}}", retiredCobaltContainer) != nil {
		return cobaltDockerNoContainer
	}
	return cobaltDockerContainer
}

func retiredCobaltManual() string {
	return fmt.Sprintf("sudo docker rm -f %s; sudo docker image rm %s", retiredCobaltContainer, retiredCobaltImage)
}

func reconcileRetiredCobalt(checkOnly bool) recResult {
	var present []string
	for _, p := range retiredCobaltFiles() {
		if sys.Exists(p) {
			present = append(present, p)
		}
	}
	if len(present) == 0 {
		return okRes(i18n.T("no retired download engine leftovers"))
	}
	docker := probeCobaltDocker()

	if checkOnly {
		switch docker {
		case cobaltDockerContainer:
			return wouldRes(i18n.T("retired download engine leftovers remain: %s, and the %s container"), tildeList(present), retiredCobaltContainer).
				withFix(i18n.T("ryoku doctor removes them, the container and its image %s"), retiredCobaltImage)
		case cobaltDockerUnreachable:
			return wouldRes(i18n.T("retired download engine leftovers remain: %s; docker is not reachable without privilege, so a %s container cannot be checked"), tildeList(present), retiredCobaltContainer).
				withFix(i18n.T("ryoku doctor removes the files; remove the container by hand if it exists: %s"), retiredCobaltManual())
		default:
			return wouldRes(i18n.T("retired download engine leftovers remain: %s"), tildeList(present)).
				withFix(i18n.T("ryoku doctor removes them"))
		}
	}

	// The container goes first: if it cannot be removed the files stay, so the
	// next run still has its evidence and tries again.
	var imageErr error
	if docker == cobaltDockerContainer {
		if err := cobaltDocker("container", "rm", "--force", retiredCobaltContainer); err != nil {
			return failRes(i18n.T("could not remove the %s container: %v"), retiredCobaltContainer, err).
				withFix(retiredCobaltManual())
		}
		if cobaltDocker("image", "inspect", "--format", "{{.Id}}", retiredCobaltImage) == nil {
			imageErr = cobaltDocker("image", "rm", retiredCobaltImage)
		}
	}

	var removed []string
	for _, p := range present {
		if err := os.RemoveAll(p); err != nil {
			return failRes(i18n.T("could not remove %s: %v"), tildeOf(p), err)
		}
		removed = append(removed, tildeOf(p))
	}
	files := strings.Join(removed, ", ")

	switch {
	case docker == cobaltDockerUnreachable:
		return warnRes(i18n.T("removed %s; docker is not reachable without privilege, so a leftover %s container was not checked"), files, retiredCobaltContainer).
			withFix(i18n.T("if the container exists, remove it by hand: %s"), retiredCobaltManual())
	case imageErr != nil:
		return warnRes(i18n.T("removed %s and the %s container, but kept the image %s: %v"), files, retiredCobaltContainer, retiredCobaltImage, imageErr).
			withFix(fmt.Sprintf("docker image rm %s", retiredCobaltImage))
	case docker == cobaltDockerContainer:
		return fixedRes(i18n.T("removed %s and the %s container with its image"), files, retiredCobaltContainer)
	default:
		return fixedRes(i18n.T("removed the retired download engine leftovers: %s"), files)
	}
}
