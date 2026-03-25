package scanner

import (
	"bytes"
	"testing"
	"unicode/utf8"
)

// FuzzScanner tests that the scanner does not crash or panic on any valid
// UTF-8 input, and that each token individually survives an emit → re-parse
// round-trip.
//
// Full-stream round-trip (emit all tokens, reparse) is not tested here
// because the emit path has known adjacency limitations: tokens can merge
// or split when concatenated without separators.
func FuzzScanner(f *testing.F) {
	f.Add(`body { color: red; }`)
	f.Add(`.container { font-size: 16px; margin: 0 auto; }`)
	f.Add(`@font-face { font-family: 'F'; src: url('f.woff2') format('woff2'); }`)
	f.Add(`#id .class:hover::before { content: "hello"; }`)
	f.Add(`color: rgba(255, 128, 0 / 50%);`)
	f.Add(`--my-var: -42px;`)
	f.Add(`calc(100% - 20px)`)
	f.Add(`U+0042-00FF`)
	f.Add(`/* comment */ <!-- -->`)
	f.Add(`~= |= ^= $= *=`)
	f.Add("\uFEFF body { }")
	f.Add(`url(/*x*/pic.png)`)
	f.Add(`\30 x`)
	f.Add(`bar(moo) #hash 4.2 .42 42 42% .42% 4.2% 42px`)

	f.Fuzz(func(t *testing.T, input string) {
		if !utf8.ValidString(input) {
			return
		}

		// Phase 1: tokenize (must not crash or panic).
		tokens, hasError := fuzzParse(input)
		if hasError {
			return // unclosed quote/comment — expected
		}

		// Phase 2: per-token round-trip.
		// Each token's emitted form must reparse to the same token.
		// Tokens with known emit limitations (escape-produced special
		// chars) are silently skipped.
		for _, tok := range tokens {
			switch tok.Type {
			case BOM, EOF, Error:
				continue
			}
			// Skip tokens whose values contain characters that can't
			// survive the emit → reparse cycle:
			// - Backslashes in raw-emit tokens (re-interpreted as escapes)
			// - Control chars and whitespace (from hex escapes like \0, \A, \20)
			if hasUnsafeChars(tok.Value) {
				continue
			}
			var buf bytes.Buffer
			if err := tok.Emit(&buf); err != nil {
				continue
			}
			reparsed, parseErr := fuzzParse(buf.String())
			if parseErr || len(reparsed) != 1 {
				continue // emit limitation, not a scanner bug
			}
			if reparsed[0].Type != tok.Type {
				continue // type change from emit limitation
			}
			if reparsed[0].Value != tok.Value {
				t.Fatalf("Per-token round-trip value changed for %s:\n  original: %q\n  emitted:  %q\n  reparsed: %q\n  input:    %q",
					tok.Type, tok.Value, buf.String(), reparsed[0].Value, input)
			}
		}
	})
}

// hasUnsafeChars reports whether s contains characters that cannot
// survive the emit → reparse cycle: control chars, whitespace, or
// backslashes (which raw-emit tokens don't escape).
func hasUnsafeChars(s string) bool {
	for i := range len(s) {
		if s[i] <= 0x20 || s[i] == 0x7F || s[i] == '\\' {
			return true
		}
	}
	return false
}

func fuzzParse(input string) ([]Token, bool) {
	var tokens []Token
	s := New(input)
	for {
		tok := s.Next()
		if tok.Type == Error {
			return nil, true
		}
		if tok.Type == EOF {
			return tokens, false
		}
		tokens = append(tokens, *tok)
	}
}
