package main

import "testing"

func TestGenerateCharset(t *testing.T) {
	got := generateCharset("a-zA-Z0-9!#@+-")
	wanted := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!#@+-"

	if got != wanted {
		t.Errorf("got %s, wanted %s", got, wanted)
	}
}

func TestHash(t *testing.T) {
	got := hash("a")
	wanted := "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb"

	if got != wanted {
		t.Errorf("got %s, wanted %s", got, wanted)
	}

	got = hash("b")
	wanted = "3e23e8160039594a33894f6564e1b1348bbd7a0088d42c4acb73eeaed59c009d"

	if got != wanted {
		t.Errorf("got %s, wanted %s", got, wanted)
	}
}
