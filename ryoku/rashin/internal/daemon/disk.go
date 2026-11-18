package daemon

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func diskToken() string { var b [16]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }

func diskWalk(ctx context.Context, root, kind string, accept func(string, fs.FileInfo) bool) (int64, []protocol.DiskFile, bool) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var size int64
	var files []protocol.DiskFile
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if kind == "cache" {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			parts := strings.Split(rel, string(filepath.Separator))
			if len(parts) < 4 || parts[2] != ".cache" {
				return nil
			}
		}
		size += info.Size()
		if accept != nil && accept(path, info) {
			file := protocol.DiskFile{Path: path, Bytes: info.Size(), Modified: info.ModTime()}
			if stat, ok := info.Sys().(*syscall.Stat_t); ok {
				file.Device, file.Inode = uint64(stat.Dev), stat.Ino
			}
			files = append(files, file)
			if len(files) > 10000 {
				return errors.New("cleanup manifest is too large")
			}
		}
		return nil
	})
	partial := err != nil && !errors.Is(err, os.ErrNotExist)
	if partial {
		files = nil
	}
	return size, files, partial
}

func (d *Daemon) diskUsage(ctx context.Context, c *connection, p protocol.DiskUsageParams) (protocol.DiskUsage, error) {
	if err := d.requireInfra("diskcare"); err != nil {
		return protocol.DiskUsage{}, err
	}
	d.diskMu.Lock()
	defer d.diskMu.Unlock()
	return d.measureDisk(ctx, p.Refresh)
}

func (d *Daemon) measureDisk(ctx context.Context, refresh bool) (protocol.DiskUsage, error) {
	if !refresh && time.Since(d.diskAt) < time.Minute {
		return d.diskCached, nil
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(paths.Home(), &stat); err != nil {
		return protocol.DiskUsage{}, err
	}
	usage := protocol.DiskUsage{Free: stat.Bavail * uint64(stat.Bsize), Total: stat.Blocks * uint64(stat.Bsize), Measured: time.Now()}
	d.diskPlans = make(map[string][]protocol.DiskFile)
	add := func(id, name, root string, accept func(string, fs.FileInfo) bool) {
		size, files, partial := diskWalk(ctx, root, id, accept)
		item := protocol.DiskItem{ID: id, Name: name, Bytes: size, Partial: partial, Files: files, Cleanable: len(files) > 0}
		if item.Cleanable {
			item.Plan = diskToken()
			d.diskPlans[item.Plan] = files
		}
		usage.Items = append(usage.Items, item)
	}
	var snapshot protocol.Snapshot
	if d.deps.Teams != nil {
		d.deps.Teams.Snapshot(ctx, &snapshot)
	}
	var teamFolders, dreamFolders []string
	for _, team := range snapshot.Teams {
		if team.Folder == "" {
			continue
		}
		if team.Dream != nil {
			dreamFolders = append(dreamFolders, team.Folder)
		} else {
			teamFolders = append(teamFolders, team.Folder)
		}
	}
	usage.Items = append(usage.Items, diskFolders(ctx, "teams", i18n.T("Team folders"), teamFolders), diskFolders(ctx, "dreams", i18n.T("Dream folders"), dreamFolders))
	add("harness", i18n.T("Harness homes"), paths.HarnessHomes(), nil)
	ended := make(map[string]bool)
	for _, info := range d.manager.List("") {
		live, ok := d.manager.Info(info.ID)
		if ok {
			info = live
		}
		d.manager.mu.Lock()
		_, loaded := d.manager.items[info.ID]
		d.manager.mu.Unlock()
		if !loaded && !info.Alive && info.State != protocol.StateWorking && info.State != protocol.StateWaiting {
			ended[info.ID] = true
		}
	}
	add("logs", i18n.T("Ended session logs"), paths.SessionsDir(), func(path string, info fs.FileInfo) bool {
		rel, err := filepath.Rel(paths.SessionsDir(), path)
		if err != nil {
			return false
		}
		parts := strings.Split(rel, string(filepath.Separator))
		return len(parts) == 2 && ended[parts[0]] && (parts[1] == "events.jsonl" || parts[1] == "stderr.log")
	})
	usage.Items = append(usage.Items, d.checkpointUsage(ctx))
	add("cache", i18n.T("Harness caches"), paths.HarnessHomes(), func(path string, info fs.FileInfo) bool {
		rel, err := filepath.Rel(paths.HarnessHomes(), path)
		if err != nil {
			return false
		}
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) < 4 || parts[2] != ".cache" {
			return false
		}
		for _, session := range d.manager.List(protocol.KindAgent) {
			if live, ok := d.manager.Info(session.ID); ok {
				session = live
			}
			if session.Alive || session.State == protocol.StateWorking || session.State == protocol.StateWaiting {
				return false
			}
		}
		return true
	})
	if owner, ok := d.deps.Teams.(interface{ DiskTasks() []protocol.TeamTask }); ok {
		eligible := make(map[string]string)
		days := d.Settings().Teams.KeepDays
		cutoff := time.Now().AddDate(0, 0, -days)
		for _, task := range owner.DiskTasks() {
			if days > 0 && (task.Status == protocol.TaskDone || task.Status == protocol.TaskStopped) && task.Ended != nil && task.Ended.Before(cutoff) {
				eligible[task.ID+".json"] = task.ID
			}
		}
		add("tasks", i18n.T("Finished task records"), filepath.Join(paths.TeamsDir(), "tasks"), func(path string, info fs.FileInfo) bool { return eligible[filepath.Base(path)] != "" })
		item := &usage.Items[len(usage.Items)-1]
		for index := range item.Files {
			item.Files[index].Task = eligible[filepath.Base(item.Files[index].Path)]
		}
		if item.Plan != "" {
			d.diskPlans[item.Plan] = item.Files
		}
	}
	usage.Items[2].Bytes = max(0, usage.Items[2].Bytes-usage.Items[5].Bytes)
	d.diskCached, d.diskAt = usage, time.Now()
	return usage, nil
}

