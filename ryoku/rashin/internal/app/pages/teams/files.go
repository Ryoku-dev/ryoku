package teams

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	appstate "ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

const (
	// the newest files the section lists
	filesShown = 6
	// how many folders deep under the team's own the scan reads, and how many entries at most, so a large folder costs little
	filesDepth = 2
	filesRead  = 4000
)

// a file in the team's folder and when it last changed
type teamFile struct {
	path     string
	modified time.Time
}

type fileRow struct {
	link   linkRow
	report kit.Click
	reveal kit.IconPlaque
	tip    look.Tip
}

// the newest files in the team's folder, read off the frame when the page opens and when a task ends
type teamFiles struct {
	load  load
	team  string
	files []teamFile
	rows  map[string]*fileRow
}

// the page opened again, so the folder is read again
func (f *teamFiles) stale() { f.load.key = "" }

// a task that ends is what changes the folder, so the scan follows the team's finished and working counts
func (f *teamFiles) want(e *env.Env, team protocol.Team) {
	stats := team.Stats
	last := int64(0)
	if stats.LastReport != nil {
		last = stats.LastReport.UnixNano()
	}
	key := fmt.Sprintf("%s/%d/%s/%d/%d/%d", team.ID, e.Snapshot().Epoch, team.Folder, stats.Done, stats.Working, last)
	f.load.want(e, key, func(done func()) {
		folder := team.Folder
		go func() {
			found := scanFiles(folder)
			e.Post(func() {
				done()
				f.files, f.team = found, team.ID
			})
		}()
	})
}

func (f *teamFiles) row(path string) *fileRow {
	if f.rows == nil {
		f.rows = make(map[string]*fileRow)
	}
	row := f.rows[path]
	if row == nil {
		row = new(fileRow)
		f.rows[path] = row
	}
	return row
}

// the section: each file opens in what opens it, shows in its folder, and names the report it came from
func (f *teamFiles) layout(gtx layout.Context, e *env.Env, snapshot appstate.Snapshot, team protocol.Team) layout.Widget {
	th := e.Theme
	f.want(e, team)
	if f.team != team.ID || len(f.files) == 0 {
		return section(th, i18n.T("Files"), quiet(th, i18n.T("Nothing in the team's folder yet. What its tasks make lands here.")))
	}
	reports := reportFiles(snapshot, team.ID)
	kept := make(map[string]*fileRow, len(f.files))
	rows := make([]layout.Widget, len(f.files))
	for index, file := range f.files {
		row := f.row(file.path)
		kept[file.path] = row
		task, fromReport := reports[file.path]
		row.reveal.Kind, row.reveal.Small, row.reveal.Icon = kit.PlaqueQuiet, true, look.Icon.FolderOpen
		row.reveal.Description = i18n.T("Show in folder")
		// the row hears the clicks of what stands on it, so those are taken first and win
		reveal, report, open := row.reveal.Clicked(gtx), row.report.Clicked(gtx), row.link.click.Clicked(gtx)
		switch {
		case reveal:
			e.OpenFolder(filepath.Dir(file.path))
		case report && fromReport:
			teamsui.OpenTask(e, task.ID)
		case open:
			e.OpenPath(file.path)
		}
		name := filepath.Base(file.path)
		where, _ := filepath.Rel(team.Folder, filepath.Dir(file.path))
		rows[index] = func(gtx layout.Context) layout.Dimensions {
			return row.link.layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(look.IconWidget(look.Icon.File, 16, th.R.Fg3)),
					layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return kit.Label{Style: th.T.Label, Color: th.R.Fg, MaxLines: 1}.Layout(gtx, th.Theme, name)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return fileFacts(gtx, th, row, file, where, task, fromReport)
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return row.tip.Layout(gtx, th, e.Overlays, row.reveal.Description, func(gtx layout.Context) layout.Dimensions {
							return row.reveal.Layout(gtx, th.Theme)
						})
					}),
				)
			})
		}
	}
	f.rows = kept
	return section(th, i18n.T("Files"), stack(th, rows))
}

// where the file stands, when it changed and the report that names it, which opens its task, set apart by space
func fileFacts(gtx layout.Context, th *look.Theme, row *fileRow, file teamFile, where string, task protocol.TeamTask, fromReport bool) layout.Dimensions {
	gap := layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout)
	var children []layout.FlexChild
	// the folder is a path, so it is the one literal on the line
	if where != "." && where != "" {
		children = append(children, layout.Rigid(kit.Text(th.Theme, th.T.MonoSm, th.R.Fg3, where)), gap)
	}
	children = append(children, layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, look.Ago(gtx.Now, file.modified))))
	if fromReport {
		title := task.Report.Title
		if title == "" {
			title = task.Title
		}
		children = append(children, gap, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return look.Link(gtx, th, &row.report, i18n.Tf("From “%s”", title))
		}))
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx, children...)
}

// the files the team's reports name, each with the newest report that names it
func reportFiles(snapshot appstate.Snapshot, team string) map[string]protocol.TeamTask {
	reports := make(map[string]protocol.TeamTask)
	for _, task := range snapshot.TeamTasks {
		if task.Team != team || task.Report == nil {
			continue
		}
		for _, path := range task.Report.Files {
			if known, ok := reports[path]; !ok || task.Report.At.After(known.Report.At) {
				reports[path] = task
			}
		}
	}
	return reports
}

// the newest files under folder, at most filesDepth folders down and filesRead entries in all;
// hidden names, such as what Rashin keeps for the team, are left out
func scanFiles(folder string) []teamFile {
	var found []teamFile
	budget := filesRead
	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		if budget <= 0 {
			return
		}
		handle, err := os.Open(dir)
		if err != nil {
			return
		}
		entries, _ := handle.ReadDir(budget)
		handle.Close()
		budget -= len(entries)
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			path := filepath.Join(dir, name)
			if entry.IsDir() {
				if depth < filesDepth {
					walk(path, depth+1)
				}
				continue
			}
			if !entry.Type().IsRegular() {
				continue
			}
			if info, err := entry.Info(); err == nil {
				found = append(found, teamFile{path: path, modified: info.ModTime()})
			}
		}
	}
	walk(folder, 0)
	sort.Slice(found, func(i, j int) bool { return found[i].modified.After(found[j].modified) })
	// the newest few keep no hold on everything the scan read
	return append([]teamFile(nil), found[:min(len(found), filesShown)]...)
}
