package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// videoPlayer drives video wallpapers through ryogami-live, one process per
// live output. Each process owns its own process group and sequence token, so
// replacing one workspace's clip cannot stop or revive another output's clip.
// Play is asynchronous: the one-time transcode runs off the hot path while the
// shell paints the poster. Each player announces READY after its first frame;
// the shell yields only the addressed output once every process in that request
// is ready, and keeps the poster visible if they all exit.
type videoPlayer struct {
	mu      sync.Mutex
	gen     int64
	procs   map[string]*liveProc
	slotGen map[string]int64
	log     *os.File
	path    string
	outputs []string
	deaths  map[string]int
	deathAt map[string]time.Time
}

type playGroup struct {
	expect    int
	announced map[string]bool
	onLive    func(bool)
}

// liveProc is one player bound to one output slot.
type liveProc struct {
	cmd    *exec.Cmd
	output string
	file   string
	capW   int
	fit    string
	gen    int64
	ready  bool
	group  *playGroup
}

func newVideoPlayer() *videoPlayer {
	return &videoPlayer{
		procs:   map[string]*liveProc{},
		slotGen: map[string]int64{},
		deaths:  map[string]int{},
		deathAt: map[string]time.Time{},
	}
}

// liveSlots is the set of outputs a broadcast video spans: every connected
// output by name, or the NULL slot ("", the compositor's primary) when the
// list can't be read.
func liveSlots() []string {
	outs := outputs.list()
	if len(outs) == 0 {
		return []string{""}
	}
	slots := make([]string, 0, len(outs))
	for _, o := range outs {
		slots = append(slots, o.Name)
	}
	return slots
}

