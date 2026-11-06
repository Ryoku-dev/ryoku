package profile

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type pacmanResolver struct {
	o        PacmanResolverOptions
	temp     string
	config   string
	db       string
	repos    []string
	setupErr error
}

func NewPacmanResolver(o PacmanResolverOptions) Resolver {
	if o.PacmanConf == "" {
		o.PacmanConf = "/etc/pacman.conf"
	}
	if o.DBPath == "" {
		o.DBPath = "/var/lib/pacman"
	}
	if o.BlackArchServer == "" {
		o.BlackArchServer = "https://www.blackarch.org/blackarch/blackarch/os/x86_64/blackarch.db"
	}
	if o.ScratchDir == "" {
		cache := os.Getenv("XDG_CACHE_HOME")
		if cache == "" {
			if home, err := os.UserHomeDir(); err == nil {
				cache = filepath.Join(home, ".cache")
			}
		}
		o.ScratchDir = filepath.Join(cache, "ryoku", "profile-plan")
	}
	r := &pacmanResolver{o: o}
	r.setupErr = r.setup()
	return r
}

func (r *pacmanResolver) setup() error {
	if err := os.MkdirAll(r.o.ScratchDir, 0o700); err != nil {
		return err
	}
	temp, err := os.MkdirTemp(r.o.ScratchDir, "db-")
	if err != nil {
		return err
	}
	r.temp, r.db = temp, filepath.Join(temp, "db")
	if err := os.MkdirAll(filepath.Join(r.db, "sync"), 0o755); err != nil {
		return err
	}
	repos, err := parsePacmanRepos(r.o.PacmanConf)
	if err != nil {
		return err
	}
	r.repos = repos
	for _, repo := range repos {
		if err := copySyncDB(r.o.DBPath, r.db, repo); err != nil {
			return fmt.Errorf("copy %s sync database: %w", repo, err)
		}
	}
	localSource := filepath.Join(r.o.DBPath, "local")
	if err := os.Symlink(localSource, filepath.Join(r.db, "local")); err != nil {
		return err
	}
	r.config = filepath.Join(temp, "pacman.conf")
	return r.writeConfig(false)
}

func (r *pacmanResolver) writeConfig(blackarch bool) error {
	var b strings.Builder
	b.WriteString("[options]\nArchitecture = auto\nSigLevel = Never\n")
	for _, repo := range r.repos {
		if repo == "blackarch" {
			continue
		}
		fmt.Fprintf(&b, "\n[%s]\nServer = file:///var/empty\n", repo)
	}
	if blackarch || contains(r.repos, "blackarch") {
		b.WriteString("\n[blackarch]\nServer = file:///var/empty\n")
	}
	return os.WriteFile(r.config, []byte(b.String()), 0o600)
}

func (r *pacmanResolver) ensureRepos(extra []string) error {
	if r.setupErr != nil {
		return r.setupErr
	}
	wantBlackArch := contains(extra, "blackarch")
	if !wantBlackArch || contains(r.repos, "blackarch") {
		return nil
	}
	dst := filepath.Join(r.db, "sync", "blackarch.db")
	if _, err := os.Stat(dst); errors.Is(err, os.ErrNotExist) {
		resp, err := (&http.Client{Timeout: 30 * time.Second}).Get(r.o.BlackArchServer)
		if err != nil {
			return fmt.Errorf("download blackarch.db: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("download blackarch.db: HTTP %s", resp.Status)
		}
		f, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(f, resp.Body)
		closeErr := f.Close()
		if copyErr != nil {
			os.Remove(dst)
			return copyErr
		}
		if closeErr != nil {
			os.Remove(dst)
			return closeErr
		}
	} else if err != nil {
		return err
	}
	return r.writeConfig(true)
}

func (r *pacmanResolver) Resolve(targets []string, extraRepos []string) ([]PackageRef, error) {
	if err := r.ensureRepos(extraRepos); err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return []PackageRef{}, nil
	}
	installed, err := r.Installed(targets)
	if err != nil {
		return nil, err
	}
	args := []string{"--config", r.config, "--dbpath", r.db, "-Sp", "--needed", "--print-format", "%r %n %v %s"}
	args = append(args, targets...)
	cmd := exec.Command("pacman", args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	body, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("pacman resolve: %s", commandErrorText(err))
	}
	refs, err := parseResolveOutput(string(body), targets)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, ref := range refs {
		seen[ref.Name] = true
	}
	for _, name := range targets {
		if installed[name] && !seen[name] {
			refs = append(refs, PackageRef{Name: name, Target: true, PreExisting: true})
			seen[name] = true
		}
	}
	names := make([]string, 0, len(refs))
	for _, ref := range refs {
		names = append(names, ref.Name)
	}
	if len(names) == 0 {
		return refs, nil
	}
	infoArgs := []string{"--config", r.config, "--dbpath", r.db, "-Si"}
	infoArgs = append(infoArgs, names...)
	info := exec.Command("pacman", infoArgs...)
	info.Env = append(os.Environ(), "LC_ALL=C")
	infoBody, infoErr := info.Output()
	if infoErr != nil {
		return nil, fmt.Errorf("pacman info: %s", commandErrorText(infoErr))
	}
	metadata, err := parsePacmanMetadata(string(infoBody))
	if err != nil {
		return nil, err
	}
	for i := range refs {
		meta := metadata[refs[i].Name]
		refs[i].Repo, refs[i].Version, refs[i].Installed = meta.Repo, meta.Version, meta.Installed
		refs[i].PreExisting = installed[refs[i].Name]
	}
	return refs, nil
}

