package updater

import (
	"fmt"
	"os"
	"strings"

	"ryoku-cli/internal/host"
	"ryoku-cli/internal/sys"
)

func repositoryHost() *host.App {
	return host.New(host.Config{
		PacmanConf:    sys.PacmanConf,
		PacmanSyncDir: sys.PacmanSyncDir,
	})
}

func repositoryPathsOverridden() bool {
	return sys.PacmanConf != "/etc/pacman.conf" ||
		sys.PacmanSyncDir != "/var/lib/pacman/sync" ||
		os.Getenv("RYOKU_DNF_REPO_CONFIG") != ""
}

func updatePackageManager() host.PackageManager {
	manager, err := host.Default().PackageManager()
	if err != nil {
		return host.Pacman
	}
	return manager
}

func updateInitSystem() host.InitSystem {
	init, err := host.Default().Init()
	if err != nil {
		return host.Systemd
	}
	return init
}

func packagedChannel() string {
	channel, _ := repositoryHost().RepoChannel()
	return channel
}

func packagedRepoURL() string {
	url, _ := repositoryHost().RepoURL()
	return url
}

func channelRepoURL(channel string) string {
	return host.RepoURLFor(updatePackageManager(), channel)
}

func repoReleaseURL(channel string) string {
	return expandHTTPRepoURL(channelRepoURL(channel))
}

func expandHTTPRepoURL(url string) string {
	url = host.ExpandRepoURL(url)
	if strings.Contains(url, "$") {
		return ""
	}
	return url
}

func repoSetChannel(channel string) error {
	app := repositoryHost()
	if os.Geteuid() == 0 || repositoryPathsOverridden() || os.Getenv("RYOKU_PACMAN_CONF") != "" || os.Getenv("RYOKU_XBPS_CONFIG_DIR") != "" {
		return app.RepoSetChannel(channel)
	}
	return sys.Sudo("ryoku-host", "repo", "set-channel", channel)
}

func repoSetURL(url string) error {
	app := repositoryHost()
	if os.Geteuid() == 0 || repositoryPathsOverridden() || os.Getenv("RYOKU_PACMAN_CONF") != "" || os.Getenv("RYOKU_XBPS_CONFIG_DIR") != "" {
		return app.RepoSetURL(url)
	}
	return sys.Sudo("ryoku-host", "repo", "set-url", url)
}

func repoSync(force bool) error {
	app := repositoryHost()
	if os.Geteuid() == 0 || repositoryPathsOverridden() || os.Getenv("RYOKU_PACMAN_CONF") != "" || os.Getenv("RYOKU_XBPS_CONFIG_DIR") != "" {
		return app.RepoSync(force)
	}
	args := []string{"ryoku-host", "repo", "sync"}
	if force {
		args = append(args, "--force")
	}
	return sys.Sudo(args...)
}

func repoInstalledSet(allowDowngrade bool) ([]string, int, error) {
	excluded := map[string]bool(nil)
	if updatePackageManager() == host.Pacman {
		excluded = externalReleasePkgs
	}
	return repositoryHost().RepoInstalledSet(allowDowngrade, excluded)
}

func repoAvailableVersion(name string) string {
	version, _ := repositoryHost().RepoAvailableVersion(name)
	return version
}

func service(args ...string) int {
	return host.Default().Service(args)
}

func serviceError(action string, code int) error {
	if code == host.ExitOK {
		return nil
	}
	return fmt.Errorf("%s: host service exit %d", action, code)
}
