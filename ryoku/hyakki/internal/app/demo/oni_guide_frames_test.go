package demo

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/protocol"
)

// servedGuide serves the guide demo on a socket of its own and returns it
func servedGuide(t *testing.T) string {
	t.Helper()
	// a short directory keeps the socket path within the Unix limit
	dir, err := os.MkdirTemp("", "hyakki-demo-guide")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	socket := filepath.Join(dir, "oni.sock")
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, socket, FeatureOniGuide) }()
	t.Cleanup(func() {
		stop()
		if err := <-done; err != nil {
			t.Error(err)
		}
	})
	deadline := time.Now().Add(5 * time.Second)
	for !client.Reachable(context.Background(), socket) {
		if time.Now().After(deadline) {
			t.Fatal("the demo never listened")
		}
		time.Sleep(5 * time.Millisecond)
	}
	return socket
}

func dialDemo(t *testing.T, socket string) *client.Conn {
	t.Helper()
	conn, err := client.Dial(context.Background(), client.Options{Client: protocol.ClientApp, Socket: socket})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func demoCall(t *testing.T, conn *client.Conn, method string, params, result any) {
	t.Helper()
	if err := conn.Call(context.Background(), method, params, result); err != nil {
		t.Fatalf("%s: %v", method, err)
	}
}

// heard is the next oni frame the watcher hears, decoded strictly
func heard(t *testing.T, conn *client.Conn) feature.Payload {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case event := <-conn.Events():
			if event.Event != protocol.EvtOni {
				continue
			}
			payload, err := feature.DecodeFrame(event.Data)
			if err != nil {
				t.Fatalf("the demo sent a bad frame %s: %v", event.Data, err)
			}
			return payload
		case <-timeout:
			t.Fatal("no oni frame came")
		}
	}
}

func heardGuide(t *testing.T, conn *client.Conn, phase string) *feature.GuideFrame {
	t.Helper()
	f, ok := heard(t, conn).(*feature.GuideFrame)
	if !ok || f.Phase != phase {
		t.Fatalf("heard %+v, want a %s guide frame", f, phase)
	}
	return f
}

// A connection that opened the oni event as an observer hears the guide's
// frames as another connection walks it: reading while its picture waits,
// each step with its checklist, the end, the keep offer and its answer,
// then idle; one that opens mid-guide starts from the step, and one that
// closed hears nothing more.
func TestDemoObserverHearsTheGuideFrames(t *testing.T) {
	socket := servedGuide(t)
	watcher, actor := dialDemo(t, socket), dialDemo(t, socket)
	var opened protocol.OniOpenResult
	demoCall(t, watcher, protocol.MOniOpen, protocol.OniOpenParams{Renderer: protocol.OniRendererObserver}, &opened)
	if opened.Presenter || opened.Reason != protocol.OniOpenObserver || len(opened.Frames) != 0 {
		t.Fatalf("opening with no guide = %+v", opened)
	}

	var result protocol.OniAskResult
	demoCall(t, actor, protocol.MOniGuide, protocol.OniGuideParams{Action: protocol.OniGuideStart, Goal: "save this file"}, &result)
	if reading := heardGuide(t, watcher, feature.GuideReading); reading.Step != 1 || reading.Text != "" {
		t.Fatalf("reading = %+v", reading)
	}
	demoCall(t, actor, protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: result.Consent.ID, Answer: "yes"}, &result)
	step1 := heardGuide(t, watcher, feature.GuideWaiting)
	if step1.Step != 1 || step1.Kicker != "Step 1" || step1.Text != result.Answer.Text || step1.Target != result.Answer.Seq ||
		step1.Plan == nil || step1.Plan.Current != step1.Text || len(step1.Plan.Next) != 1 || len(step1.Plan.Done) != 0 {
		t.Fatalf("step 1 = %+v", step1)
	}

	late := dialDemo(t, socket)
	demoCall(t, late, protocol.MOniOpen, protocol.OniOpenParams{Renderer: protocol.OniRendererObserver}, &opened)
	if len(opened.Frames) != 1 {
		t.Fatalf("opening mid-guide gave %d frames", len(opened.Frames))
	}
	if f, err := feature.DecodeFrame(opened.Frames[0]); err != nil || f.(*feature.GuideFrame).Seq != step1.Seq {
		t.Fatalf("opening mid-guide starts from %v: %v", f, err)
	}
	demoCall(t, late, protocol.MOniClose, protocol.Empty{}, nil)

	demoCall(t, actor, protocol.MOniGuide, protocol.OniGuideParams{Action: protocol.OniGuideNext}, &result)
	if step2 := heardGuide(t, watcher, feature.GuideWaiting); step2.Step != 2 || len(step2.Plan.Done) != 1 || step2.Plan.Next != nil || step2.Seq <= step1.Seq {
		t.Fatalf("step 2 = %+v", step2)
	}
	demoCall(t, actor, protocol.MOniGuide, protocol.OniGuideParams{Action: protocol.OniGuideNext}, &result)
	if done := heardGuide(t, watcher, feature.GuideDone); done.Text != result.Answer.Text {
		t.Fatalf("done = %+v", done)
	}
	heardGuide(t, watcher, feature.GuideKeeping)
	offer, ok := heard(t, watcher).(*feature.ConsentFrame)
	if !ok || offer.ID != result.Keep.ID || offer.Feature != feature.GuideKeep || offer.Destination != feature.EgressLocal {
		t.Fatalf("the keep offer = %+v", offer)
	}
	demoCall(t, actor, protocol.MOniKeep, protocol.OniKeepParams{ID: offer.ID}, nil)
	if answered, ok := heard(t, watcher).(*feature.ConsentAnswerFrame); !ok || answered.ID != offer.ID || answered.Answer != "yes" {
		t.Fatalf("the offer's answer = %+v", answered)
	}
	heardGuide(t, watcher, feature.GuideIdle)

	// the late watcher closed: it hears nothing of the next guide
	demoCall(t, actor, protocol.MOniGuide, protocol.OniGuideParams{Action: protocol.OniGuideStart, Goal: "save this file"}, &result)
	if replay := heardGuide(t, watcher, feature.GuideWaiting); !replay.Replay || replay.Kicker != "Kept from before" {
		t.Fatalf("the replay = %+v", replay)
	}
	select {
	case event := <-late.Events():
		t.Fatalf("a closed watcher heard %s", event.Data)
	case <-time.After(50 * time.Millisecond):
	}
}
