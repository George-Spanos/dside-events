package main

import (
	"strings"
	"time"
)

// greek maps lowercase Greek letters (accents stripped) to Latin.
var greek = map[rune]string{
	'α': "a", 'β': "v", 'γ': "g", 'δ': "d", 'ε': "e", 'ζ': "z", 'η': "i", 'θ': "th",
	'ι': "i", 'κ': "k", 'λ': "l", 'μ': "m", 'ν': "n", 'ξ': "x", 'ο': "o", 'π': "p",
	'ρ': "r", 'σ': "s", 'ς': "s", 'τ': "t", 'υ': "y", 'φ': "f", 'χ': "ch", 'ψ': "ps", 'ω': "o",
	// accented / dieresis forms
	'ά': "a", 'έ': "e", 'ή': "i", 'ί': "i", 'ό': "o", 'ύ': "y", 'ώ': "o",
	'ϊ': "i", 'ϋ': "y", 'ΐ': "i", 'ΰ': "y",
}

const slugMax = 60

// slugBase turns free text into a lowercase ASCII slug: Greek is
// transliterated, other letters are lowered and stripped of accents where
// they decompose trivially, everything else becomes a single dash. The result
// is capped at slugMax runes on a dash boundary and never empty.
func slugBase(s string) string {
	var b strings.Builder
	dash := true // suppress leading dashes
	for _, r := range strings.ToLower(s) {
		var out string
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out = string(r)
		default:
			if g, ok := greek[r]; ok {
				out = g
			} else if l, ok := latin[r]; ok {
				out = l
			}
		}
		if out == "" {
			if !dash {
				b.WriteByte('-')
				dash = true
			}
			continue
		}
		b.WriteString(out)
		dash = false
	}
	out := strings.TrimRight(b.String(), "-")
	if len(out) > slugMax {
		out = out[:slugMax]
		if i := strings.LastIndexByte(out, '-'); i > 0 {
			out = out[:i]
		}
		out = strings.TrimRight(out, "-")
	}
	if out == "" {
		return "event"
	}
	return out
}

// latin maps common accented Latin letters to their base letter.
var latin = map[rune]string{
	'à': "a", 'á': "a", 'â': "a", 'ã': "a", 'ä': "a", 'å': "a", 'ç': "c",
	'è': "e", 'é': "e", 'ê': "e", 'ë': "e", 'ì': "i", 'í': "i", 'î': "i", 'ï': "i",
	'ñ': "n", 'ò': "o", 'ó': "o", 'ô': "o", 'õ': "o", 'ö': "o", 'ø': "o",
	'ù': "u", 'ú': "u", 'û': "u", 'ü': "u", 'ý': "y", 'ÿ': "y", 'ß': "ss",
}

// eventSlug derives the base slug for an event: transliterated title plus the
// Athens date, e.g. "synavlia-sto-gagarin-2026-09-07".
func eventSlug(title string, start time.Time, loc *time.Location) string {
	return slugBase(title) + "-" + start.In(loc).Format("2006-01-02")
}