func (d *Daemon) diskClean(ctx context.Context, c *connection, p protocol.DiskCleanParams) (protocol.DiskUsage, error) {
	if err := d.requireInfra("diskcare"); err != nil {
		return protocol.DiskUsage{}, err
	}
	d.diskMu.Lock()
	defer d.diskMu.Unlock()
	files, ok := d.diskPlans[p.Plan]
	if !ok || p.Plan == "" {
		return protocol.DiskUsage{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The cleanup list changed. Review it again."))
	}
	delete(d.diskPlans, p.Plan)
	sessionFiles := false
	for _, file := range files {
		if file.Task == "" {
			sessionFiles = true
			break
		}
	}
	if sessionFiles {
		d.manager.mu.Lock()
		defer d.manager.mu.Unlock()
		for _, managed := range d.manager.items {
			info := managed.log.Info()
			if info.Alive || info.State == protocol.StateWorking || info.State == protocol.StateWaiting {
				return protocol.DiskUsage{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Stop active sessions before clearing their files."))
			}
			for _, file := range files {
				if strings.HasPrefix(file.Path, paths.SessionDir(info.ID)+string(filepath.Separator)) {
					return protocol.DiskUsage{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("A listed session was opened. Measure disk use again to review what can be cleared."))
				}
			}
		}
	}
	roots := make([]*os.Root, 0, len(files))
	parents := make(map[string]*os.Root)
	defer func() {
		for _, root := range parents {
			root.Close()
		}
	}()
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return protocol.DiskUsage{}, err
		}
		parent := filepath.Dir(file.Path)
		root := parents[parent]
		var err error
		if root == nil {
			root, err = diskParent(file.Path)
			if err != nil {
				return protocol.DiskUsage{}, err
			}
			parents[parent] = root
		}
		roots = append(roots, root)
		info, err := root.Lstat(filepath.Base(file.Path))
		if err == nil {
			stat, valid := info.Sys().(*syscall.Stat_t)
			if !valid || !info.Mode().IsRegular() || info.Size() != file.Bytes || !info.ModTime().Equal(file.Modified) || uint64(stat.Dev) != file.Device || stat.Ino != file.Inode {
				err = errors.New(i18n.T("A listed file changed. Review the cleanup again."))
			}
		}
		if err != nil {
			return protocol.DiskUsage{}, err
		}
	}
	for index, file := range files {
		var err error
		if file.Task != "" {
			owner, ok := d.deps.Teams.(interface{ DiskRemoveTask(string, *os.Root) error })
			if !ok {
				return protocol.DiskUsage{}, errors.New(i18n.T("Task cleanup is unavailable."))
			}
			err = owner.DiskRemoveTask(file.Task, roots[index])
		} else {
			err = roots[index].Remove(filepath.Base(file.Path))
		}
		if err != nil {
			d.diskAt = time.Time{}
			return protocol.DiskUsage{}, err
		}
	}
	d.diskAt = time.Time{}
	result := d.diskCached
	for index := range result.Items {
		if result.Items[index].Plan == p.Plan {
			var removed int64
			for _, file := range files {
				removed += file.Bytes
			}
			result.Items[index].Bytes = max(0, result.Items[index].Bytes-removed)
			result.Items[index].Cleanable, result.Items[index].Plan, result.Items[index].Files = false, "", nil
		}
	}
	var stat syscall.Statfs_t
	if syscall.Statfs(paths.Home(), &stat) == nil {
		result.Free, result.Total = stat.Bavail*uint64(stat.Bsize), stat.Blocks*uint64(stat.Bsize)
	}
	result.Measured = time.Now()
	d.diskCached = result
	return result, nil
}

