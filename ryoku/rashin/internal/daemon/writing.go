package daemon

import (
	"context"
	"sync"
	"time"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/writing"
)

type writingRequestKey struct {
	daemon *Daemon
	id     string
}

type writingRequest struct {
	cancel  context.CancelFunc
	expires time.Time
}

const writingRequestLimit = 1024
const writingCancelLifetime = 3 * time.Minute

var writingRequests struct {
	sync.Mutex
	active map[writingRequestKey]*writingRequest
}

func expireWritingRequests(now time.Time) {
	for key, request := range writingRequests.active {
		if request.cancel == nil && !request.expires.After(now) {
			delete(writingRequests.active, key)
		}
	}
}

func (d *Daemon) writingContext(ctx context.Context, request string) (context.Context, func(), error) {
	ctx, cancel := context.WithCancel(ctx)
	if request == "" {
		return ctx, cancel, nil
	}
	if len(request) > 512 {
		cancel()
		return nil, nil, protocol.Errorf(protocol.CodeBadRequest, "Keep the writing request id under 512 bytes.")
	}
	key := writingRequestKey{daemon: d, id: request}
	writingRequests.Lock()
	expireWritingRequests(time.Now())
	if previous := writingRequests.active[key]; previous != nil {
		writingRequests.Unlock()
		cancel()
		if previous.cancel == nil {
			return nil, nil, context.Canceled
		}
		return nil, nil, protocol.Errorf(protocol.CodeBadRequest, "This writing request id is already running.")
	}
	if len(writingRequests.active) >= writingRequestLimit {
		writingRequests.Unlock()
		cancel()
		return nil, nil, protocol.Errorf(protocol.CodeConflict, "Too many writing requests are waiting.")
	}
	if writingRequests.active == nil {
		writingRequests.active = make(map[writingRequestKey]*writingRequest)
	}
	entry := &writingRequest{cancel: cancel}
	writingRequests.active[key] = entry
	writingRequests.Unlock()
	return ctx, func() {
		cancel()
		writingRequests.Lock()
		if writingRequests.active[key] == entry {
			delete(writingRequests.active, key)
		}
		writingRequests.Unlock()
	}, nil
}

func (d *Daemon) writingCheck(ctx context.Context, c *connection, p protocol.WritingCheckParams) (protocol.WritingCheckResult, error) {
	ctx, finish, err := d.writingContext(ctx, p.Request)
	if err != nil {
		return protocol.WritingCheckResult{}, err
	}
	defer finish()
	return writing.Check(ctx, d.deps.Gateway, p)
}

func (d *Daemon) writingTransform(ctx context.Context, c *connection, p protocol.WritingTransformParams) (protocol.WritingTransformResult, error) {
	if p.Action == "enhance" && !d.feature("enhance") {
		return protocol.WritingTransformResult{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Turn on Improve my prompt in Settings first."))
	}
	ctx, finish, err := d.writingContext(ctx, p.Request)
	if err != nil {
		return protocol.WritingTransformResult{}, err
	}
	defer finish()
	return writing.Transform(ctx, d.deps.Gateway, p)
}

func (d *Daemon) writingCancel(ctx context.Context, c *connection, p protocol.WritingCancelParams) (protocol.Empty, error) {
	if p.Request == "" {
		return protocol.Empty{}, nil
	}
	if len(p.Request) > 512 {
		return protocol.Empty{}, protocol.Errorf(protocol.CodeBadRequest, "Keep the writing request id under 512 bytes.")
	}
	writingRequests.Lock()
	defer writingRequests.Unlock()
	now := time.Now()
	expireWritingRequests(now)
	key := writingRequestKey{daemon: d, id: p.Request}
	if entry := writingRequests.active[key]; entry != nil {
		if entry.cancel != nil {
			entry.cancel()
		}
		return protocol.Empty{}, nil
	}
	if len(writingRequests.active) >= writingRequestLimit {
		return protocol.Empty{}, protocol.Errorf(protocol.CodeConflict, "Too many writing requests are waiting.")
	}
	if writingRequests.active == nil {
		writingRequests.active = make(map[writingRequestKey]*writingRequest)
	}
	// hiding a writing surface can beat its request to the daemon
	writingRequests.active[key] = &writingRequest{expires: now.Add(writingCancelLifetime)}
	return protocol.Empty{}, nil
}

func (d *Daemon) writingVoiceGet(ctx context.Context, c *connection, p protocol.WritingVoiceGetParams) (protocol.WritingVoice, error) {
	return writing.VoiceGet()
}

func (d *Daemon) writingVoiceSet(ctx context.Context, c *connection, p protocol.WritingVoiceSetParams) (protocol.WritingVoice, error) {
	return writing.VoiceSet(p)
}

func (d *Daemon) writingVoiceLearn(ctx context.Context, c *connection, p protocol.WritingVoiceLearnParams) (protocol.WritingVoice, error) {
	ctx, finish, err := d.writingContext(ctx, p.Request)
	if err != nil {
		return protocol.WritingVoice{}, err
	}
	defer finish()
	return writing.VoiceLearn(ctx, d.deps.Gateway, p)
}

func (d *Daemon) writingWords(ctx context.Context, c *connection, p protocol.WritingWordsParams) (protocol.WritingWords, error) {
	return writing.WordsGet()
}

func (d *Daemon) writingWordsAdd(ctx context.Context, c *connection, p protocol.WritingWordParams) (protocol.WritingWords, error) {
	return writing.WordsAdd(ctx, p.Word)
}

func (d *Daemon) writingWordsRemove(ctx context.Context, c *connection, p protocol.WritingWordParams) (protocol.WritingWords, error) {
	return writing.WordsRemove(ctx, p.Word)
}
