package meowcaller

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"go.mau.fi/libsignal/ecc"
	"go.mau.fi/libsignal/keys/identity"
	"go.mau.fi/libsignal/keys/prekey"
	"go.mau.fi/libsignal/util/optional"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/util/keys"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// memSignalStore keeps a test device's Signal identities and sessions in memory.
type memSignalStore struct {
	sessions map[string][]byte
}

func (s *memSignalStore) PutIdentity(context.Context, string, [32]byte) error { return nil }
func (s *memSignalStore) DeleteAllIdentities(context.Context, string) error   { return nil }
func (s *memSignalStore) DeleteIdentity(context.Context, string) error        { return nil }
func (s *memSignalStore) IsTrustedIdentity(context.Context, string, [32]byte) (bool, error) {
	return true, nil
}

func (s *memSignalStore) GetSession(_ context.Context, address string) ([]byte, error) {
	return s.sessions[address], nil
}

func (s *memSignalStore) HasSession(_ context.Context, address string) (bool, error) {
	_, ok := s.sessions[address]
	return ok, nil
}

func (s *memSignalStore) GetManySessions(_ context.Context, addresses []string) (map[string][]byte, error) {
	out := make(map[string][]byte, len(addresses))
	for _, address := range addresses {
		out[address] = s.sessions[address]
	}
	return out, nil
}

func (s *memSignalStore) PutSession(_ context.Context, address string, session []byte) error {
	s.sessions[address] = session
	return nil
}

func (s *memSignalStore) PutManySessions(_ context.Context, sessions map[string][]byte) error {
	for address, session := range sessions {
		s.sessions[address] = session
	}
	return nil
}

func (s *memSignalStore) DeleteAllSessions(context.Context, string) error { return nil }

func (s *memSignalStore) DeleteSession(_ context.Context, address string) error {
	delete(s.sessions, address)
	return nil
}

func (s *memSignalStore) MigratePNToLID(context.Context, types.JID, types.JID) error { return nil }

func testEngineWithSignalStore() *engine {
	mem := &memSignalStore{sessions: map[string][]byte{}}
	device := &store.Device{
		Log: waLog.Noop, IdentityKey: keys.NewKeyPair(), RegistrationID: 1,
		Identities: mem, Sessions: mem,
	}
	c := &Client{wa: whatsmeow.NewClient(device, waLog.Noop), log: zerolog.Nop()}
	c.eng = newEngine(c)
	return c.eng
}

// peerPreKeyBundle is what a prekey fetch returns for a device that has not talked to us yet.
func peerPreKeyBundle(device uint16) *prekey.Bundle {
	identityKey := keys.NewKeyPair()
	signed := identityKey.CreateSignedPreKey(1)
	oneTime := keys.NewPreKey(2)
	return prekey.NewBundle(1, uint32(device),
		optional.NewOptionalUint32(oneTime.KeyID), signed.KeyID,
		ecc.NewDjbECPublicKey(*oneTime.Pub), ecc.NewDjbECPublicKey(*signed.Pub), *signed.Signature,
		identity.NewKey(ecc.NewDjbECPublicKey(*identityKey.Pub)))
}

func TestAcceptCallKeyEncWithoutSessionAcceptsWithoutIt(t *testing.T) {
	eng := testEngineWithSignalStore()
	if dk := eng.acceptCallKeyEnc("CID", peerJID(), make([]byte, 32)); dk != nil {
		t.Fatalf("accept enc = %+v, want none without a session", dk)
	}
}

func TestAcceptCallKeyEncRefusesASessionThatWouldSendPkmsg(t *testing.T) {
	eng := testEngineWithSignalStore()
	peer := peerJID()
	// A session we started from the peer's prekeys and the peer never answered still sends pkmsg
	_, _, err := eng.c.wa.DangerousInternals().EncryptMessageForDevice(
		context.Background(), []byte("hi"), peer, peerPreKeyBundle(peer.Device), nil, nil)
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	if dk := eng.acceptCallKeyEnc("CID", peer, make([]byte, 32)); dk != nil {
		t.Fatalf("accept enc = %s, want none: a pkmsg needs a device identity the accept lacks", dk.EncType)
	}
}
