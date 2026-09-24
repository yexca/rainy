package store

import (
	"reflect"
	"testing"
)

func TestMergeHiddenEntries(t *testing.T) {
	e := func(id string, missing bool) playlistEntry { return playlistEntry{TrackID: id, Missing: missing} }
	tests := []struct {
		name    string
		old     []playlistEntry
		visible []string
		want    []string
	}{
		{"no hidden", []playlistEntry{e("a", false), e("b", false)}, []string{"b", "a"}, []string{"b", "a"}},
		{"hidden follows anchor", []playlistEntry{e("a", false), e("x", true), e("b", false)}, []string{"b", "a"}, []string{"b", "a", "x"}},
		{"leading hidden", []playlistEntry{e("x", true), e("a", false), e("b", false)}, []string{"b", "a"}, []string{"x", "b", "a"}},
		{"anchor removed", []playlistEntry{e("a", false), e("x", true), e("b", false)}, []string{"b"}, []string{"b", "x"}},
		{"duplicate anchors", []playlistEntry{e("a", false), e("a", false), e("x", true)}, []string{"a", "b", "a"}, []string{"a", "b", "a", "x"}},
		{"all hidden", []playlistEntry{e("x", true), e("y", true)}, nil, []string{"x", "y"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mergeHiddenEntries(tt.old, tt.visible); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}
