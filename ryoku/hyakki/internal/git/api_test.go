package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func gitTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "LC_ALL=C", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeTestFile(t *testing.T, root, name, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
}

func newTestRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gitTest(t, root, "init", "-b", "main")
	gitTest(t, root, "config", "user.name", "Rashin Test")
	gitTest(t, root, "config", "user.email", "rashin@example.invalid")
	writeTestFile(t, root, "seed.txt", "seed\n")
	gitTest(t, root, "add", "seed.txt")
	gitTest(t, root, "commit", "-m", "seed")
	return root
}

func TestWorktreesLockedPrunableAndExistingBranchOwner(t *testing.T) {
	ctx := context.Background()
	root := newTestRepo(t)
	repo, err := Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(t.TempDir(), "locked")
	if err := repo.AddWorktree(ctx, AddOptions{Path: locked, Mode: protocol.WorktreeNewBranch, Branch: "locked", Base: "main", Lock: true, LockReason: "test lock"}); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "missing")
	if err := repo.AddWorktree(ctx, AddOptions{Path: missing, Mode: protocol.WorktreeNewBranch, Branch: "missing", Base: "main"}); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(missing); err != nil {
		t.Fatal(err)
	}
	worktrees, err := repo.Worktrees(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(worktrees) != 3 || !worktrees[0].Main {
		t.Fatalf("unexpected worktrees: %+v", worktrees)
	}
	var sawLocked, sawPrunable bool
	for _, wt := range worktrees {
		sawLocked = sawLocked || wt.Path == locked && wt.Locked && wt.LockReason == "test lock"
		sawPrunable = sawPrunable || wt.Path == missing && wt.Prunable && wt.Missing
	}
	if !sawLocked || !sawPrunable {
		t.Fatalf("locked=%v prunable=%v worktrees=%+v", sawLocked, sawPrunable, worktrees)
	}
	second := filepath.Join(t.TempDir(), "second")
	err = repo.AddWorktree(ctx, AddOptions{Path: second, Mode: protocol.WorktreeExistingBranch, Branch: "locked"})
	if err == nil || !strings.Contains(err.Error(), locked) {
		t.Fatalf("expected owning worktree in error, got %v", err)
	}
}

func TestRemovePreviewFlagsLossAndMain(t *testing.T) {
	ctx := context.Background()
	root := newTestRepo(t)
	repo, err := Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "feature")
	if err := repo.AddWorktree(ctx, AddOptions{Path: path, Mode: protocol.WorktreeNewBranch, Branch: "feature", Base: "main"}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, path, "seed.txt", "dirty\n")
	writeTestFile(t, path, "loose.txt", "untracked\n")
	writeTestFile(t, path, "commit.txt", "commit\n")
	gitTest(t, path, "add", "commit.txt")
	gitTest(t, path, "commit", "-m", "unpushed work")
	if err := repo.LockWorktree(ctx, path, true, "keep"); err != nil {
		t.Fatal(err)
	}
	worktrees, err := repo.Worktrees(ctx)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := repo.RemovePreview(ctx, worktrees[1], "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Dirty) == 0 || len(preview.Untracked) == 0 || len(preview.Unpushed) == 0 || !preview.Unmerged || !preview.Locked || !preview.NeedsForce || !preview.CanRemove {
		t.Fatalf("incomplete preview: %+v", preview)
	}
	mainPreview, err := repo.RemovePreview(ctx, worktrees[0], "main")
	if err != nil {
		t.Fatal(err)
	}
	if !mainPreview.Main || mainPreview.CanRemove || mainPreview.BlockReason == "" {
		t.Fatalf("unsafe main preview: %+v", mainPreview)
	}
}

