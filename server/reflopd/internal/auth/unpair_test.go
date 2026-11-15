// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"strings"
	"testing"
	"time"
)

// Issue #6: once paired, a Guest could never pair again. Unpairing on both
// sides (the Guest's `reflop unpair` revokes, the Owner deletes) must make
// the same machine key pair like the first time (ADR-025).
func TestUnpairBothSidesThenPairAgain(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	nonce, fp := make([]byte, 32), make([]byte, 32)
	rand.Read(nonce)
	rand.Read(fp)
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	inv, _ := s.CreateInvite("alice", time.Hour)
	first, err := s.Redeem(pairReq(t, inv, priv, nonce, fp), nonce, fp)
	if err != nil {
		t.Fatal(err)
	}

	// The Guest forgot the server without telling it: pairing again names
	// the identity and the Owner's command, and the invite is used up.
	inv2, _ := s.CreateInvite("alice", time.Hour)
	_, err = s.Redeem(pairReq(t, inv2, priv, nonce, fp), nonce, fp)
	var kr *KeyRegisteredError
	if !errors.As(err, &kr) || kr.Name != "alice" || !strings.Contains(err.Error(), "reflopd unpair alice") {
		t.Fatalf("pairing a registered key: %v", err)
	}

	// The Guest unpairs: the key stops working at once, and it is marked.
	removed, err := s.GuestUnpair(first.ID)
	if err != nil || removed.Name != "alice" || len(removed.Keys) != 1 {
		t.Fatalf("GuestUnpair: %+v %v", removed, err)
	}
	if _, err := s.Authenticate(authReq(priv, nonce, fp), nonce, fp, ""); err == nil {
		t.Fatal("key still accepted after the Guest unpaired")
	}
	got, _ := s.Get("alice")
	if !got.Revoked || got.GuestUnpaired == nil || len(got.Keys) != 0 {
		t.Fatalf("identity after GuestUnpair: %+v", got)
	}
	if _, err := s.GuestUnpair(first.ID); !errors.Is(err, ErrAuth) {
		t.Fatalf("second GuestUnpair: %v", err)
	}
	if _, err := s.GuestUnpair("00000000000000000000000000000000"); !errors.Is(err, ErrAuth) {
		t.Fatalf("GuestUnpair of an unknown id: %v", err)
	}
	// The name stays taken until the Owner deletes it, and says how.
	if _, err := s.CreateInvite("alice", time.Hour); err == nil || !strings.Contains(err.Error(), "reflopd unpair alice") {
		t.Fatalf("invite for an unpaired name: %v", err)
	}

	// The Owner deletes it; the same machine key pairs again, as new.
	if _, err := s.Delete("alice"); err != nil {
		t.Fatal(err)
	}
	inv3, err := s.CreateInvite("alice", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.Redeem(pairReq(t, inv3, priv, nonce, fp), nonce, fp)
	if err != nil {
		t.Fatalf("pairing again after unpairing on both sides: %v", err)
	}
	if again.ID == first.ID || again.GuestUnpaired != nil || again.Revoked {
		t.Fatalf("re-paired identity carries the old state: %+v", again)
	}
	if _, err := s.Authenticate(authReq(priv, nonce, fp), nonce, fp, ""); err != nil {
		t.Fatalf("auth after pairing again: %v", err)
	}
}

// GuestUnpair survives a reload (another reflopd command reads the file).
func TestGuestUnpairPersists(t *testing.T) {
	dir := t.TempDir()
	s, _ := Open(dir)
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	id, err := s.AddKey("bob", pub, "ssh", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.GuestUnpair(id.ID); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := s2.Get("bob")
	if !got.Revoked || got.GuestUnpaired == nil {
		t.Fatalf("reloaded: %+v", got)
	}
	if _, err := s2.AddKey("bob", pub, "ssh", ""); err == nil || !strings.Contains(err.Error(), "reflopd unpair bob") {
		t.Fatalf("ssh key for an unpaired name: %v", err)
	}
}