func (r *pacmanResolver) Installed(names []string) (map[string]bool, error) {
	out := map[string]bool{}
	if len(names) == 0 {
		return out, nil
	}
	for _, name := range names {
		cmd := exec.Command("pacman", "-Qq", name)
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		body, err := cmd.CombinedOutput()
		if err == nil {
			out[name] = true
			continue
		}
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			return nil, fmt.Errorf("pacman installed query: %w", err)
		}
		if len(strings.TrimSpace(string(body))) == 0 && exitErr.ExitCode() != 1 {
			return nil, fmt.Errorf("pacman installed query: %w", err)
		}
	}
	return out, nil
}

func (r *pacmanResolver) Required(names []string) (map[string][]string, error) {
	out := map[string][]string{}
	if len(names) == 0 {
		return out, nil
	}
	args := append([]string{"-Qi"}, names...)
	cmd := exec.Command("pacman", args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	body, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("pacman required-by: %s", strings.TrimSpace(string(body)))
	}
	return parseRequiredBy(string(body)), nil
}

func parsePacmanRepos(path string) ([]string, error) {
	seen := map[string]bool{}
	var repos []string
	var parse func(string) error
	parse = func(file string) error {
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(strings.SplitN(sc.Text(), "#", 2)[0])
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
				section := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
				if section != "options" && !seen[section] {
					seen[section] = true
					repos = append(repos, section)
				}
				continue
			}
			key, value, ok := strings.Cut(line, "=")
			if ok && strings.EqualFold(strings.TrimSpace(key), "Include") {
				matches, globErr := filepath.Glob(strings.TrimSpace(value))
				if globErr != nil {
					return globErr
				}
				for _, match := range matches {
					if err := parse(match); err != nil {
						return err
					}
				}
			}
		}
		return sc.Err()
	}
	if err := parse(path); err != nil {
		return nil, err
	}
	return repos, nil
}

func copySyncDB(hostDB, tempDB, repo string) error {
	matches, err := filepath.Glob(filepath.Join(hostDB, "sync", repo+".db*"))
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		return os.ErrNotExist
	}
	sort.Strings(matches)
	src := matches[0]
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(filepath.Join(tempDB, "sync", filepath.Base(src)), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func commandErrorText(err error) string {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if text := strings.TrimSpace(string(exitErr.Stderr)); text != "" {
			return text
		}
	}
	return err.Error()
}

func parseResolveOutput(body string, targets []string) ([]PackageRef, error) {
	target := map[string]bool{}
	for _, name := range targets {
		target[name] = true
	}
	var out []PackageRef
	sc := bufio.NewScanner(strings.NewReader(body))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 4 {
			return nil, fmt.Errorf("unexpected pacman plan line %q", line)
		}
		size, err := strconv.ParseInt(fields[3], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("pacman size for %s: %w", fields[1], err)
		}
		out = append(out, PackageRef{Name: fields[1], Repo: fields[0], Version: fields[2], Download: size, Target: target[fields[1]]})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

type pacmanMetadata struct {
	Repo, Version string
	Installed     int64
}

func parsePacmanMetadata(body string) (map[string]pacmanMetadata, error) {
	out := map[string]pacmanMetadata{}
	for _, block := range strings.Split(strings.TrimSpace(body), "\n\n") {
		name := ""
		meta := pacmanMetadata{}
		for _, line := range strings.Split(block, "\n") {
			key, value, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			switch strings.TrimSpace(key) {
			case "Repository":
				meta.Repo = strings.TrimSpace(value)
			case "Name":
				name = strings.TrimSpace(value)
			case "Version":
				meta.Version = strings.TrimSpace(value)
			case "Installed Size":
				size, err := parseHumanBytes(strings.TrimSpace(value))
				if err != nil {
					return nil, fmt.Errorf("installed size for %s: %w", name, err)
				}
				meta.Installed = size
			}
		}
		if name != "" {
			out[name] = meta
		}
	}
	return out, nil
}

func parsePacmanInfo(body string) (map[string]int64, error) {
	metadata, err := parsePacmanMetadata(body)
	if err != nil {
		return nil, err
	}
	out := map[string]int64{}
	for name, meta := range metadata {
		out[name] = meta.Installed
	}
	return out, nil
}

func parseHumanBytes(value string) (int64, error) {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return 0, fmt.Errorf("empty size")
	}
	n, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, err
	}
	factor := float64(1)
	if len(fields) > 1 {
		switch strings.ToLower(fields[1]) {
		case "kib":
			factor = 1024
		case "mib":
			factor = 1024 * 1024
		case "gib":
			factor = 1024 * 1024 * 1024
		case "b":
			factor = 1
		default:
			return 0, fmt.Errorf("unknown unit %q", fields[1])
		}
	}
	return int64(n * factor), nil
}

func parseRequiredBy(body string) map[string][]string {
	out := map[string][]string{}
	name := ""
	readingRequired := false
	for _, line := range strings.Split(body, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if ok {
			switch strings.TrimSpace(key) {
			case "Name":
				name = strings.TrimSpace(value)
				readingRequired = false
			case "Required By":
				readingRequired = true
				value = strings.TrimSpace(value)
				if name != "" {
					if value == "None" || value == "" {
						out[name] = []string{}
					} else {
						out[name] = append(out[name], strings.Fields(value)...)
					}
				}
			default:
				readingRequired = false
			}
			continue
		}
		if readingRequired && name != "" {
			out[name] = append(out[name], strings.Fields(line)...)
		}
	}
	return out
}
