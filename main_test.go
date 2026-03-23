package main

import (
	"bytes"
	"encoding/hex"
	"log"
	"testing"
)

var (
	charset        = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!#@+-"
	passwordLength = 6
	chainLength    = 1000
)

func TestGenerateCharset(t *testing.T) {
	got := generateCharset("a-zA-Z0-9!#@+-")
	wanted := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!#@+-"

	if got != wanted {
		t.Errorf("got %s, wanted %s", got, wanted)
	}
}

func TestHash(t *testing.T) {
	got := hash("a")
	wanted, err := hex.DecodeString("ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb")

	if err != nil {
		log.Fatal(err)
	}

	if got != [32]byte(wanted) {
		t.Errorf("got %s, wanted %s", got, wanted)
	}

	got = hash("b")
	wanted, err = hex.DecodeString("3e23e8160039594a33894f6564e1b1348bbd7a0088d42c4acb73eeaed59c009d")

	if err != nil {
		log.Fatal(err)
	}

	if got != [32]byte(wanted) {
		t.Errorf("got %s, wanted %s", got, wanted)
	}
}

func TestReduce(t *testing.T) {
	got := reduce(hash("a"), 0, 6, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!#@+-")
	wanted := "EvBSP6"

	if got != wanted {
		t.Errorf("got %s, wanted %s", got, wanted)
	}
}

func TestGenerateChain(t *testing.T) {
	got := generateChain("a", 1000, 6, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!#@+-")

	temp, err := hex.DecodeString("167f77bdbcbd83ba7366cbd55f9cc87c98e7f5e5e35b130fb6915347c2d0fe6e")
	if err != nil {
		log.Fatal(err)
	}

	wanted := TableEntry{
		End: [32]byte(temp),
	}
	wanted.Start = []byte("a")

	if !bytes.Equal(got.Start, wanted.Start) {
		t.Errorf("Start mismatch: got %s, wanted %s", got.Start, wanted.Start)
	}

	// Compare the End array (this still works with != because it's a fixed array)
	if got.End != wanted.End {
		t.Errorf("End hash mismatch:\nGot:    %x\nWanted: %x", got.End, wanted.End)
	}
}
