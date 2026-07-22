package repo

import "testing"

func TestItoaShot(t *testing.T) {
	cases := map[int]string{0: "0", 1: "1", 12: "12", 12345: "12345"}
	for in, want := range cases {
		if got := itoa(in); got != want {
			t.Fatalf("itoa(%d)=%q want %q", in, got, want)
		}
	}
}
