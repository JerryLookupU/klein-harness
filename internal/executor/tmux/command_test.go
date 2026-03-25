package tmux

import (
	"reflect"
	"testing"
)

func TestBuildNewSessionArgs(t *testing.T) {
	got := BuildNewSession("kh_T_001", "/tmp/repo")
	want := []string{"new-session", "-d", "-s", "kh_T_001", "-c", "/tmp/repo"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected new-session args: %#v", got)
	}
}

func TestBuildPipePaneArgsQuoteLogPath(t *testing.T) {
	got := BuildPipePane("kh_T_001", "/tmp/my repo/tmux.log")
	want := []string{"pipe-pane", "-o", "-t", "kh_T_001", "cat >> '/tmp/my repo/tmux.log'"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected pipe-pane args: %#v", got)
	}
}

func TestBuildAttachSessionArgs(t *testing.T) {
	got := BuildAttachSession("kh_T_001")
	want := []string{"attach-session", "-t", "kh_T_001"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected attach args: %#v", got)
	}
}
