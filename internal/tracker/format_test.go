package tracker

import "testing"

func TestFormatGold(t *testing.T) {
	cases := map[int64]string{
		0:       "0g",
		999:     "999g",
		1000:    "1,000g",
		395120:  "395,120g",
		1234567: "1,234,567g",
	}
	for in, want := range cases {
		if got := FormatGold(in); got != want {
			t.Errorf("FormatGold(%d) = %q, want %q", in, got, want)
		}
	}
}
