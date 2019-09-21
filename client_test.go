package main

import "testing"

// код писать тут
func Test(t *testing.T) {
	got := 2
	want := 1 + 1

	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}
