package report

import "fmt"

// Commas writes n with thousands separators: 4312 -> "4,312".
func Commas(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

// Files is "1 file" or "4,312 files".
func Files(n int) string {
	if n == 1 {
		return "1 file"
	}
	return Commas(n) + " files"
}

// HumanBytes writes a size with binary units: 1536 -> "1.5 KiB".
func HumanBytes(b int64) string {
	const u = 1024
	if b < u {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(u), 0
	for n := b / u; n >= u; n /= u {
		div *= u
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGT"[exp])
}
