package chat

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"gioui.org/gpu/headless"
	"gioui.org/io/clipboard"
	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
)

func WritePicture(gtx layout.Context, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 128<<20 {
		file.Close()
		return fmt.Errorf("%s", i18n.T("This picture is too large to copy."))
	}
	gtx.Execute(clipboard.WriteCmd{Type: "image/png", Data: file})
	return nil
}

func pictureFolder() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	directory := filepath.Join(home, "Pictures", "Rashin")
	return directory, os.MkdirAll(directory, 0o700)
}

func (state *viewState) shareAnswer(gtx layout.Context, th *look.Theme, block blockContext) {
	question := retryAction(block.model, block.index, block.row.turn.number).Value
	answer := answerText(block.model, block.index)
	options := state.answerSettings()
	path, err := renderAnswerPicture(th, question, answer, gtx.Now, state.feature)
	if err == nil {
		err = WritePicture(gtx, path)
	}
	if err != nil {
		if options.Notify != nil {
			options.Notify(err.Error())
		}
		return
	}
	message := i18n.T("Picture copied and saved to Pictures/Rashin")
	if options.NotifyAction != nil && options.OpenFolder != nil {
		options.NotifyAction(message, i18n.T("Open folder"), func() { options.OpenFolder(filepath.Dir(path)) })
	} else if options.Notify != nil {
		options.Notify(message)
	}
}

func renderAnswerPicture(th *look.Theme, question, answer string, date time.Time, feature func(string) bool) (string, error) {
	const width, tileHeight, maxHeight = 1080, 2048, 32768
	var operations op.Ops
	var router input.Router
	gtx := layout.Context{Ops: &operations, Now: date, Source: router.Source(), Metric: unit.Metric{PxPerDp: 1.5, PxPerSp: 1.5}, Constraints: layout.Constraints{Max: image.Pt(width, maxHeight)}}
	text := new(viewState)
	text.ensure()
	text.features = feature
	body := func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = width
		return layout.UniformInset(32).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(look.RyokuMark(28, th.R.Fg)),
						layout.Rigid(layout.Spacer{Width: 13}.Layout),
						layout.Rigid(kit.Text(th.Theme, th.T.Section, th.R.Fg, i18n.T("Rashin"))),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.E.Layout(gtx, kit.Text(th.Theme, th.T.Caption, th.R.Fg3, date.Format("2 Jan 2006")))
						}),
					)
				}),
				layout.Rigid(layout.Spacer{Height: 24}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return text.layoutQuestionSlab(gtx, th, "share:question", th.T.Proof, question)
				}),
				layout.Rigid(layout.Spacer{Height: 24}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return text.layoutMarkdown(gtx, th, "share:answer", answer, nil)
				}),
			)
		})
	}
	measured := body(gtx)
	height := measured.Size.Y
	if height >= maxHeight || height <= 0 {
		return "", fmt.Errorf("%s", i18n.T("This answer is too long to share as one picture."))
	}
	window, err := headless.NewWindow(width, min(tileHeight, height))
	if err != nil {
		return "", fmt.Errorf("%s: %w", i18n.T("Could not make the picture"), err)
	}
	defer window.Release()
	picture := image.NewRGBA(image.Rect(0, 0, width, height))
	tile := image.NewRGBA(image.Rect(0, 0, width, min(tileHeight, height)))
	for top := 0; top < height; top += tileHeight {
		operations.Reset()
		paint.Fill(&operations, th.R.Ground)
		offset := op.Offset(image.Pt(0, -top)).Push(&operations)
		kit.FillCut(&operations, image.Rect(12, 12, width-12, height-12), gtx.Dp(th.Cut.Mid), th.R.Stone2)
		gtx.Constraints = layout.Constraints{Max: image.Pt(width, maxHeight)}
		body(gtx)
		offset.Pop()
		if err := window.Frame(&operations); err != nil {
			return "", err
		}
		if err := window.Screenshot(tile); err != nil {
			return "", err
		}
		draw.Draw(picture, image.Rect(0, top, width, min(top+tileHeight, height)), tile, image.Point{}, draw.Src)
	}
	directory, err := pictureFolder()
	if err != nil {
		return "", err
	}
	file, err := os.CreateTemp(directory, "Rashin-"+date.Format("20060102-150405")+"-*.png")
	if err != nil {
		return "", err
	}
	name := file.Name()
	writeErr := png.Encode(file, picture)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		os.Remove(name)
		if writeErr != nil {
			return "", writeErr
		}
		return "", closeErr
	}
	return name, nil
}
