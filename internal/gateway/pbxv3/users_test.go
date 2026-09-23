package pbxv3

import "testing"

// v3 accepts only certain page sizes and silently substitutes its default
// for anything else, so a request must be snapped before it is sent.
func TestSnapPerPage(t *testing.T) {
	cases := []struct{ requested, want int }{
		{0, 10},
		{-5, 10},
		{1, 10},
		{10, 10},
		{25, 10},   // the pager offers 25; v3 does not
		{49, 10},
		{50, 50},
		{99, 50},
		{100, 100},
		{5000, 1000},
	}
	for _, c := range cases {
		if got := snapPerPage(c.requested); got != c.want {
			t.Errorf("snapPerPage(%d) = %d, want %d", c.requested, got, c.want)
		}
	}
}
