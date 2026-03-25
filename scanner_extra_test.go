package css

import (
	"bytes"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Edge-case tests
// ---------------------------------------------------------------------------

func TestNegativeNumbers(t *testing.T) {
	// CSS Syntax Level 3: leading minus is part of the number token.
	// "-42px" is a single Dimension("-42px"), "-42" is Number("-42"), etc.
	for _, test := range []struct {
		input  string
		tokens []Token
	}{
		{"-42px", []Token{T(Dimension, "-42px")}},
		{"-42%", []Token{T(Percentage, "-42")}},
		{"-42", []Token{T(Number, "-42")}},
		{"-.5em", []Token{T(Dimension, "-.5em")}},
		{"-.5", []Token{T(Number, "-.5")}},
		// Positive numbers with explicit sign
		{"+42px", []Token{T(Dimension, "+42px")}},
		{"+42%", []Token{T(Percentage, "+42")}},
		{"+42", []Token{T(Number, "+42")}},
		{"+.5", []Token{T(Number, "+.5")}},
		// Plus/minus as delimiters when not followed by digit
		{"+ x", []Token{T(Delim, "+"), T(S, " "), T(Ident, "x")}},
		{"- x", []Token{T(Delim, "-"), T(S, " "), T(Ident, "x")}},
	} {
		tokens, err := parse(test.input)
		if err != nil {
			t.Fatalf("For %q: unexpected error", test.input)
		}
		if len(tokens) != len(test.tokens) {
			t.Fatalf("For %q: expected %d tokens, got %d: %#v", test.input, len(test.tokens), len(tokens), tokens)
		}
		for i, tok := range tokens {
			if tok.Type != test.tokens[i].Type || tok.Value != test.tokens[i].Value {
				t.Fatalf("For %q token %d: expected %#v, got %#v", test.input, i, test.tokens[i], tok)
			}
		}
	}
}

func TestUnicodeIdentifiers(t *testing.T) {
	for _, test := range []struct {
		input string
		value string
	}{
		{"café", "café"},
		{"über", "über"},
		{"日本語", "日本語"},
		{"α-β-γ", "α-β-γ"},
		{"emöji", "emöji"},
	} {
		tokens, err := parse(test.input)
		if err != nil {
			t.Fatalf("For %q: unexpected error", test.input)
		}
		if len(tokens) != 1 {
			t.Fatalf("For %q: expected 1 token, got %d: %#v", test.input, len(tokens), tokens)
		}
		if tokens[0].Type != Ident || tokens[0].Value != test.value {
			t.Fatalf("For %q: expected Ident %q, got %s %q", test.input, test.value, tokens[0].Type, tokens[0].Value)
		}
	}
}

func TestEmptyInput(t *testing.T) {
	s := New("")
	tok := s.Next()
	if tok.Type != EOF {
		t.Fatalf("Expected EOF for empty input, got %s", tok.Type)
	}
	// Calling Next again should still return EOF
	tok = s.Next()
	if tok.Type != EOF {
		t.Fatalf("Expected EOF on second call, got %s", tok.Type)
	}
}

func TestOnlyWhitespace(t *testing.T) {
	tokens, err := parse("   \t\n\r  ")
	if err != nil {
		t.Fatal("Unexpected error for whitespace-only input")
	}
	if len(tokens) != 1 || tokens[0].Type != S {
		t.Fatalf("Expected single S token, got %#v", tokens)
	}
}

func TestMultilineComments(t *testing.T) {
	for _, test := range []struct {
		input string
		value string
	}{
		{"/* line1\nline2 */", " line1\nline2 "},
		{"/* * * * */", " * * * "},
		{"/****/", "**"},
		{"/**/", ""},
	} {
		tokens, err := parse(test.input)
		if err != nil {
			t.Fatalf("For %q: unexpected error", test.input)
		}
		if len(tokens) != 1 || tokens[0].Type != Comment {
			t.Fatalf("For %q: expected Comment, got %#v", test.input, tokens)
		}
		if tokens[0].Value != test.value {
			t.Fatalf("For %q: expected value %q, got %q", test.input, test.value, tokens[0].Value)
		}
	}
}

func TestUnclosedComment(t *testing.T) {
	_, err := parse("/* never closed")
	if err == nil {
		t.Fatal("Expected error for unclosed comment")
	}
}

func TestUnclosedString(t *testing.T) {
	for _, input := range []string{
		`"never closed`,
		`'never closed`,
	} {
		_, err := parse(input)
		if err == nil {
			t.Fatalf("Expected error for unclosed string: %q", input)
		}
	}
}

func TestEscapedIdentifiers(t *testing.T) {
	for _, test := range []struct {
		input string
		value string
	}{
		{`\30 x`, "0x"},            // hex escape followed by space
		{`\000030x`, "0x"},         // 6-digit hex escape
		{`\41`, "A"},              // hex A without trailing space
		{`\41 `, "A"},             // hex A with trailing space
		{`\!important`, "!important"}, // literal escape
	} {
		tokens, err := parse(test.input)
		if err != nil {
			t.Fatalf("For %q: unexpected error", test.input)
		}
		if len(tokens) == 0 {
			t.Fatalf("For %q: no tokens", test.input)
		}
		if tokens[0].Type != Ident || tokens[0].Value != test.value {
			t.Fatalf("For %q: expected Ident %q, got %s %q (all tokens: %#v)", test.input, test.value, tokens[0].Type, tokens[0].Value, tokens)
		}
	}
}

func TestStringEscapes(t *testing.T) {
	for _, test := range []struct {
		input string
		value string
	}{
		{`"hello\nworld"`, "hellonworld"},  // \n in CSS string = literal n, not newline
		{`"hello\27world"`, "hello'world"}, // hex escape for apostrophe
		// NOTE: \0Ab reads 3 hex digits greedily (0, A, b) → U+0AB = «
		// To get a newline, you need \00000A or \0A followed by space
		{`"line\0A break"`, "line\nbreak"}, // hex escape for newline (space-terminated)
	} {
		tokens, err := parse(test.input)
		if err != nil {
			t.Fatalf("For %q: unexpected error", test.input)
		}
		if len(tokens) != 1 || tokens[0].Type != String {
			t.Fatalf("For %q: expected 1 String, got %#v", test.input, tokens)
		}
		if tokens[0].Value != test.value {
			t.Fatalf("For %q: expected %q, got %q", test.input, test.value, tokens[0].Value)
		}
	}
}

func TestLineColumnTracking(t *testing.T) {
	s := New("ab\ncd\nef")
	tok := s.Next() // "ab"
	if tok.Line != 1 || tok.Column != 1 {
		t.Fatalf("Token 'ab': expected line 1 col 1, got line %d col %d", tok.Line, tok.Column)
	}
	tok = s.Next() // "\n"
	tok = s.Next() // "cd"
	if tok.Line != 2 || tok.Column != 1 {
		t.Fatalf("Token 'cd': expected line 2 col 1, got line %d col %d", tok.Line, tok.Column)
	}
	tok = s.Next() // "\n"
	tok = s.Next() // "ef"
	if tok.Line != 3 || tok.Column != 1 {
		t.Fatalf("Token 'ef': expected line 3 col 1, got line %d col %d", tok.Line, tok.Column)
	}
}

func TestCSSSelectors(t *testing.T) {
	// Typical CSS selectors should tokenize without error
	inputs := []string{
		"div > p.class#id",
		".foo:hover::before",
		"[data-attr~='value']",
		"a:not(.active)",
		"*",
		"div + p ~ span",
	}
	for _, input := range inputs {
		_, err := parse(input)
		if err != nil {
			t.Fatalf("For %q: unexpected error", input)
		}
	}
}

func TestModernCSSValues(t *testing.T) {
	// Modern CSS constructs should tokenize (even if the scanner is CSS2-based)
	inputs := []string{
		"calc(100% - 20px)",
		"var(--my-color)",
		"clamp(1rem, 2vw, 3rem)",
		"rgb(255 128 0 / 50%)",
		"linear-gradient(to right, red, blue)",
		"env(safe-area-inset-top)",
	}
	for _, input := range inputs {
		tokens, err := parse(input)
		if err != nil {
			t.Fatalf("For %q: unexpected error", input)
		}
		if len(tokens) == 0 {
			t.Fatalf("For %q: no tokens", input)
		}
	}
}

func TestCustomProperties(t *testing.T) {
	// CSS Syntax Level 3: custom properties (--name) are a single ident token.
	for _, test := range []struct {
		input string
		value string
	}{
		{"--my-var", "--my-var"},
		{"--color", "--color"},
		{"--a", "--a"},
		{"--123", "--123"},
		{"--my-var-2", "--my-var-2"},
	} {
		tokens, err := parse(test.input)
		if err != nil {
			t.Fatalf("For %q: unexpected error", test.input)
		}
		if len(tokens) != 1 || tokens[0].Type != Ident {
			t.Fatalf("For %q: expected single Ident, got %#v", test.input, tokens)
		}
		if tokens[0].Value != test.value {
			t.Fatalf("For %q: expected %q, got %q", test.input, test.value, tokens[0].Value)
		}
	}
}

func TestConsecutiveOperators(t *testing.T) {
	for _, test := range []struct {
		input string
		types []Type
	}{
		{"~=|=", []Type{Includes, DashMatch}},
		{"^=$=*=", []Type{PrefixMatch, SuffixMatch, SubstringMatch}},
	} {
		tokens, err := parse(test.input)
		if err != nil {
			t.Fatalf("For %q: unexpected error", test.input)
		}
		if len(tokens) != len(test.types) {
			t.Fatalf("For %q: expected %d tokens, got %d: %#v", test.input, len(test.types), len(tokens), tokens)
		}
		for i, tok := range tokens {
			if tok.Type != test.types[i] {
				t.Fatalf("For %q token %d: expected %s, got %s", test.input, i, test.types[i], tok.Type)
			}
		}
	}
}

func TestUnicodeRange(t *testing.T) {
	for _, test := range []struct {
		input string
		value string
	}{
		{"U+0000-00FF", "U+0000-00FF"},
		{"U+0042", "U+0042"},
		{"U+????", "U+????"},
		{"U+00??", "U+00??"},
	} {
		tokens, err := parse(test.input)
		if err != nil {
			t.Fatalf("For %q: unexpected error", test.input)
		}
		if len(tokens) == 0 {
			t.Fatalf("For %q: no tokens", test.input)
		}
		if tokens[0].Type != UnicodeRange {
			t.Fatalf("For %q: expected UnicodeRange, got %s %q", test.input, tokens[0].Type, tokens[0].Value)
		}
	}
}

func TestDimensionUnits(t *testing.T) {
	units := []string{
		"px", "em", "rem", "vh", "vw", "vmin", "vmax",
		"cm", "mm", "in", "pt", "pc", "ch", "ex",
		"deg", "rad", "grad", "turn",
		"s", "ms", "Hz", "kHz", "dpi", "dpcm", "dppx",
		"fr",
	}
	for _, unit := range units {
		input := "42" + unit
		tokens, err := parse(input)
		if err != nil {
			t.Fatalf("For %q: unexpected error", input)
		}
		if len(tokens) != 1 || tokens[0].Type != Dimension {
			t.Fatalf("For %q: expected Dimension, got %#v", input, tokens)
		}
	}
}

func TestBOMHandling(t *testing.T) {
	// BOM at start
	tokens, err := parse("\uFEFF body { }")
	if err != nil {
		t.Fatal("Unexpected error")
	}
	if tokens[0].Type != BOM {
		t.Fatalf("Expected BOM first, got %s", tokens[0].Type)
	}

	// BOM not at start should not be BOM token
	tokens, err = parse("a\uFEFF")
	if err != nil {
		t.Fatal("Unexpected error")
	}
	hasBOM := false
	for _, tok := range tokens {
		if tok.Type == BOM {
			hasBOM = true
		}
	}
	if hasBOM {
		t.Fatal("BOM should only be detected at start of input")
	}
}

func TestEmitRoundTrip(t *testing.T) {
	// Complex real-world CSS should round-trip through Emit
	input := `.container {
  font-size: 16px;
  color: #333;
  background: url('img/bg.png');
  content: "hello world";
  margin: 0 auto;
}`
	tokens, err := parse(input)
	if err != nil {
		t.Fatal("Unexpected error")
	}
	var buf bytes.Buffer
	for _, tok := range tokens {
		if err := tok.Emit(&buf); err != nil {
			t.Fatalf("Emit failed: %v", err)
		}
	}
	// Re-parse the emitted output
	tokens2, err := parse(buf.String())
	if err != nil {
		t.Fatalf("Re-parse of emitted output failed: %v", err)
	}
	if len(tokens) != len(tokens2) {
		t.Fatalf("Round-trip token count mismatch: %d vs %d", len(tokens), len(tokens2))
	}
	for i := range tokens {
		if tokens[i].Type != tokens2[i].Type || tokens[i].Value != tokens2[i].Value {
			t.Fatalf("Round-trip mismatch at token %d:\n  original: %#v\n  reparsed: %#v", i, tokens[i], tokens2[i])
		}
	}
}

func TestLargeInput(t *testing.T) {
	// Test that the scanner handles large inputs without issues
	var sb strings.Builder
	for i := 0; i < 1000; i++ {
		sb.WriteString(".class-")
		sb.WriteString(strings.Repeat("a", 50))
		sb.WriteString(" { color: #fff; font-size: 12px; }\n")
	}
	input := sb.String()
	tokens, err := parse(input)
	if err != nil {
		t.Fatal("Unexpected error on large input")
	}
	if len(tokens) == 0 {
		t.Fatal("No tokens from large input")
	}
}

// ---------------------------------------------------------------------------
// ReDoS / pathological input tests
// ---------------------------------------------------------------------------

func TestCommentRegexNotPathological(t *testing.T) {
	// Potential ReDoS: Comment regex with many asterisks
	input := "/*" + strings.Repeat("*", 10000) + "/"
	tokens, err := parse(input)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(tokens) != 1 || tokens[0].Type != Comment {
		t.Fatalf("Expected Comment, got %#v", tokens)
	}
}

func TestManyNestedParens(t *testing.T) {
	// Deep nesting of function calls
	input := strings.Repeat("a(", 100) + "x" + strings.Repeat(")", 100)
	_, err := parse(input)
	if err != nil {
		t.Fatalf("Unexpected error on nested parens: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

var benchmarkCSS = `.container {
  display: flex;
  justify-content: center;
  align-items: center;
  width: 100%;
  max-width: 1200px;
  margin: 0 auto;
  padding: 16px 24px;
  font-family: 'Helvetica Neue', Arial, sans-serif;
  font-size: 14px;
  line-height: 1.5;
  color: #333333;
  background-color: rgba(255, 255, 255, 0.95);
  border: 1px solid #e0e0e0;
  border-radius: 4px;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.container:hover {
  border-color: #007bff;
  box-shadow: 0 4px 8px rgba(0, 123, 255, 0.2);
}

@media (max-width: 768px) {
  .container {
    padding: 8px 12px;
    font-size: 12px;
  }
}

@font-face {
  font-family: 'CustomFont';
  src: url('/fonts/custom.woff2') format('woff2'),
       url('/fonts/custom.woff') format('woff');
  font-weight: 400;
  font-style: normal;
  font-display: swap;
}`

func BenchmarkScanTypicalCSS(b *testing.B) {
	for b.Loop() {
		s := New(benchmarkCSS)
		for {
			tok := s.Next()
			if tok.Type == EOF || tok.Type == Error {
				break
			}
		}
	}
}

func BenchmarkScanSimpleRule(b *testing.B) {
	input := "color: #fff;"
	for b.Loop() {
		s := New(input)
		for {
			tok := s.Next()
			if tok.Type == EOF || tok.Type == Error {
				break
			}
		}
	}
}

func BenchmarkScanLargeCSS(b *testing.B) {
	// ~50KB CSS
	var sb strings.Builder
	for i := 0; i < 500; i++ {
		sb.WriteString(".class-")
		sb.WriteString(string(rune('a' + (i % 26))))
		sb.WriteString(" { color: #fff; font-size: 12px; margin: 0 auto; padding: 10px 20px; }\n")
	}
	input := sb.String()
	b.ResetTimer()
	for b.Loop() {
		s := New(input)
		for {
			tok := s.Next()
			if tok.Type == EOF || tok.Type == Error {
				break
			}
		}
	}
}

func BenchmarkScanURL(b *testing.B) {
	input := "url('https://example.com/path/to/resource.woff2?v=123')"
	for b.Loop() {
		s := New(input)
		for {
			tok := s.Next()
			if tok.Type == EOF || tok.Type == Error {
				break
			}
		}
	}
}

func BenchmarkUnbackslash(b *testing.B) {
	input := `hello\26 world\27 foo\2F bar`
	for b.Loop() {
		unbackslash(input, false)
	}
}

func BenchmarkEmit(b *testing.B) {
	tokens, _ := parse(benchmarkCSS)
	buf := &bytes.Buffer{}
	b.ResetTimer()
	for b.Loop() {
		buf.Reset()
		for _, tok := range tokens {
			tok.Emit(buf)
		}
	}
}

func BenchmarkNewlineNormalization(b *testing.B) {
	// Input with many \r\n sequences to test the normalization cost
	input := strings.Repeat("body { color: red; }\r\n", 500)
	for b.Loop() {
		s := New(input)
		for {
			tok := s.Next()
			if tok.Type == EOF || tok.Type == Error {
				break
			}
		}
	}
}
