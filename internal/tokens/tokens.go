package tokens

import "unicode/utf8"

// Estimate returns a conservative token estimate without binding ctxpack to a
// provider-specific tokenizer. English/code averages around 3.5-4 chars/token;
// using 3.6 keeps budget enforcement slightly conservative.
func Estimate(text string) int {
	if text == "" {
		return 0
	}
	runes := utf8.RuneCountInString(text)
	tokens := runes / 4
	if runes%4 != 0 {
		tokens++
	}
	lines := 0
	for _, r := range text {
		if r == '\n' {
			lines++
		}
	}
	return tokens + lines/12 + 1
}
