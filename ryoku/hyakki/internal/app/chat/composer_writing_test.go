package chat

import (
	"image"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/design"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

func TestComposerWritingMessageOnlyStylesCodeSpans(t *testing.T) {
	for _, test := range []struct {
		name, source string
		want         []inlineSpan
	}{
		{
			name:   "Harper explanation",
			source: "Use `their` instead of `there` here.",
			want:   []inlineSpan{{text: "Use "}, {text: "their", style: inlineCode}, {text: " instead of "}, {text: "there", style: inlineCode}, {text: " here."}},
		},
		{
			name:   "literal markup",
			source: "<b>Use</b><br> **this** [word](https://example.com) &amp; `<script>x</script>`.",
			want:   []inlineSpan{{text: "<b>Use</b><br> **this** [word](https://example.com) &amp; "}, {text: "<script>x</script>", style: inlineCode}, {text: "."}},
		},
		{
			name:   "unmatched backticks",
			source: "Keep `this literal.",
			want:   []inlineSpan{{text: "Keep `this literal."}},
		},
		{
			name:   "backtick within code",
			source: "Use ``a`b`` here.",
			want:   []inlineSpan{{text: "Use "}, {text: "a`b", style: inlineCode}, {text: " here."}},
		},
		{
			name:   "Unicode",
			source: "Replace `teh` with `the` before 😀 中文.",
			want:   []inlineSpan{{text: "Replace "}, {text: "teh", style: inlineCode}, {text: " with "}, {text: "the", style: inlineCode}, {text: " before 😀 中文."}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := writingMessageSpans(test.source); !slices.Equal(got, test.want) {
				t.Fatalf("spans = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestComposerWritingIssuesReadWithoutBackticksInEveryDesign(t *testing.T) {
	for _, style := range []design.Style{design.Lithic, design.Material, design.Adwaita, design.Fluent} {
		t.Run(style.String(), func(t *testing.T) {
			for _, mode := range []palette.Mode{palette.Dark, palette.Light} {
				frame := newChatFrame(image.Pt(520, 320))
				frame.theme = look.NewWithRoles(palette.FromPalette(palette.Load(mode), palette.Look{}))
				frame.theme.Design = style
				f := newComposerWritingFixture()
				state := f.composer.ensureState()
				state.ask.reviewWriting(state.editor.Text(), &protocol.WritingCheckResult{Issues: []protocol.WritingIssue{{
					Start: 0, End: 8, Category: "spelling", Message: "Use `Original` before <br> **draft**.", Replacements: []string{"Revised"},
				}}})
				state.layoutWritingIssues(frame.context(), frame.theme.Theme)
				frame.finish()
				want := "Use Original before <br> **draft**."
				found, previewFound := false, false
				for _, node := range frame.router.AppendSemantics(nil) {
					if node.Desc.Label == want {
						found = true
					}
					if node.Desc.Label == "Original → Revised" {
						previewFound = true
					}
				}
				if !found {
					t.Fatalf("design %s mode %v did not expose the literal explanation without code delimiters", style, mode)
				}
				if !previewFound {
					t.Fatalf("design %s mode %v hid the first replacement preview", style, mode)
				}
				actions := &state.ask.issueActions[0]
				if actions.message.faces.base != frame.theme.T.Detail || actions.message.faces.mono != codeStyle(frame.theme) {
					t.Fatal("the issue did not use the current design's detail and inline code typography")
				}
				if actions.apply.Label != "Apply" || actions.add.Label != "Add to dictionary" {
					t.Fatal("rendering the explanation removed its correction controls")
				}
				actions.apply.Clickable.Click()
				state.layoutWritingIssues(frame.context(), frame.theme.Theme)
				frame.finish()
				if state.editor.Text() != "Revised draft" || state.ask.writingResult != nil || state.ask.undoBefore != "Original draft" {
					t.Fatal("applying a rendered correction lost the draft or its undo source")
				}
			}
		})
	}
}

func TestComposerWritingMessageCachesAndRefreshesItsLayout(t *testing.T) {
	frame := newChatFrame(image.Pt(300, 200))
	var actions draftIssueActions
	layoutMessage := func(source string) {
		gtx := frame.context()
		gtx.Constraints.Min = image.Point{}
		if dimensions := actions.layoutMessage(gtx, frame.theme.Theme, source); dimensions == (layout.Dimensions{}) {
			t.Fatal("the explanation has no visible layout")
		}
		frame.finish()
	}
	layoutMessage("Use `their` here.")
	first := actions.message.set
	spans := &actions.message.spans[0]
	layoutMessage("Use `their` here.")
	if actions.message.set != first || &actions.message.spans[0] != spans {
		t.Fatal("an unchanged frame reparsed or reshaped the explanation")
	}
	frame.size.X = 120
	layoutMessage("Use `their` here.")
	if actions.message.set == first || &actions.message.spans[0] != spans {
		t.Fatal("resizing did not reshape the cached spans")
	}
	first = actions.message.set
	frame.theme.T.Detail.Size++
	layoutMessage("Use `their` here.")
	if actions.message.set == first {
		t.Fatal("current typography did not replace the cached layout")
	}
	layoutMessage("Use `there` instead.")
	if actions.message.set.label != "Use there instead." {
		t.Fatalf("a new explanation retained old text: %q", actions.message.set.label)
	}
}

func TestComposerWritingMessageKeepsDictionaryAndDismissActions(t *testing.T) {
	frame := newChatFrame(image.Pt(520, 320))
	f := newComposerWritingFixture()
	state := f.composer.ensureState()
	state.setText("Ryoku")
	state.ask.writingDraft = state.editor.Text()
	state.ask.writingResult = &protocol.WritingCheckResult{Issues: []protocol.WritingIssue{{
		Start: 0, End: 5, Category: "spelling", Message: "Did you mean `Ryoku`?",
	}}}
	state.ask.issueActions = make([]draftIssueActions, 1)
	draw := func() {
		state.layoutWritingIssues(frame.context(), frame.theme.Theme)
		frame.finish()
	}
	draw()
	draw()
	if len(f.calls) != 0 || state.ask.busy {
		t.Fatal("drawing the explanation started background work")
	}
	state.ask.issueActions[0].add.Clickable.Click()
	draw()
	if len(f.calls) != 1 || f.calls[0].method != protocol.MWritingWordsAdd || f.calls[0].params.(protocol.WritingWordParams).Word != "Ryoku" {
		t.Fatalf("the dictionary control did not add the draft word: %#v", f.calls)
	}
	f.calls[0].done(nil)
	if len(f.calls) != 2 || f.calls[1].method != protocol.MWritingCheck {
		t.Fatalf("the dictionary control did not refresh the check: %#v", f.calls)
	}
	f.calls[1].result.(*protocol.WritingCheckResult).Issues = []protocol.WritingIssue{{
		Start: 0, End: 5, Category: "spelling", Message: "Use `Ryoku` here.",
	}}
	f.calls[1].done(nil)
	draw()
	state.ask.dismissWriting.Clickable.Click()
	draw()
	if state.ask.writingResult != nil || state.editor.Text() != "Ryoku" || len(f.calls) != 2 {
		t.Fatal("dismissing a rendered explanation changed the draft or started work")
	}
}

func TestComposerWritingPreviewUsesExactUTF16SpanAndFirstReplacement(t *testing.T) {
	var composer askComposerState
	composer.reviewWriting("😀 中文 libary", &protocol.WritingCheckResult{Issues: []protocol.WritingIssue{
		{Start: 6, End: 12, Replacements: []string{"library", "libraries"}},
		{Start: 6, End: 12, Replacements: []string{""}},
		{Start: 6, End: 12},
	}})
	want := []string{"libary → library", "libary → ", ""}
	for index, preview := range want {
		if composer.issueActions[index].preview != preview {
			t.Fatalf("preview %d=%q, want %q", index, composer.issueActions[index].preview, preview)
		}
	}
}

func TestComposerWritingMenuOffersActionsWithoutCheckedPlaceholder(t *testing.T) {
	for _, style := range []design.Style{design.Lithic, design.Material, design.Adwaita, design.Fluent} {
		t.Run(style.String(), func(t *testing.T) {
			frame := newChatFrame(image.Pt(520, 320))
			frame.theme.Design = style
			f := newComposerWritingFixture()
			state := f.composer.ensureState()
			a := state.ask
			a.hooks.Overlays = &frame.overlays
			for _, voice := range []bool{false, true, false} {
				a.voice = voice
				items := a.writingActions()
				count := 5
				if voice {
					count++
				}
				if len(items) != count || items[0].Label != "Fix writing" {
					t.Fatalf("unexpected menu items: %#v", items)
				}
				for _, item := range items {
					if item.Checked || item.Disabled || item.Label == "Writing" {
						t.Fatalf("writing menu contains a selected placeholder: %#v", item)
					}
				}
			}
			gtx := frame.context()
			state.layoutWritingMenu(gtx, frame.theme.Theme)
			frame.overlays.Layout(gtx, frame.theme)
			frame.finish()
			a.writing.Clickable.Click()
			gtx = frame.context()
			state.layoutWritingMenu(gtx, frame.theme.Theme)
			frame.overlays.Layout(gtx, frame.theme)
			frame.finish()
			if !frame.overlays.IsOpen(&a.writing.Clickable) {
				t.Fatal("Writing did not open its action menu")
			}
			a.closeWritingMenu()
			if _, open := frame.overlays.Top(); open {
				t.Fatal("writing menu remained interactive after abandonment")
			}
		})
	}
}

func TestComposerWritingReviewScrollsWithoutOverlappingFooter(t *testing.T) {
	const sentence = "Their going to visit the libary tomorow with my freind."
	originals := []string{"Their", "libary", "tomorow", "freind"}
	replacements := []string{"They're", "library", "tomorrow", "friend"}
	for _, style := range []design.Style{design.Lithic, design.Material, design.Adwaita, design.Fluent} {
		for _, mode := range []palette.Mode{palette.Dark, palette.Light} {
			for _, count := range []int{4, 8} {
				t.Run(style.String()+"/"+strconv.Itoa(int(mode))+"/"+strconv.Itoa(count), func(t *testing.T) {
					frame := newChatFrame(image.Pt(520, 320))
					frame.theme = look.NewWithRoles(palette.FromPalette(palette.Load(mode), palette.Look{}))
					frame.theme.Design = style
					fixture := newComposerWritingFixture()
					state := fixture.composer.ensureState()
					draft := sentence
					if count == 8 {
						draft += " " + sentence
					}
					state.setText(draft)
					result := &protocol.WritingCheckResult{Engine: "local"}
					lastLabel := ""
					for index := range count {
						original := originals[index%4]
						start := index/4*(len(sentence)+1) + strings.Index(sentence, original)
						message := "Did you mean to spell `" + original + "` this way? Fix " + strconv.Itoa(index+1)
						result.Issues = append(result.Issues, protocol.WritingIssue{
							Start: start, End: start + len(original), Category: "spelling", Message: message,
							Replacements: []string{replacements[index%4]},
						})
						lastLabel = strings.ReplaceAll(message, "`", "")
					}
					state.ask.reviewWriting(draft, result)
					lastVisible := false
					draw := func() {
						t.Helper()
						gtx := frame.context()
						gtx.Constraints.Min = image.Point{}
						dimensions := state.layoutWritingIssues(gtx, frame.theme.Theme)
						if dimensions.Size.Y > frame.size.Y {
							t.Fatalf("review grew beyond available height: %v", dimensions.Size)
						}
						frame.finish()
						nodes := frame.router.AppendSemantics(nil)
						footerTop, footerBottom := frame.size.Y, 0
						footerFound := false
						for _, node := range nodes {
							if node.Desc.Label == "Apply fixes" || node.Desc.Label == "Dismiss" {
								footerTop = min(footerTop, node.Desc.Bounds.Min.Y)
								footerBottom = max(footerBottom, node.Desc.Bounds.Max.Y)
								footerFound = true
							}
						}
						if !footerFound {
							t.Fatal("the review footer is not reachable")
						}
						lastVisible = false
						rowIDs := make(map[input.SemanticID]string)
						for _, node := range nodes {
							label, bounds := node.Desc.Label, node.Desc.Bounds
							rowContent := strings.HasPrefix(label, "Did you mean to spell ") ||
								strings.Contains(label, " → ") || label == "Apply" || label == "Add to dictionary"
							if rowContent {
								rowIDs[node.ID] = label
							}
							if label == lastLabel && !bounds.Empty() && bounds.Max.Y <= footerTop {
								lastVisible = true
							}
						}
						for y := footerTop; y < footerBottom; y += 4 {
							for x := 0; x < frame.size.X; x += 4 {
								id, _ := frame.router.SemanticAt(f32.Pt(float32(x), float32(y)))
								if label, overlaps := rowIDs[id]; overlaps {
									t.Fatalf("issue row reaches the footer at %d,%d: %q", x, y, label)
								}
							}
						}
					}
					draw()
					for attempt := 0; attempt < 16 && (!lastVisible || state.ask.issueList.Position.BeforeEnd); attempt++ {
						at := f32.Pt(260, 70)
						frame.router.Queue(
							pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: at},
							pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: at, Scroll: f32.Pt(0, 240)},
						)
						draw()
						frame.now = frame.now.Add(time.Second)
						draw()
					}
					if !lastVisible || state.ask.issueList.Position.BeforeEnd || state.ask.issueList.Position.First == 0 {
						t.Fatalf("wheel did not reach the complete last issue: visible=%v position=%#v", lastVisible, state.ask.issueList.Position)
					}
				})
			}
		}
	}
}