func (d *Daemon) checkpointUsage(ctx context.Context) protocol.DiskItem {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	item := protocol.DiskItem{ID: "checkpoints", Name: i18n.T("Checkpoints (shared project objects)")}
	seen := make(map[string]bool)
	for _, info := range d.manager.List(protocol.KindAgent) {
		if !info.Checkpoints || info.Cwd == "" {
			continue
		}
		command := func(args ...string) ([]byte, error) {
			return diskCommand(exec.CommandContext(ctx, "git", append([]string{"-C", info.Cwd}, args...)...))
		}
		dir, err := command("rev-parse", "--git-common-dir")
		if err != nil {
			item.Partial = true
			continue
		}
		key := strings.TrimSpace(string(dir))
		if !filepath.IsAbs(key) {
			key = filepath.Join(info.Cwd, key)
		}
		key = filepath.Clean(key)
		if seen[key] {
			continue
		}
		seen[key] = true
		refs, err := command("for-each-ref", "--format=%(objectname)", "refs/rashin/")
		if err != nil {
			item.Partial = true
			continue
		}
		if len(refs) == 0 {
			continue
		}
		walk := exec.CommandContext(ctx, "git", "-C", info.Cwd, "rev-list", "--objects", "--no-object-names", "--stdin")
		walk.Stdin = strings.NewReader(string(refs))
		objects, err := diskCommand(walk)
		if err != nil {
			item.Partial = true
			continue
		}
		check := exec.CommandContext(ctx, "git", "-C", info.Cwd, "cat-file", "--batch-check=%(objectname) %(objectsize:disk)")
		check.Stdin = strings.NewReader(string(objects))
		sizes, err := diskCommand(check)
		if err != nil {
			item.Partial = true
			continue
		}
		for line := range strings.SplitSeq(string(sizes), "\n") {
			fields := strings.Fields(line)
			if len(fields) != 2 {
				continue
			}
			bytes, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				item.Partial = true
				continue
			}
			item.Bytes += bytes
		}
	}
	if ctx.Err() != nil {
		item.Partial = true
	}
	return item
}

type diskOutput struct{ bytes.Buffer }

func (w *diskOutput) Write(p []byte) (int, error) {
	if w.Len()+len(p) > 8<<20 {
		return 0, errors.New("checkpoint listing is too large")
	}
	return w.Buffer.Write(p)
}

func diskCommand(cmd *exec.Cmd) ([]byte, error) {
	var output diskOutput
	cmd.Stdout = &output
	err := cmd.Run()
	return output.Bytes(), err
}

func diskParent(path string) (*os.Root, error) {
	for _, base := range []string{paths.SessionsDir(), paths.HarnessHomes(), filepath.Join(paths.TeamsDir(), "tasks")} {
		rel, err := filepath.Rel(base, path)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		root, err := os.OpenRoot(base)
		if err != nil {
			return nil, err
		}
		parent, err := root.OpenRoot(filepath.Dir(rel))
		root.Close()
		return parent, err
	}
	return nil, errors.New(i18n.T("This path is outside agent data."))
}

func diskFolders(ctx context.Context, id, name string, folders []string) protocol.DiskItem {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	item := protocol.DiskItem{ID: id, Name: name}
	roots := make([]string, 0, len(folders))
	for _, folder := range folders {
		root, err := filepath.EvalSymlinks(folder)
		if err != nil {
			item.Partial = true
			continue
		}
		roots = append(roots, root)
	}
	sort.Strings(roots)
	previous := ""
	for _, root := range roots {
		if previous != "" && (root == previous || strings.HasPrefix(root, previous+string(filepath.Separator))) {
			continue
		}
		size, _, partial := diskWalk(ctx, root, id, nil)
		item.Bytes += size
		item.Partial = item.Partial || partial
		previous = root
	}
	return item
}
