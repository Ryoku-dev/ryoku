package chat

import (
	"image"
	"io"
	"net/url"
	"os"
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/transfer"
	"gioui.org/layout"
	"gioui.org/op/clip"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"ryoku-i18n"
)

func (state *composerState) registerDropTarget(gtx layout.Context, size image.Point) {
	defer clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, &state.dropTag)
}

func (state *composerState) processDropEvents(gtx layout.Context) {
	for _, mime := range [...]string{"text/uri-list", "image/png", "image/jpeg"} {
		filter := transfer.TargetFilter{Target: &state.dropTag, Type: mime}
		for {
			raw, ok := gtx.Event(filter)
			if !ok {
				break
			}
			switch dropped := raw.(type) {
			case transfer.InitiateEvent:
				state.dropActive = true
			case transfer.CancelEvent:
				state.dropActive = false
			case transfer.DataEvent:
				state.dropActive = false
				reader := dropped.Open()
				if reader == nil {
					continue
				}
				if dropped.Type == "text/uri-list" {
					data, _ := io.ReadAll(io.LimitReader(reader, 1<<20))
					_ = reader.Close()
					state.addDroppedURIs(string(data))
					continue
				}
				state.saveDroppedImage(reader, dropped.Type)
			}
		}
	}
}

func (state *composerState) addDroppedURIs(value string) {
	for _, line := range strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parsed, err := url.Parse(line)
		switch {
		case err != nil:
		case parsed.Scheme == "file":
			state.addAttachment(parsed.Path)
		case parsed.Scheme == "":
			state.addAttachment(line)
		}
	}
}

// the daemon takes attachments by path, so dropped image bytes go to a temp file
func (state *composerState) saveDroppedImage(reader io.ReadCloser, mime string) {
	defer reader.Close()
	extension := ".png"
	if mime == "image/jpeg" {
		extension = ".jpg"
	}
	file, err := os.CreateTemp("", "rashin-attachment-*"+extension)
	if err != nil {
		return
	}
	path := file.Name()
	_, copyErr := io.Copy(file, io.LimitReader(reader, 32<<20))
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(path)
		return
	}
	state.addAttachment(path)
}

func (state *composerState) layoutDropTarget(gtx layout.Context, th *theme.Theme, size image.Point) {
	bounds := image.Rectangle{Max: size}
	cut := gtx.Dp(th.Cut.Mid)
	kit.FillCut(gtx.Ops, bounds, cut, th.R.Stone4)
	kit.StrokeCut(gtx.Ops, bounds, cut, max(1, gtx.Dp(1)), th.R.Vein)
	child := gtx
	child.Constraints = layout.Exact(size)
	layout.Center.Layout(child, kit.Text(th, th.T.Label, th.R.Fg, i18n.T("Drop to attach")))
}
