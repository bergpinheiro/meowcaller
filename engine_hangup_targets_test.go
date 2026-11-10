package meowcaller

import (
	"context"
	"errors"
	"testing"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

func plainTerminateTargets(nodes []waBinary.Node) map[types.JID]int {
	targets := map[types.JID]int{}
	for _, node := range nodes {
		for _, child := range node.GetChildren() {
			if child.Tag == "terminate" && child.AttrGetter().OptionalString("reason") == "" {
				targets[node.AttrGetter().JID("to")]++
			}
		}
	}
	return targets
}

func ringingDevices() (phone, web, companion types.JID) {
	phone = types.JID{User: "222222222222222", Server: types.HiddenUserServer}
	web = types.JID{User: "222222222222222", Server: types.HiddenUserServer, Device: 28}
	companion = types.JID{User: "222222222222222", Server: types.HiddenUserServer, Device: 20}
	return
}

func TestHangupWhileRingingTerminatesEveryRungDevice(t *testing.T) {
	eng, call := testEngineWithOutgoingCall()
	var sent []waBinary.Node
	eng.sendCallNode = func(_ context.Context, node waBinary.Node) error {
		sent = append(sent, node)
		return nil
	}
	phone, web, companion := ringingDevices()
	eng.calls["CID"].peerDevices = []types.JID{phone, web, companion}

	if err := call.Hangup(); err != nil {
		t.Fatalf("Hangup: %v", err)
	}

	targets := plainTerminateTargets(sent)
	if len(sent) != 3 || len(targets) != 3 || targets[phone] != 1 || targets[web] != 1 || targets[companion] != 1 {
		t.Fatalf("terminate targets = %v (%d stanzas), want one each to %s, %s and %s", targets, len(sent), phone, web, companion)
	}
	for _, node := range sent {
		if got := node.GetChildren()[0].AttrGetter().JID("call-creator"); got != creatorJID() {
			t.Fatalf("terminate call-creator = %s, want %s", got, creatorJID())
		}
	}
	if call.State() != CallPhaseEnded || eng.lookup(call.ID()) != nil {
		t.Fatal("Hangup did not end the call locally")
	}
}

func TestHangupAfterAcceptTerminatesOnlyTheAnsweringDevice(t *testing.T) {
	eng, call := testEngineWithOutgoingCall()
	var sent []waBinary.Node
	eng.sendCallNode = func(_ context.Context, node waBinary.Node) error {
		sent = append(sent, node)
		return nil
	}
	phone, web, companion := ringingDevices()
	eng.calls["CID"].peerDevices = []types.JID{phone, web, companion}
	eng.onAccept(&events.CallAccept{
		BasicCallMeta: types.BasicCallMeta{CallID: "CID", From: companion},
		Data:          &waBinary.Node{Tag: "accept"},
	})
	sent = nil

	if err := call.Hangup(); err != nil {
		t.Fatalf("Hangup: %v", err)
	}

	targets := plainTerminateTargets(sent)
	if len(sent) != 1 || targets[companion] != 1 {
		t.Fatalf("terminate targets = %v (%d stanzas), want only %s", targets, len(sent), companion)
	}
}

func TestHangupWhileRingingSucceedsWhenSomeDeviceIsNotified(t *testing.T) {
	eng, call := testEngineWithOutgoingCall()
	phone, web, companion := ringingDevices()
	eng.calls["CID"].peerDevices = []types.JID{phone, web, companion}
	var attempts int
	eng.sendCallNode = func(_ context.Context, node waBinary.Node) error {
		attempts++
		if node.AttrGetter().JID("to") == web {
			return errors.New("network unavailable")
		}
		return nil
	}

	if err := call.Hangup(); err != nil {
		t.Fatalf("Hangup returned %v although two of three devices were notified", err)
	}
	if attempts != 3 {
		t.Fatalf("terminate attempts = %d, want 3 (a failed device must not stop the rest)", attempts)
	}
}

func TestHangupWhileRingingFailsWhenNoDeviceIsNotified(t *testing.T) {
	eng, call := testEngineWithOutgoingCall()
	phone, web, _ := ringingDevices()
	eng.calls["CID"].peerDevices = []types.JID{phone, web}
	eng.sendCallNode = func(context.Context, waBinary.Node) error {
		return errors.New("network unavailable")
	}

	if err := call.Hangup(); err == nil {
		t.Fatal("Hangup returned nil although no device was notified")
	}
	if call.State() != CallPhaseEnded || eng.lookup(call.ID()) != nil {
		t.Fatal("Hangup did not end the call locally after signaling failed")
	}
}
