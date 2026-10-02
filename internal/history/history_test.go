package history

import "testing"

func TestMove(t *testing.T) {
	h := FileHistory{Versions: make([]FileVersion, 3)}
	for _, tc := range []struct{ from, delta, want int }{{0, -1, 0}, {0, 1, 1}, {2, 1, 2}, {2, -1, 1}, {1, 10, 2}} {
		if got := h.Move(tc.from, tc.delta); got != tc.want {
			t.Fatalf("Move(%d,%d)=%d", tc.from, tc.delta, got)
		}
	}
	if (FileHistory{}).Move(0, 1) != 0 {
		t.Fatal("empty history")
	}
}