// Play replaces only the addressed output players. A broadcast replaces every
// player; a workspace switch on one output leaves every other output alone.
func (p *videoPlayer) Play(outputs []string, path, fit, tier string, onLive func(bool)) {
	all := len(outputs) == 0 || contains(outputs, "*")
	targets := append([]string(nil), outputs...)
	if all {
		targets = liveSlots()
	}

	p.mu.Lock()
	p.gen++
	gen := p.gen
	var killed []*liveProc
	var log *os.File
	if all {
		killed = p.takeLocked(nil)
		log = p.log
		p.log = nil
	} else {
		killed = p.takeLocked(targets)
	}
	if p.log == nil {
		if f, err := os.OpenFile(managedLogPath("video"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
			p.log = f
		}
	}
	group := &playGroup{expect: len(targets), announced: map[string]bool{}, onLive: onLive}
	for _, output := range targets {
		p.slotGen[output] = gen
		delete(p.deaths, output)
		delete(p.deathAt, output)
	}
	p.path = path
	if all {
		p.outputs = []string{"*"}
	} else {
		p.outputs = append([]string(nil), outputs...)
	}
	p.mu.Unlock()

	reapLivePlayers(killed, log, all)

	go func() {
		capW := liveCapWidth(tier)
		src := liveProbe(path)
		fps := liveFps(tier, src)
		file := livewallSource(path, src, capW, fps)
		if file == "" {
			fmt.Fprintf(os.Stderr, "ryogami: transcode failed for %s; keeping the still\n", path)
			return
		}
		p.mu.Lock()
		defer p.mu.Unlock()
		for _, output := range targets {
			if p.slotGen[output] == gen {
				p.spawnLocked(gen, output, file, capW, fit, group)
			}
		}
	}()
}

// takeLocked moves the tracked players (all, or those on the named outputs)
// out of the bookkeeping; caller holds the lock. A nil or "*" filter takes all.
func (p *videoPlayer) takeLocked(want []string) []*liveProc {
	all := len(want) == 0
	for _, o := range want {
		if o == "*" {
			all = true
		}
	}
	taken := make([]*liveProc, 0, len(p.procs))
	for slot, lp := range p.procs {
		if !all && !contains(want, slot) {
			continue
		}
		taken = append(taken, lp)
		delete(p.procs, slot)
	}
	return taken
}

// reapLivePlayers kills the addressed process groups. A full replacement also
// reaps orphan renderers left by an older daemon; a one-output switch must not
// kill another output's player.
func reapLivePlayers(procs []*liveProc, log *os.File, reapOrphans bool) {
	for _, lp := range procs {
		if lp.cmd.Process != nil {
			_ = syscall.Kill(-lp.cmd.Process.Pid, syscall.SIGKILL)
		}
	}
	if log != nil {
		_ = log.Close()
	}
	if !reapOrphans {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = exec.CommandContext(ctx, "pkill", "-x", liveDaemon).Run()
	_ = exec.CommandContext(ctx, "pkill", "-f", "mpvpaper").Run()
}

// StopOutputs tears down the players on the named outputs (empty or "*" means
// every one), leaving the rest of the bookkeeping intact.
func (p *videoPlayer) StopOutputs(want []string) {
	all := len(want) == 0 || contains(want, "*")
	p.mu.Lock()
	p.gen++
	gen := p.gen
	killed := p.takeLocked(want)
	var log *os.File
	if all {
		p.slotGen = map[string]int64{}
		log = p.log
		p.log = nil
	} else {
		for _, output := range want {
			p.slotGen[output] = gen
		}
	}
	p.mu.Unlock()
	reapLivePlayers(killed, log, all)
}

// spawnLocked launches one player bound to output ("" = compositor primary);
// caller holds the lock. Stdout is scanned for the READY handshake and teed to
// the managed log. A crash while the player still believes it should be
// running is logged to stderr, and respawn brings it back with a bounded
// backoff: a video wallpaper must not silently stay a still until next login.
func (p *videoPlayer) spawnLocked(gen int64, output, file string, capW int, fit string, group *playGroup) {
	args := []string{file, strconv.Itoa(capW), fit}
	if output != "" && output != "*" {
		args = append(args, output)
	}
	cmd := exec.Command(liveDaemon, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdin = nil
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdout = nil
	}
	if p.log != nil {
		if stdout == nil {
			cmd.Stdout = p.log
		}
		cmd.Stderr = p.log
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "ryogami: launch %s: %v (build it with ryoku/shell/livewall/build.sh)\n", liveDaemon, err)
		if group.expect > 0 {
			group.expect--
		}
		return
	}
	lp := &liveProc{cmd: cmd, output: output, file: file, capW: capW, fit: fit, gen: gen, group: group}
	p.procs[output] = lp
	if stdout != nil {
		go p.scanReady(lp, stdout)
	}
	go func() {
		err := cmd.Wait()
		p.mu.Lock()
		// Still recorded means Stop did not remove it: an unexpected exit.
		tracked := p.procs[output] == lp && p.slotGen[output] == gen
		if tracked {
			delete(p.procs, output)
		}
		groupGone := tracked && lp.ready
		if groupGone {
			for _, other := range p.procs {
				if other.group == lp.group {
					groupGone = false
					break
				}
			}
		}
		cb := lp.group.onLive
		p.mu.Unlock()
		if tracked {
			fmt.Fprintf(os.Stderr, "ryogami: %s on %q exited unexpectedly: %v\n", liveDaemon, output, err)
		}
		if groupGone && cb != nil {
			cb(false)
		}
		if tracked {
			p.respawn(lp)
		}
	}()
}

// respawn brings back a player that died under the daemon (a compositor
// output cycle, a suspend, a stray kill), so a video wallpaper never stays a
// still until the next login. Backoff steps 1s, 2s, 4s, 8s and gives up after
// the fifth death within a minute (a clip that cannot play at all would spin
// otherwise); Stop or a new Play (a generation change) cancels a pending one.
func (p *videoPlayer) respawn(lp *liveProc) {
	p.mu.Lock()
	now := time.Now()
	if now.Sub(p.deathAt[lp.output]) > time.Minute {
		p.deaths[lp.output] = 0
	}
	p.deaths[lp.output]++
	p.deathAt[lp.output] = now
	n := p.deaths[lp.output]
	p.mu.Unlock()
	if n > 5 {
		fmt.Fprintf(os.Stderr, "ryogami: %s on %q keeps dying; leaving the still\n", liveDaemon, lp.output)
		return
	}
	delay := time.Second << (n - 1)
	if delay > 8*time.Second {
		delay = 8 * time.Second
	}
	time.Sleep(delay)
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.slotGen[lp.output] != lp.gen {
		return
	}
	delete(lp.group.announced, lp.output)
	p.spawnLocked(lp.gen, lp.output, lp.file, lp.capW, lp.fit, lp.group)
}

// scanReady tees one player's stdout into the managed log and marks its output
// announced on that output's first READY of the current generation. onLive
// fires with true only once every requested output has presented, so the
// shell never yields a screen whose player has not committed a frame yet.
func (p *videoPlayer) scanReady(lp *liveProc, r io.Reader) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		p.mu.Lock()
		if p.log != nil {
			fmt.Fprintln(p.log, line)
		}
		var cb func(bool)
		if strings.TrimSpace(line) == "READY" && p.procs[lp.output] == lp &&
			p.slotGen[lp.output] == lp.gen && !lp.group.announced[lp.output] {
			lp.ready = true
			lp.group.announced[lp.output] = true
			if lp.group.expect > 0 && len(lp.group.announced) >= lp.group.expect {
				cb = lp.group.onLive
			}
		}
		p.mu.Unlock()
		if cb != nil {
			cb(true)
		}
	}
}

// Stop invalidates any in-flight transcode, kills every spawned player (whole
// process group) and, mirroring how process.go clears a stale picker, pkills
// orphans: livewall from a crashed daemon, mpvpaper from releases that shipped
// it (an orphan's surface stacks above ours, hiding every later wallpaper).
func (p *videoPlayer) Stop() {
	p.mu.Lock()
	p.gen++
	killed := p.takeLocked(nil)
	log := p.log
	p.log = nil
	p.path = ""
	p.outputs = nil
	p.slotGen = map[string]int64{}
	p.mu.Unlock()

	reapLivePlayers(killed, log, true)
}

// Playing reports whether any player process is tracked.
func (p *videoPlayer) Playing() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.procs) > 0
}

// ReadyOutputs reports whether every addressed output has committed a frame.
func (p *videoPlayer) ReadyOutputs(want []string) bool {
	targets := want
	if len(targets) == 0 || contains(targets, "*") {
		targets = liveSlots()
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, output := range targets {
		lp := p.procs[output]
		if lp == nil || !lp.ready {
			return false
		}
	}
	return len(targets) > 0
}

func (p *videoPlayer) Ready() bool {
	return p.ReadyOutputs(nil)
}

func (p *videoPlayer) Current() (path string, outputs []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.path, append([]string(nil), p.outputs...)
}
