package ssh

import "testing"

func TestQuoteRemotePathPreservesTilde(t *testing.T) {
	if got := QuoteRemotePath("/repo"); got != "/repo" {
		t.Fatalf("unexpected safe path quote: %s", got)
	}
	if got := QuoteRemotePath("~/repo with space"); got != "~/'repo with space'" {
		t.Fatalf("unexpected quote: %s", got)
	}
	if got := ExpandTilde("~/repo"); got != "~/repo" {
		t.Fatalf("unexpected expand: %s", got)
	}
}

func TestWrapShellCmd(t *testing.T) {
	got := WrapShellCmd("echo '$HOME'", "zsh")
	want := "zsh -ic 'echo '\"'\"'$HOME'\"'\"''"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
