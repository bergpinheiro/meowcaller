package meowcaller

import (
	"context"
	"testing"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

func testEngineAnsweringCall() (*engine, *Call, types.JID, *[]waBinary.Node) {
	eng := testEngineWithSignalStore()
	sent := &[]waBinary.Node{}
	eng.sendCallNode = func(_ context.Context, node waBinary.Node) error {
		*sent = append(*sent, node)
		return nil
	}
	caller := types.JID{User: "100", Device: 7, Server: types.HiddenUserServer}
	call := &Call{eng: eng, id: "CID", peer: caller, phase: CallPhaseRinging}
	eng.calls["CID"] = &engineCall{
		call:      call,
		direction: CallDirectionIncoming,
		from:      caller,
		creator:   caller,
	}
	return eng, call, caller, sent
}

func callerMuteV2(caller types.JID, muteState string) *waBinary.Node {
	return &waBinary.Node{
		Tag:   "call",
		Attrs: waBinary.Attrs{"from": caller, "id": "MUTE"},
		Content: []waBinary.Node{{
			Tag:   "mute_v2",
			Attrs: waBinary.Attrs{"call-id": "CID", "call-creator": caller, "mute-state": muteState},
		}},
	}
}

func acceptsIn(nodes []waBinary.Node) int {
	count := 0
	for _, node := range nodes {
		for _, child := range node.GetChildren() {
			if child.Tag == "accept" {
				count++
			}
		}
	}
	return count
}

func TestAnswerSendsAcceptToTheOfferingDevice(t *testing.T) {
	eng, call, caller, sent := testEngineAnsweringCall()

	if err := eng.answer(call); err != nil {
		t.Fatalf("Answer: %v", err)
	}
	if len(*sent) != 1 || acceptsIn(*sent) != 1 {
		t.Fatalf("sent %d nodes with %d accepts, want 1 accept", len(*sent), acceptsIn(*sent))
	}
	wrapper := (*sent)[0]
	if wrapper.AttrGetter().JID("to") != caller {
		t.Fatalf("accept target = %s, want the offering device %s", wrapper.AttrGetter().JID("to"), caller)
	}
	attrs := wrapper.GetChildren()[0].AttrGetter()
	if attrs.String("call-id") != "CID" || attrs.JID("call-creator") != caller {
		t.Fatalf("accept identity = %#v", wrapper.GetChildren()[0].Attrs)
	}
	if call.State() != CallPhaseConnecting {
		t.Fatalf("phase = %d, want connecting", call.State())
	}
}

func TestMuteV2WhileRingingDoesNotAccept(t *testing.T) {
	eng, call, caller, sent := testEngineAnsweringCall()

	eng.onCallRaw(callerMuteV2(caller, "0"))

	if acceptsIn(*sent) != 0 {
		t.Fatalf("mute_v2 before Answer sent %d accepts", acceptsIn(*sent))
	}
	if call.State() != CallPhaseRinging {
		t.Fatalf("phase = %d, want ringing", call.State())
	}
}

func TestAnswerAcceptsOnceWhateverTheCallerSends(t *testing.T) {
	eng, call, caller, sent := testEngineAnsweringCall()
	var muteStates []bool
	call.OnMuteState(func(muted bool) { muteStates = append(muteStates, muted) })

	if err := eng.answer(call); err != nil {
		t.Fatalf("Answer: %v", err)
	}
	eng.onCallRaw(callerMuteV2(caller, "0"))
	eng.onCallRaw(callerMuteV2(caller, "1"))
	if err := eng.answer(call); err != nil {
		t.Fatalf("second Answer: %v", err)
	}

	if acceptsIn(*sent) != 1 {
		t.Fatalf("sent %d accepts, want exactly 1", acceptsIn(*sent))
	}
	if len(muteStates) != 2 || muteStates[0] || !muteStates[1] {
		t.Fatalf("mute states = %v, want [false true]", muteStates)
	}
}
