package meowcaller

import (
	"context"

	"github.com/rs/zerolog"
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
