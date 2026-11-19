package services

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"

	"ryoget/internal/hls"
	"ryoget/internal/media"
	"ryoget/internal/plan"

	"ryoku-i18n"
)

var redditVideoBase = "https://v.redd.it"

// reddit serves its videos keyless from v.redd.it; post pages and their JSON
// now answer anonymous clients with a block page, so only video links resolve
// here and post links go on to the generic chain.
type reddit struct{}

func init() { media.Register(reddit{}) }

func (reddit) ID() string   { return "reddit" }
func (reddit) Name() string { return "Reddit" }

func (reddit) Match(u *url.URL) bool { return media.HostIs(u.Hostname(), "v.redd.it") }

var vredditIDRe = regexp.MustCompile(`^/([A-Za-z0-9]{8,})`)

func (s reddit) Resolve(ctx context.Context, u *url.URL, env *plan.Env) (*media.Media, error) {
	m := vredditIDRe.FindStringSubmatch(u.Path)
	if m == nil {
		return nil, fmt.Errorf("%w: %s", media.ErrUnsupported, i18n.T("Only links to a single video can be downloaded."))
	}
	id := m[1]
	vs, err := hls.Variants(ctx, env, redditVideoBase+"/"+id+"/HLSPlaylist.m3u8", nil)
	if err != nil {
		if errors.Is(err, media.ErrNetwork) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %v", media.ErrUnavailable, err)
	}
	out := &media.Media{Kind: plan.KindMedia, URL: redditVideoBase + "/" + id, Source: media.Source{ID: s.ID(), Name: s.Name()},
		Title: i18n.Tf("Reddit video %s", id), Formats: media.HLSFormats(vs, nil)}
	if len(out.Formats) == 0 {
		return nil, fmt.Errorf("%w: %s", media.ErrUnavailable, i18n.T("The stream has no playable variants."))
	}
	out.Default = best(out.Formats)
	return out, nil
}
