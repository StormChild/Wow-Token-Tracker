package tracker

import "strconv"

// FormatGold renders whole gold with thousands separators, e.g. 395120 → "395,120g".
func FormatGold(gold int64) string {
	s := strconv.FormatInt(gold, 10)
	out := make([]byte, 0, len(s)+len(s)/3+2)
	for i := range len(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, s[i])
	}
	return string(out) + "g"
}
