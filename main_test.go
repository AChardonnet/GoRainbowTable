package rainbowtable

import "testing"

func TestGenerateCharset(t *testing.T) {
	got := generateCharset("a-zA-Z0-9!#@+-")
	wanted := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!#@+-"

	if got != wanted {
		t.Errorf("got %s, wanted %s", got, wanted)
	}
}
