package cli

import "testing"

func TestMainVersion(t *testing.T) {
	if got := Main([]string{"version"}); got != 0 {
		t.Fatalf("version exit = %d, want 0", got)
	}
}

func TestMainUnknownCommand(t *testing.T) {
	if got := Main([]string{"bogus"}); got != 2 {
		t.Fatalf("unknown cmd exit = %d, want 2", got)
	}
}

func TestMainNoArgs(t *testing.T) {
	if got := Main(nil); got != 2 {
		t.Fatalf("no args exit = %d, want 2", got)
	}
}