func TestStatusRenameConflictUntrackedAndBaseCounts(t *testing.T) {
	ctx := context.Background()
	root := newTestRepo(t)
	writeTestFile(t, root, "rename.txt", "one\ntwo\nthree\nfour\n")
	writeTestFile(t, root, "conflict.txt", "base\n")
	gitTest(t, root, "add", ".")
	gitTest(t, root, "commit", "-m", "base files")
	gitTest(t, root, "branch", "base")
	gitTest(t, root, "checkout", "-b", "feature")
	writeTestFile(t, root, "feature.txt", "feature\n")
	gitTest(t, root, "add", "feature.txt")
	gitTest(t, root, "commit", "-m", "feature commit")
	gitTest(t, root, "checkout", "base")
	writeTestFile(t, root, "conflict.txt", "base side\n")
	gitTest(t, root, "add", "conflict.txt")
	gitTest(t, root, "commit", "-m", "base side")
	gitTest(t, root, "checkout", "feature")
	writeTestFile(t, root, "conflict.txt", "feature side\n")
	gitTest(t, root, "add", "conflict.txt")
	gitTest(t, root, "commit", "-m", "feature side")
	cmd := exec.Command("git", "-C", root, "merge", "base")
	cmd.Env = append(os.Environ(), "LC_ALL=C", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_EDITOR=true")
	if err := cmd.Run(); err == nil {
		t.Fatal("merge unexpectedly succeeded")
	}
	gitTest(t, root, "mv", "rename.txt", "renamed.txt")
	writeTestFile(t, root, "untracked.txt", "a\nb\n")
	got, err := Status(ctx, root, "base")
	if err != nil {
		t.Fatal(err)
	}
	var rename, conflict, untracked bool
	for _, file := range got.Files {
		rename = rename || file.Status == protocol.ChangeRenamed && file.Path == "renamed.txt" && file.OldPath == "rename.txt"
		conflict = conflict || file.Status == protocol.ChangeConflict && file.Path == "conflict.txt"
		untracked = untracked || file.Status == protocol.ChangeUntracked && file.Path == "untracked.txt" && file.Added == 2
	}
	if !rename || !conflict || !untracked || got.Status.AheadBase == 0 || got.Status.BehindBase == 0 || got.Status.Operation != protocol.OpMerge {
		t.Fatalf("status missing data: %+v", got)
	}
}

func TestDiffHunksAndRename(t *testing.T) {
	ctx := context.Background()
	root := newTestRepo(t)
	writeTestFile(t, root, "modify.txt", "one\ntwo\nthree\n")
	writeTestFile(t, root, "delete.txt", "gone\n")
	writeTestFile(t, root, "rename.txt", "alpha\nbeta\ngamma\ndelta\n")
	gitTest(t, root, "add", ".")
	gitTest(t, root, "commit", "-m", "diff base")
	writeTestFile(t, root, "modify.txt", "one\nchanged\nthree\nadded\n")
	if err := os.Remove(filepath.Join(root, "delete.txt")); err != nil {
		t.Fatal(err)
	}
	gitTest(t, root, "mv", "rename.txt", "renamed.txt")
	writeTestFile(t, root, "added.txt", "new\n")
	gitTest(t, root, "add", "-A")
	got, err := Diff(ctx, root, DiffOptions{Mode: protocol.DiffStaged, Context: 3})
	if err != nil {
		t.Fatal(err)
	}
	var sawAdd, sawDelete, sawModify, sawRename bool
	for _, file := range got.Files {
		switch file.Status {
		case protocol.ChangeAdded:
			sawAdd = file.Added == 1 && len(file.Hunks) == 1 && file.Hunks[0].Lines[0].New == 1
		case protocol.ChangeDeleted:
			sawDelete = file.Removed == 1 && len(file.Hunks) == 1 && file.Hunks[0].Lines[0].Old == 1
		case protocol.ChangeModified:
			if file.Path == "modify.txt" {
				var addAt2, deleteAt2 bool
				for _, line := range file.Hunks[0].Lines {
					addAt2 = addAt2 || line.Kind == protocol.LineAdd && line.New == 2
					deleteAt2 = deleteAt2 || line.Kind == protocol.LineDel && line.Old == 2
				}
				sawModify = addAt2 && deleteAt2
			}
		case protocol.ChangeRenamed:
			sawRename = file.OldPath == "rename.txt" && file.Path == "renamed.txt" && file.Added == 0 && file.Removed == 0 && len(file.Hunks) == 0
		}
	}
	if !sawAdd || !sawDelete || !sawModify || !sawRename {
		t.Fatalf("diff incomplete: %+v", got)
	}
}

func TestLogPagingStableAndHiddenRefsExcluded(t *testing.T) {
	ctx := context.Background()
	root := newTestRepo(t)
	for index := 0; index < 6; index++ {
		writeTestFile(t, root, "seed.txt", gitTestNumber(index)+"\n")
		gitTest(t, root, "add", "seed.txt")
		gitTest(t, root, "commit", "-m", "commit "+gitTestNumber(index))
	}
	repo, err := Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	hidden, err := Checkpoint(ctx, root, "refs/rashin/test/1")
	if err != nil {
		t.Fatal(err)
	}
	big, _, err := repo.Log(ctx, LogQuery{Scope: protocol.ScopeAll, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	page1, next, err := repo.Log(ctx, LogQuery{Scope: protocol.ScopeAll, Limit: 3})
	if err != nil || next == "" {
		t.Fatalf("first page: next=%q err=%v", next, err)
	}
	writeTestFile(t, root, "after-page.txt", "new tip\n")
	gitTest(t, root, "add", "after-page.txt")
	gitTest(t, root, "commit", "-m", "commit after first page")
	page2, _, err := repo.Log(ctx, LogQuery{Scope: protocol.ScopeAll, Limit: 3, Cursor: next})
	if err != nil {
		t.Fatal(err)
	}
	combined := append(append([]protocol.GitCommit{}, page1...), page2...)
	if len(big) < len(combined) || !reflect.DeepEqual(combined, big[:len(combined)]) {
		t.Fatalf("paging changed after a new tip: combined=%v big=%v", combined, big)
	}
	for _, commit := range big {
		if commit.Hash == hidden {
			t.Fatalf("hidden checkpoint appeared in log: %s", hidden)
		}
	}
}

func gitTestNumber(value int) string {
	return string(rune('0' + value))
}

func TestCheckpointRestoreExactAndPreservesIgnored(t *testing.T) {
	ctx := context.Background()
	root := newTestRepo(t)
	writeTestFile(t, root, ".gitignore", "ignored.txt\n")
	writeTestFile(t, root, "tracked.txt", "tracked before\n")
	writeTestFile(t, root, "loose.txt", "loose before\n")
	gitTest(t, root, "add", ".gitignore", "tracked.txt")
	gitTest(t, root, "commit", "-m", "checkpoint base")
	if _, err := Checkpoint(ctx, root, "refs/rashin/session/turn-1"); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, root, "tracked.txt", "tracked after\n")
	writeTestFile(t, root, "loose.txt", "loose after\n")
	writeTestFile(t, root, "created.txt", "created after\n")
	writeTestFile(t, root, "ignored.txt", "keep ignored\n")
	if err := Restore(ctx, root, "refs/rashin/session/turn-1"); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"tracked.txt": "tracked before\n", "loose.txt": "loose before\n", "ignored.txt": "keep ignored\n"} {
		got, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || string(got) != want {
			t.Fatalf("%s: got %q err=%v want %q", name, got, err, want)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "created.txt")); !os.IsNotExist(err) {
		t.Fatalf("created file remains: %v", err)
	}
}

func TestMergeConflictAndAbort(t *testing.T) {
	ctx := context.Background()
	root := newTestRepo(t)
	gitTest(t, root, "checkout", "-b", "side")
	writeTestFile(t, root, "seed.txt", "side\n")
	gitTest(t, root, "add", "seed.txt")
	gitTest(t, root, "commit", "-m", "side")
	gitTest(t, root, "checkout", "main")
	writeTestFile(t, root, "seed.txt", "main\n")
	gitTest(t, root, "add", "seed.txt")
	gitTest(t, root, "commit", "-m", "main")
	result, err := Merge(ctx, root, "side", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.OK || result.Operation != protocol.OpMerge || !reflect.DeepEqual(result.Conflicts, []string{"seed.txt"}) {
		t.Fatalf("merge conflict not reported: %+v", result)
	}
	aborted, err := Abort(ctx, root)
	if err != nil || !aborted.OK {
		t.Fatalf("abort: %+v %v", aborted, err)
	}
	if Operation(ctx, root) != "" {
		t.Fatalf("operation remains after abort: %s", Operation(ctx, root))
	}
}

func TestRejectsOptionLikeRevisionsAndBranches(t *testing.T) {
	ctx := context.Background()
	root := newTestRepo(t)
	repo, err := Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	gitTest(t, root, "branch", "feature")
	target := filepath.Join(t.TempDir(), "injected")
	malicious := "--output=" + target
	if _, err := Diff(ctx, root, DiffOptions{Mode: protocol.DiffRange, From: malicious, To: "HEAD"}); err == nil {
		t.Fatal("diff accepted an option-like revision")
	}
	if _, _, err := repo.Log(ctx, LogQuery{Scope: protocol.ScopeBranch, Ref: malicious}); err == nil {
		t.Fatal("log accepted an option-like revision")
	}
	if _, err := repo.Show(ctx, malicious); err == nil {
		t.Fatal("show accepted an option-like revision")
	}
	if err := repo.CreateBranch(ctx, "-D", "feature"); err == nil {
		t.Fatal("branch creation accepted an option-like name")
	}
	if err := repo.CreateBranch(ctx, "safe-name", malicious); err == nil {
		t.Fatal("branch creation accepted an option-like start revision")
	}
	if err := repo.DeleteBranch(ctx, "-D", true); err == nil {
		t.Fatal("branch deletion accepted an option-like name")
	}
	operations := []struct {
		name string
		run  func() error
	}{
		{"checkout", func() error { _, err := Checkout(ctx, root, "--detach"); return err }},
		{"merge", func() error { _, err := Merge(ctx, root, "--abort", false, false); return err }},
		{"cherry-pick", func() error { _, err := CherryPick(ctx, root, []string{"--abort"}); return err }},
		{"revert", func() error { _, err := Revert(ctx, root, []string{"--abort"}); return err }},
		{"rebase", func() error { _, err := Rebase(ctx, root, "--exec=touch "+target); return err }},
	}
	for _, operation := range operations {
		if err := operation.run(); err == nil {
			t.Fatalf("%s accepted an option-like revision", operation.name)
		}
	}
	if err := repo.AddWorktree(ctx, AddOptions{Path: filepath.Join(t.TempDir(), "bad-branch"), Mode: protocol.WorktreeNewBranch, Branch: "-D", Base: "HEAD"}); err == nil {
		t.Fatal("worktree creation accepted an option-like branch")
	}
	if err := repo.AddWorktree(ctx, AddOptions{Path: filepath.Join(t.TempDir(), "bad-base"), Mode: protocol.WorktreeNewBranch, Branch: "safe-worktree", Base: malicious}); err == nil {
		t.Fatal("worktree creation accepted an option-like base")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("option injection created %s: %v", target, err)
	}
	gitTest(t, root, "show-ref", "--verify", "refs/heads/feature")
}

func TestLiteralPathspecs(t *testing.T) {
	ctx := context.Background()
	root := newTestRepo(t)
	writeTestFile(t, root, "notes*", "literal\n")
	writeTestFile(t, root, "notes-important.md", "keep\n")
	if err := Discard(ctx, root, []string{"notes*"}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "notes*")); !os.IsNotExist(err) {
		t.Fatalf("literal file remains: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "notes-important.md")); err != nil || string(got) != "keep\n" {
		t.Fatalf("glob-like path removed another file: %q %v", got, err)
	}
	writeTestFile(t, root, "stage*", "literal\n")
	writeTestFile(t, root, "stage-other", "keep\n")
	if err := Stage(ctx, root, []string{"stage*"}, false); err != nil {
		t.Fatal(err)
	}
	if got := gitTest(t, root, "diff", "--cached", "--name-only"); got != "stage*" {
		t.Fatalf("stage expanded a literal pathspec: %q", got)
	}
}

func TestDetachedUnpushedPreviewNeedsForce(t *testing.T) {
	ctx := context.Background()
	root := newTestRepo(t)
	repo, err := Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "detached")
	if err := repo.AddWorktree(ctx, AddOptions{Path: path, Mode: protocol.WorktreeDetached, Base: "main"}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, path, "precious.txt", "precious\n")
	gitTest(t, path, "add", "precious.txt")
	gitTest(t, path, "commit", "-m", "precious")
	worktrees, err := repo.Worktrees(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var detached Worktree
	for _, worktree := range worktrees {
		if worktree.Path == path {
			detached = worktree
		}
	}
	preview, err := repo.RemovePreview(ctx, detached, "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Unpushed) == 0 || !preview.NeedsForce || !preview.Unmerged {
		t.Fatalf("detached work was not protected: %+v", preview)
	}
}

func TestPureRenameCountsStayZero(t *testing.T) {
	ctx := context.Background()
	root := newTestRepo(t)
	writeTestFile(t, root, "old.txt", "one\ntwo\nthree\n")
	gitTest(t, root, "add", "old.txt")
	gitTest(t, root, "commit", "-m", "rename base")
	gitTest(t, root, "mv", "old.txt", "new.txt")
	statusResult, err := Status(ctx, root, "")
	if err != nil {
		t.Fatal(err)
	}
	var statusRename bool
	for _, file := range statusResult.Files {
		if file.Status == protocol.ChangeRenamed {
			statusRename = file.OldPath == "old.txt" && file.Path == "new.txt" && file.Added == 0 && file.Removed == 0
		}
	}
	diffResult, err := Diff(ctx, root, DiffOptions{Mode: protocol.DiffStaged, Context: 3})
	if err != nil {
		t.Fatal(err)
	}
	var diffRename bool
	for _, file := range diffResult.Files {
		if file.Status == protocol.ChangeRenamed {
			diffRename = file.OldPath == "old.txt" && file.Path == "new.txt" && file.Added == 0 && file.Removed == 0 && len(file.Hunks) == 0
		}
	}
	gitTest(t, root, "commit", "-m", "pure rename")
	repo, err := Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	detail, err := repo.Show(ctx, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	var showRename bool
	for _, file := range detail.Files {
		if file.Status == protocol.ChangeRenamed {
			showRename = file.OldPath == "old.txt" && file.Path == "new.txt" && file.Added == 0 && file.Removed == 0
		}
	}
	if !statusRename || !diffRename || !showRename {
		t.Fatalf("rename counts changed: status=%+v diff=%+v show=%+v", statusResult, diffResult, detail)
	}
}

func TestTextContainingBinaryHeaderIsNotBinary(t *testing.T) {
	patch := []byte("diff --git a/file b/file\n--- a/file\n+++ b/file\n@@ -1 +1 @@\n-Binary files old\n+Binary files new\n")
	file := parsePatch(patch, namedDiff{status: protocol.ChangeModified, path: "file"})
	if file.Binary || len(file.Hunks) != 1 {
		t.Fatalf("text patch was classified as binary: %+v", file)
	}
}

func TestAuthorSearchUsesFixedString(t *testing.T) {
	ctx := context.Background()
	root := newTestRepo(t)
	gitTest(t, root, "config", "user.name", "a+b")
	writeTestFile(t, root, "author.txt", "author\n")
	gitTest(t, root, "add", "author.txt")
	gitTest(t, root, "commit", "-m", "fixed author")
	repo, err := Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	commits, _, err := repo.Log(ctx, LogQuery{Scope: protocol.ScopeAll, Search: "a+b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 1 || commits[0].Subject != "fixed author" {
		t.Fatalf("fixed author search failed: %+v", commits)
	}
}

func TestInheritedRepositoryEnvironmentIsRemoved(t *testing.T) {
	ctx := context.Background()
	root := newTestRepo(t)
	other := newTestRepo(t)
	t.Setenv("GIT_DIR", filepath.Join(other, ".git"))
	t.Setenv("GIT_WORK_TREE", other)
	top, err := TopLevel(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if top != root {
		t.Fatalf("inherited Git environment redirected repository: %s", top)
	}
}

func TestPushSetUpstreamUsesOnlyRemote(t *testing.T) {
	ctx := context.Background()
	root := newTestRepo(t)
	bare := t.TempDir()
	gitTest(t, bare, "init", "--bare")
	gitTest(t, root, "remote", "add", "upstream", bare)
	result, err := Push(ctx, root, true, false)
	if err != nil || !result.OK {
		t.Fatalf("push failed: %+v %v", result, err)
	}
	if got := gitTest(t, root, "config", "branch.main.remote"); got != "upstream" {
		t.Fatalf("wrong upstream remote: %q", got)
	}
}

func TestRemovePreviewExcludesSurvivingLocalBranches(t *testing.T) {
	ctx := context.Background()
	root := newTestRepo(t)
	repo, err := Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "feature")
	if err := repo.AddWorktree(ctx, AddOptions{Path: path, Mode: protocol.WorktreeNewBranch, Branch: "feature-survives", Base: "main"}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, path, "feature.txt", "feature\n")
	gitTest(t, path, "add", "feature.txt")
	gitTest(t, path, "commit", "-m", "feature commit")
	gitTest(t, root, "branch", "keeper", "feature-survives")
	worktrees, err := repo.Worktrees(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var feature Worktree
	for _, worktree := range worktrees {
		if worktree.Path == path {
			feature = worktree
		}
	}
	preview, err := repo.RemovePreview(ctx, feature, "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Unpushed) != 0 || !preview.Unmerged || !preview.NeedsForce {
		t.Fatalf("surviving branch safety was wrong: %+v", preview)
	}
}

func TestPruneReturnsWorktreePaths(t *testing.T) {
	ctx := context.Background()
	root := newTestRepo(t)
	repo, err := Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "prunable")
	if err := repo.AddWorktree(ctx, AddOptions{Path: path, Mode: protocol.WorktreeNewBranch, Branch: "prunable", Base: "main"}); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
	dry, err := repo.PruneWorktrees(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(dry, []string{path}) {
		t.Fatalf("dry prune paths: %+v", dry)
	}
	before, err := repo.Worktrees(ctx)
	if err != nil || len(before) != 2 {
		t.Fatalf("dry prune changed worktrees: %+v %v", before, err)
	}
	pruned, err := repo.PruneWorktrees(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(pruned, []string{path}) {
		t.Fatalf("pruned paths: %+v", pruned)
	}
	after, err := repo.Worktrees(ctx)
	if err != nil || len(after) != 1 {
		t.Fatalf("prune left worktree metadata: %+v %v", after, err)
	}
}
