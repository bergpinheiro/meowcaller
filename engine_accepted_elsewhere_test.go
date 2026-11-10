package meowcaller

import (
	"context"
	"testing"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

func acceptedElsewhereTargets(nodes []waBinary.Node) map[types.JID]int {
	targets := map[types.JID]int{}
	for _, node := range nodes {
		for _, child := range node.GetChildren() {
			if child.Tag == "terminate" && child.AttrGetter().String("reason") == "accepted_elsewhere" {
				targets[node.AttrGetter().JID("to")]++
			}
		}
	}
	return targets
}

func TestAcceptTellsThePeersOtherDevicesTheCallWasAcceptedElsewhere(t *testing.T) {
	eng, _ := testEngineWithOutgoingCall()
	var sent []waBinary.Node
	eng.sendCallNode = func(_ context.Context, node waBinary.Node) error {
		sent = append(sent, node)
		return nil
	}
	phone := types.JID{User: "222222222222222", Server: types.HiddenUserServer}
	web := types.JID{User: "222222222222222", Server: types.HiddenUserServer, Device: 1}
	answering := types.JID{User: "222222222222222", Server: types.HiddenUserServer, Device: 21}
	eng.calls["CID"].peerDevices = []types.JID{phone, web, answering}
	accept := &events.CallAccept{
		BasicCallMeta: types.BasicCallMeta{CallID: "CID", From: answering},
		Data:          &waBinary.Node{Tag: "accept"},
	}

	eng.onAccept(accept)
	eng.onAccept(accept)

	targets := acceptedElsewhereTargets(sent)
	if len(targets) != 2 || targets[phone] != 1 || targets[web] != 1 {
		t.Fatalf("accepted_elsewhere targets = %v, want one each to %s and %s", targets, phone, web)
	}
	if targets[answering] != 0 {
		t.Fatalf("the answering device %s was told the call was accepted elsewhere", answering)
	}
	for _, node := range sent {
		if node.GetChildren()[0].AttrGetter().JID("call-creator") != creatorJID() {
			t.Fatalf("terminate call-creator = %s, want %s", node.GetChildren()[0].AttrGetter().JID("call-creator"), creatorJID())
		}
	}
}

func TestAcceptWithoutOfferedDevicesSendsNoAcceptedElsewhere(t *testing.T) {
	eng, _ := testEngineWithOutgoingCall()
	var sent []waBinary.Node
	eng.sendCallNode = func(_ context.Context, node waBinary.Node) error {
		sent = append(sent, node)
		return nil
	}

	eng.onAccept(&events.CallAccept{
		BasicCallMeta: types.BasicCallMeta{CallID: "CID", From: peerJID()},
		Data:          &waBinary.Node{Tag: "accept"},
	})

	if targets := acceptedElsewhereTargets(sent); len(targets) != 0 {
		t.Fatalf("accepted_elsewhere sent to %v with no offered devices", targets)
	}
}
