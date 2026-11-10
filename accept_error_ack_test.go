package meowcaller

import (
	"testing"
	"time"

	waBinary "go.mau.fi/whatsmeow/binary"
)

func TestAudioSinceErrorAck(t *testing.T) {
	c := &Call{}
	ack := time.Now()
	if audioSince(c, ack) {
		t.Fatal("no audio yet: must report none")
	}
	c.lastInboundAudio.Store(ack.Add(-time.Second).UnixNano())
	if audioSince(c, ack) {
		t.Fatal("audio before the ack (early media) must not count")
	}
	c.lastInboundAudio.Store(ack.Add(200 * time.Millisecond).UnixNano())
	if !audioSince(c, ack) {
		t.Fatal("audio after the ack: the accept landed")
	}
}

func acceptErrorAck() *waBinary.Node {
	return &waBinary.Node{
		Tag:   "ack",
		Attrs: waBinary.Attrs{"class": "call", "type": "accept", "error": "500"},
		Content: []waBinary.Node{{
			Tag:   "error",
			Attrs: waBinary.Attrs{"call-id": "CID"},
		}},
	}
}

func testEngineWithAnsweredIncomingCall(t *testing.T) (*engine, *Call) {
	t.Helper()
	grace := acceptErrorAckGrace
	acceptErrorAckGrace = 30 * time.Millisecond
	t.Cleanup(func() { acceptErrorAckGrace = grace })
	eng, call := testEngineWithIncomingCall()
	eng.calls["CID"].acceptSent = true
	call.setPhase(CallPhaseConnecting)
	return eng, call
}

func TestAcceptErrorAckKeepsTheAnsweredCallWhileAudioFlows(t *testing.T) {
	eng, call := testEngineWithAnsweredIncomingCall(t)

	eng.onCallAck(acceptErrorAck())
	if eng.lookup("CID") == nil {
		t.Fatal("an error ack after our accept ended the answered call at once")
	}
	call.lastInboundAudio.Store(time.Now().Add(time.Millisecond).UnixNano())
	time.Sleep(4 * acceptErrorAckGrace)

	if eng.lookup("CID") == nil {
		t.Fatal("the call was ended although the peer's audio kept arriving")
	}
}

func TestAcceptErrorAckEndsTheCallWhenNoAudioFollows(t *testing.T) {
	eng, _ := testEngineWithAnsweredIncomingCall(t)

	eng.onCallAck(acceptErrorAck())
	time.Sleep(4 * acceptErrorAckGrace)

	if eng.lookup("CID") != nil {
		t.Fatal("the call was kept although no audio arrived after the error ack")
	}
}

func TestErrorAckBeforeAnswerStillEndsTheCall(t *testing.T) {
	eng, _ := testEngineWithIncomingCall()

	eng.onCallAck(acceptErrorAck())

	if eng.lookup("CID") != nil {
		t.Fatal("an error ack on a call we never answered did not end it")
	}
}
