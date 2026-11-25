package daemon

import (
	"crypto/rand"
	"encoding/hex"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/guide"
	"ryoku-rashin/internal/oni/keep"
	"ryoku-rashin/internal/protocol"
)

// Keep: a guide that finished with every step trusted is offered for
// keeping, on this computer, for the chip's time. Only oni.keep on the
// offer's id writes the store; a skip, the offer lapsing, a newer guide or
// any unwind writes nothing. Replays mark their entry used, missed or
// forgotten, as the guide machine says.

// guideOffer offers to keep the finished guide, for the chip's time.
func (d *Daemon) guideOffer(run *guideRun, entry keep.Entry) {
	id, err := oniOfferID()
	if err != nil {
		d.guideHandle(run, guide.KeepAnswer{})
		return
	}
	offer := &guideOffer{id: id, entry: entry}
	offer.timer = d.oniGuideClock().AfterFunc(feature.KeepOfferTTL(), func() {
		d.guideDo(func() {
			if run.offer == offer {
				d.guideAnswerOffer(run, false)
			}
		})
	})
	run.offer = offer
	if svc := d.oni.service.Load(); svc != nil {
		_ = svc.Consent(*feature.KeepOffer(id, oniKeepQuestion))
	}
}

// guideCloseOffer drops run's keep offer and says so on the frames.
func (d *Daemon) guideCloseOffer(run *guideRun, kept bool) {
	offer := run.offer
	if offer == nil {
		return
	}
	run.offer = nil
	offer.timer.Stop()
	answer := "no"
	if kept {
		answer = "yes"
	}
	if d.oni.service.Load() != nil {
		d.oniEmit(&feature.ConsentAnswerFrame{ID: offer.id, Answer: answer})
	}
}

// guideAnswerOffer answers the keep offer; only oni.keep keeps.
func (d *Daemon) guideAnswerOffer(run *guideRun, kept bool) {
	d.guideCloseOffer(run, kept)
	d.guideHandle(run, guide.KeepAnswer{Keep: kept})
}

func oniOfferID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "k-" + hex.EncodeToString(b[:]), nil
}

// oniKeepCall keeps the finished guide or the run of quick commands whose
// offer id names. Only the app, the CLI or a spoken yes from a valid press
// answer it.
func (d *Daemon) oniKeepCall(c *connection, id string) (protocol.OniKeepResult, error) {
	if err := d.oniCaller(c, false); err != nil {
		return protocol.OniKeepResult{}, err
	}
	return d.offerKeep(id)
}

// offerKeep keeps what the offer id names: a routine offer's run or the
// finished guide.
func (d *Daemon) offerKeep(id string) (protocol.OniKeepResult, error) {
	if isRoutineOffer(id) {
		return d.routineKeep(id)
	}
	return d.guideKeep(id)
}

// guideKeep writes the offered entry, the one write Keep makes.
func (d *Daemon) guideKeep(id string) (protocol.OniKeepResult, error) {
	if err := guideRefusal(protocol.EffectiveOni(d.Settings()), feature.GuideKeep); err != nil {
		return protocol.OniKeepResult{}, err
	}
	type kept struct {
		res protocol.OniKeepResult
		err error
	}
	ch := make(chan kept, 1)
	d.guideDo(func() {
		run := d.oni.guide.run
		if run == nil || run.offer == nil || run.offer.id != id {
			ch <- kept{err: oniOfferGone()}
			return
		}
		store := d.oniKeeps()
		if err := store.Put(run.offer.entry); err != nil {
			d.guideAnswerOffer(run, false)
			ch <- kept{err: err}
			return
		}
		n := 0
		if list, err := store.List(); err == nil {
			n = len(list)
		}
		d.guideAnswerOffer(run, true)
		ch <- kept{res: protocol.OniKeepResult{Kept: true, Entries: n}}
	})
	k := <-ch
	return k.res, k.err
}

// oniSkipCall lets the keep offer, the routine offer or the hint id go,
// writing nothing but a hint's snooze.
func (d *Daemon) oniSkipCall(c *connection, id string) error {
	if err := d.oniCaller(c, true); err != nil {
		return err
	}
	return d.offerSkip(id)
}

// offerSkip lets the offer id go: a routine offer, the keep offer or a
// hint.
func (d *Daemon) offerSkip(id string) error {
	if isRoutineOffer(id) {
		return d.routineSkip(id)
	}
	return d.guideSkip(id)
}

// guideSkip lets the keep offer id go, writing nothing: a skip from the
// card, the CLI or a spoken no from a valid press. On a hint's id it is
// the hint's Not now, which snoozes its app for the day.
func (d *Daemon) guideSkip(id string) error {
	ch := make(chan error, 1)
	d.guideDo(func() {
		if d.hintSkip(id) {
			ch <- nil
			return
		}
		run := d.oni.guide.run
		if run == nil || run.offer == nil || run.offer.id != id {
			ch <- oniOfferGone()
			return
		}
		d.guideAnswerOffer(run, false)
		ch <- nil
	})
	return <-ch
}

// guideOfferWaiting is the id of the keep offer waiting on an answer, or
// "".
func (d *Daemon) guideOfferWaiting() string {
	ch := make(chan string, 1)
	d.guideDo(func() {
		if run := d.oni.guide.run; run != nil && run.offer != nil {
			ch <- run.offer.id
			return
		}
		ch <- ""
	})
	return <-ch
}

func oniOfferGone() error {
	return protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That offer is not waiting any more; nothing was kept."))
}
