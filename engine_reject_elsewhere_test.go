package meowcaller

import (
	"context"
	"errors"
	"testing"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

func terminateTargetsWithReason(nodes []waBinary.Node, reason string) map[types.JID]int {
	targets := map[types.JID]int{}
	for _, node := range nodes {
		for _, child := range node.GetChildren() {
			if child.Tag == "terminate" && child.AttrGetter().OptionalString("reason") == reason {
				targets[node.AttrGetter().JID("to")]++
			}
		}
	}
	return targets
}

func peerReject(from types.JID, reason string) *events.CallReject {
	data := &waBinary.Node{Tag: "reject", Attrs: waBinary.Attrs{}}
	if reason != "" {
		data.Attrs["reason"] = reason
	}
	return &events.CallReject{BasicCallMeta: types.BasicCallMeta{CallID: "CID", From: from}, Data: data}
}

func TestPeerRejectTellsThePeersOtherDevicesTheCallWasRejectedElsewhere(t *testing.T) {
	eng, call := testEngineWithOutgoingCall()
	var sent []waBinary.Node
	eng.sendCallNode = func(_ context.Context, node waBinary.Node) error {
		sent = append(sent, node)
		return nil
	}
	phone, web, companion := ringingDevices()
	eng.calls["CID"].peerDevices = []types.JID{phone, web, companion}
	var reason string
	call.OnEnd(func(r string) { reason = r })

	eng.onReject(peerReject(companion, ""))

	targets := terminateTargetsWithReason(sent, "rejected_elsewhere")
	if len(sent) != 2 || targets[phone] != 1 || targets[web] != 1 {
		t.Fatalf("rejected_elsewhere targets = %v (%d stanzas), want one each to %s and %s", targets, len(sent), phone, web)
	}
	for _, node := range sent {
		if got := node.GetChildren()[0].AttrGetter().JID("call-creator"); got != creatorJID() {
			t.Fatalf("terminate call-creator = %s, want %s", got, creatorJID())
		}
	}
	if call.State() != CallPhaseEnded || reason != "rejected" {
		t.Fatalf("after reject: phase = %d, reason = %q, want Ended/rejected", call.State(), reason)
	}
}

func TestBusyRejectDoesNotDismissThePeersOtherDevices(t *testing.T) {
	for _, busy := range []string{"busy", "enc"} {
		eng, _ := testEngineWithOutgoingCall()
		var sent []waBinary.Node
		eng.sendCallNode = func(_ context.Context, node waBinary.Node) error {
			sent = append(sent, node)
			return nil
		}
		phone, web, companion := ringingDevices()
		eng.calls["CID"].peerDevices = []types.JID{phone, web, companion}

		eng.onReject(peerReject(companion, busy))

		if len(sent) != 0 {
			t.Fatalf("reject reason %q sent %d stanzas, want none: that device decided nothing for the peer", busy, len(sent))
		}
	}
}

func TestPeerRejectAfterAcceptDismissesNothing(t *testing.T) {
	eng, _ := testEngineWithOutgoingCall()
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

	eng.onReject(peerReject(phone, ""))

	if targets := terminateTargetsWithReason(sent, "rejected_elsewhere"); len(targets) != 0 {
		t.Fatalf("rejected_elsewhere sent to %v after the call was already accepted", targets)
	}
}

func TestPeerRejectWithoutDataStillDismissesThePeersOtherDevices(t *testing.T) {
	eng, _ := testEngineWithOutgoingCall()
	var sent []waBinary.Node
	eng.sendCallNode = func(_ context.Context, node waBinary.Node) error {
		sent = append(sent, node)
		return nil
	}
	phone, web, companion := ringingDevices()
	eng.calls["CID"].peerDevices = []types.JID{phone, web, companion}

	eng.onReject(&events.CallReject{BasicCallMeta: types.BasicCallMeta{CallID: "CID", From: companion}})

	if targets := terminateTargetsWithReason(sent, "rejected_elsewhere"); len(targets) != 2 {
		t.Fatalf("rejected_elsewhere targets = %v, want the two other devices", targets)
	}
}

func TestPeerRejectMatchesTheRejectingDeviceWhateverItsAgent(t *testing.T) {
	eng, _ := testEngineWithOutgoingCall()
	var sent []waBinary.Node
	eng.sendCallNode = func(_ context.Context, node waBinary.Node) error {
		sent = append(sent, node)
		return nil
	}
	phone, web, companion := ringingDevices()
	eng.calls["CID"].peerDevices = []types.JID{phone, web, companion}
	rejecting := companion
	rejecting.RawAgent = 1

	eng.onReject(peerReject(rejecting, ""))

	targets := terminateTargetsWithReason(sent, "rejected_elsewhere")
	if len(sent) != 2 || targets[phone] != 1 || targets[web] != 1 {
		t.Fatalf("rejected_elsewhere targets = %v (%d stanzas), want only %s and %s", targets, len(sent), phone, web)
	}
}

func TestPeerRejectOnGroupCallDismissesNothing(t *testing.T) {
	eng, _ := testEngineWithOutgoingCall()
	var sent []waBinary.Node
	eng.sendCallNode = func(_ context.Context, node waBinary.Node) error {
		sent = append(sent, node)
		return nil
	}
	phone, web, companion := ringingDevices()
	eng.calls["CID"].peerDevices = []types.JID{phone, web, companion}
	eng.calls["CID"].group = true

	eng.onReject(peerReject(companion, ""))

	if len(sent) != 0 {
		t.Fatalf("a reject on a group call sent %d stanzas, want none", len(sent))
	}
}

func TestHangupOnGroupCallGoesToTheCallScope(t *testing.T) {
	eng, call := testEngineWithOutgoingCall()
	var sent []waBinary.Node
	eng.sendCallNode = func(_ context.Context, node waBinary.Node) error {
		sent = append(sent, node)
		return nil
	}
	phone, web, companion := ringingDevices()
	callScope := types.NewJID("CID", "call")
	eng.calls["CID"].peerDevices = []types.JID{phone, web, companion}
	eng.calls["CID"].group = true
	eng.calls["CID"].from = callScope

	if err := call.Hangup(); err != nil {
		t.Fatalf("Hangup: %v", err)
	}

	if targets := plainTerminateTargets(sent); len(sent) != 1 || targets[callScope] != 1 {
		t.Fatalf("terminate targets = %v (%d stanzas), want only the call scope %s", targets, len(sent), callScope)
	}
}

func TestRejectAddressesTheDeviceTheOfferCameFrom(t *testing.T) {
	eng, call := testEngineWithIncomingCall()
	offerDevice := types.JID{User: "222222222222222", Server: types.HiddenUserServer, Device: 85}
	callCreator := types.JID{User: "222222222222222", Server: types.HiddenUserServer, Device: 7}
	eng.calls["CID"].from = offerDevice
	eng.calls["CID"].creator = callCreator
	var sent []waBinary.Node
	eng.sendCallNode = func(_ context.Context, node waBinary.Node) error {
		sent = append(sent, node)
		return nil
	}

	if err := call.Reject(); err != nil {
		t.Fatalf("Reject: %v", err)
	}

	if len(sent) != 1 {
		t.Fatalf("sent %d stanzas, want one reject", len(sent))
	}
	reject := sent[0].GetChildren()[0]
	attrs := reject.AttrGetter()
	if to := sent[0].AttrGetter().JID("to"); reject.Tag != "reject" || to != offerDevice {
		t.Fatalf("sent <%s> to %s, want <reject> to the offering device %s", reject.Tag, to, offerDevice)
	}
	if attrs.JID("call-creator") != callCreator || attrs.String("call-id") != "CID" || attrs.String("count") != "0" {
		t.Fatalf("reject attrs = %#v, want call-id CID, call-creator %s, count 0", reject.Attrs, callCreator)
	}
	if call.State() != CallPhaseEnded || eng.lookup("CID") != nil {
		t.Fatal("Reject did not end the call locally")
	}
}

func TestRejectOfAnEndedCallSendsNothing(t *testing.T) {
	eng, call := testEngineWithIncomingCall()
	var sent []waBinary.Node
	eng.sendCallNode = func(_ context.Context, node waBinary.Node) error {
		sent = append(sent, node)
		return nil
	}
	eng.finishCall("CID", "remote_terminate")

	err := call.Reject()

	if !errors.Is(err, ErrUnknownCall) {
		t.Fatalf("Reject of an ended call = %v, want ErrUnknownCall", err)
	}
	if len(sent) != 0 {
		t.Fatalf("Reject of an ended call sent %d stanzas, want none", len(sent))
	}
}
