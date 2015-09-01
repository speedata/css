// Copyright 2012 The Gorilla Authors, Copyright 2015 Barracuda Networks.
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scanner

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Type identifies the type of lexical tokens.
type Type int

// String returns a string representation of the token type.
func (t Type) String() string {
	return tokenNames[t]
}

// GoString returns a string representation of the token type.
func (t Type) GoString() string {
	return tokenNames[t]
}

// Token represents a token and the corresponding string.
type Token struct {
	Type   Type
	Value  string
	Line   int
	Column int
}

// String returns a string representation of the token.
func (t *Token) String() string {
	if len(t.Value) > 10 {
		return fmt.Sprintf("%s (line: %d, column: %d): %.10q...",
			t.Type, t.Line, t.Column, t.Value)
	}
	return fmt.Sprintf("%s (line: %d, column: %d): %q",
		t.Type, t.Line, t.Column, t.Value)
}

// All tokens -----------------------------------------------------------------

// The complete list of tokens in CSS3.
const (
	// Scanner flags.
	Error Type = iota
	EOF
	// From now on, only tokens from the CSS specification.
	Ident
	AtKeyword
	String
	Hash
	Number
	Percentage
	Dimension
	URI
	UnicodeRange
	CDO
	CDC
	S
	Comment
	Function
	Includes
	DashMatch
	PrefixMatch
	SuffixMatch
	SubstringMatch
	Delim
	BOM
)

// tokenNames maps Type's to their names. Used for conversion to string.
var tokenNames = map[Type]string{
	Error:          "error",
	EOF:            "EOF",
	Ident:          "IDENT",
	AtKeyword:      "ATKEYWORD",
	String:         "STRING",
	Hash:           "HASH",
	Number:         "NUMBER",
	Percentage:     "PERCENTAGE",
	Dimension:      "DIMENSION",
	URI:            "URI",
	UnicodeRange:   "UNICODE-RANGE",
	CDO:            "CDO",
	CDC:            "CDC",
	S:              "S",
	Comment:        "COMMENT",
	Function:       "FUNCTION",
	Includes:       "INCLUDES",
	DashMatch:      "DASHMATCH",
	PrefixMatch:    "PREFIXMATCH",
	SuffixMatch:    "SUFFIXMATCH",
	SubstringMatch: "SUBSTRINGMATCH",
	Delim:          "DELIM",
	BOM:            "BOM",
}

// Macros and productions -----------------------------------------------------
// http://www.w3.org/TR/css3-syntax/#tokenization

var macroRegexp = regexp.MustCompile(`\{[a-z]+\}`)

// macros maps macro names to patterns to be expanded.
var macros = map[string]string{
	// must be escaped: `\.+*?()|[]{}^$`
	"ident":      `-?{nmstart}{nmchar}*`,
	"name":       `{nmchar}+`,
	"nmstart":    `[a-zA-Z_]|{nonascii}|{escape}`,
	"nonascii":   "[\u0080-\uD7FF\uE000-\uFFFD\U00010000-\U0010FFFF]",
	"unicode":    `\\[0-9a-fA-F]{1,6}{wc}?`,
	"escape":     "{unicode}|\\\\[\u0020-\u007E\u0080-\uD7FF\uE000-\uFFFD\U00010000-\U0010FFFF]",
	"nmchar":     `[a-zA-Z0-9_-]|{nonascii}|{escape}`,
	"num":        `[0-9]*\.[0-9]+|[0-9]+`,
	"string":     `"(?:{stringchar}|')*"|'(?:{stringchar}|")*'`,
	"stringchar": `{urlchar}|[ ]|\\{nl}`,
	"urlchar":    "[\u0009\u0021\u0023-\u0026\u0027-\u007E]|{nonascii}|{escape}",
	"nl":         `[\n\r\f]|\r\n`,
	"w":          `{wc}*`,
	"wc":         `[\t\n\f\r ]`,
}

// productions maps the list of tokens to patterns to be expanded.
var productions = map[Type]string{
	// Unused regexps (matched using other methods) are commented out.
	Ident:        `{ident}`,
	AtKeyword:    `@{ident}`,
	String:       `{string}`,
	Hash:         `#{name}`,
	Number:       `{num}`,
	Percentage:   `{num}%`,
	Dimension:    `{num}{ident}`,
	URI:          `[Uu][Rr][Ll]\({w}(?:{string}|{urlchar}*){w}\)`,
	UnicodeRange: `[Uu]\+[0-9A-F\?]{1,6}(?:-[0-9A-F]{1,6})?`,
	//CDO:            `<!--`,
	CDC:      `-->`,
	S:        `{wc}+`,
	Comment:  `/\*[^\*]*[\*]+(?:[^/][^\*]*[\*]+)*/`,
	Function: `{ident}\(`,
	//Includes:       `~=`,
	//DashMatch:      `\|=`,
	//PrefixMatch:    `\^=`,
	//SuffixMatch:    `\$=`,
	//SubstringMatch: `\*=`,
	//Delim:           `[^"']`,
	//BOM:            "\uFEFF",
}

// matchers maps the list of tokens to compiled regular expressions.
//
// The map is filled on init() using the macros and productions defined in
// the CSS specification.
var matchers = map[Type]*regexp.Regexp{}

// matchOrder is the order to test regexps when first-char shortcuts
// can't be used.
var matchOrder = []Type{
	URI,
	Function,
	UnicodeRange,
	Ident,
	Dimension,
	Percentage,
	Number,
	CDC,
}

func init() {
	// replace macros and compile regexps for productions.
	replaceMacro := func(s string) string {
		return "(?:" + macros[s[1:len(s)-1]] + ")"
	}
	for t, s := range productions {
		for macroRegexp.MatchString(s) {
			s = macroRegexp.ReplaceAllStringFunc(s, replaceMacro)
		}
		matchers[t] = regexp.MustCompile("^(?:" + s + ")")
	}
}

// Scanner --------------------------------------------------------------------

// New returns a new CSS scanner for the given input.
func New(input string) *Scanner {
	// Normalize newlines.
	input = strings.Replace(input, "\r\n", "\n", -1)
	return &Scanner{
		input: input,
		row:   1,
		col:   1,
	}
}

// Scanner scans an input and emits tokens following the CSS3 specification.
type Scanner struct {
	input string
	pos   int
	row   int
	col   int
	err   *Token
}

// Next returns the next token from the input.
//
// At the end of the input the token type is EOF.
//
// If the input can't be tokenized the token type is Error. This occurs
// in case of unclosed quotation marks or comments.
func (s *Scanner) Next() *Token {
	if s.err != nil {
		return s.err
	}
	if s.pos >= len(s.input) {
		s.err = &Token{EOF, "", s.row, s.col}
		return s.err
	}
	if s.pos == 0 {
		// Test BOM only once, at the beginning of the file.
		if strings.HasPrefix(s.input, "\uFEFF") {
			return s.emitSimple(BOM, "\uFEFF")
		}
	}
	// There's a lot we can guess based on the first byte so we'll take a
	// shortcut before testing multiple regexps.
	input := s.input[s.pos:]
	switch input[0] {
	case '\t', '\n', '\f', '\r', ' ':
		// Whitespace.
		return s.emitToken(S, matchers[S].FindString(input))
	case '.':
		// Dot is too common to not have a quick check.
		// We'll test if this is a Char; if it is followed by a number it is a
		// dimension/percentage/number, and this will be matched later.
		if len(input) > 1 && !unicode.IsDigit(rune(input[1])) {
			return s.emitSimple(Delim, ".")
		}
	case '#':
		// Another common one: Hash or Char.
		if match := matchers[Hash].FindString(input); match != "" {
			return s.emitToken(Hash, match)
		}
		return s.emitSimple(Delim, "#")
	case '@':
		// Another common one: AtKeyword or Char.
		if match := matchers[AtKeyword].FindString(input); match != "" {
			return s.emitSimple(AtKeyword, match)
		}
		return s.emitSimple(Delim, "@")
	case ':', ',', ';', '%', '&', '+', '=', '>', '(', ')', '[', ']', '{', '}':
		// More common chars.
		return s.emitSimple(Delim, string(input[0]))
	case '"', '\'':
		// String or error.
		match := matchers[String].FindString(input)
		if match != "" {
			return s.emitToken(String, match)
		} else {
			s.err = &Token{Error, "unclosed quotation mark", s.row, s.col}
			return s.err
		}
	case '/':
		// Comment, error or Char.
		if len(input) > 1 && input[1] == '*' {
			match := matchers[Comment].FindString(input)
			if match != "" {
				return s.emitToken(Comment, match)
			} else {
				s.err = &Token{Error, "unclosed comment", s.row, s.col}
				return s.err
			}
		}
		return s.emitSimple(Delim, "/")
	case '~':
		// Includes or Char.
		return s.emitPrefixOrChar(Includes, "~=")
	case '|':
		// DashMatch or Char.
		return s.emitPrefixOrChar(DashMatch, "|=")
	case '^':
		// PrefixMatch or Char.
		return s.emitPrefixOrChar(PrefixMatch, "^=")
	case '$':
		// SuffixMatch or Char.
		return s.emitPrefixOrChar(SuffixMatch, "$=")
	case '*':
		// SubstringMatch or Char.
		return s.emitPrefixOrChar(SubstringMatch, "*=")
	case '<':
		// CDO or Char.
		return s.emitPrefixOrChar(CDO, "<!--")
	}
	// Test all regexps, in order.
	for _, token := range matchOrder {
		if match := matchers[token].FindString(input); match != "" {
			return s.emitToken(token, match)
		}
	}
	// We already handled unclosed quotation marks and comments,
	// so this can only be a Char.
	r, width := utf8.DecodeRuneInString(input)
	token := &Token{Delim, string(r), s.row, s.col}
	s.col += width
	s.pos += width
	return token
}

// updatePosition updates input coordinates based on the consumed text.
func (s *Scanner) updatePosition(text string) {
	width := utf8.RuneCountInString(text)
	lines := strings.Count(text, "\n")
	s.row += lines
	if lines == 0 {
		s.col += width
	} else {
		s.col = utf8.RuneCountInString(text[strings.LastIndex(text, "\n"):])
	}
	s.pos += len(text) // while col is a rune index, pos is a byte index
}

// emitToken returns a Token for the string v and updates the scanner position.
func (s *Scanner) emitToken(t Type, v string) *Token {
	token := &Token{t, v, s.row, s.col}
	s.updatePosition(v)
	return token
}

// emitSimple returns a Token for the string v and updates the scanner
// position in a simplified manner.
//
// The string is known to have only ASCII characters and to not have a newline.
func (s *Scanner) emitSimple(t Type, v string) *Token {
	token := &Token{t, v, s.row, s.col}
	s.col += len(v)
	s.pos += len(v)
	return token
}

// emitPrefixOrChar returns a Token for type t if the current position
// matches the given prefix. Otherwise it returns a Char token using the
// first character from the prefix.
//
// The prefix is known to have only ASCII characters and to not have a newline.
func (s *Scanner) emitPrefixOrChar(t Type, prefix string) *Token {
	if strings.HasPrefix(s.input[s.pos:], prefix) {
		return s.emitSimple(t, prefix)
	}
	return s.emitSimple(Delim, string(prefix[0]))
}

func unbackslash(s string, isString bool) string {
	// in general, strings are short, and do not contain backslashes; if
	// that is the case, just bail out with no additional allocation.
	if !strings.Contains(s, "\\") {
		return s
	}

	in := bytes.NewBufferString(s)
	var out bytes.Buffer
	out.Grow(len(s))

	hexChars := make([]byte, 6, 6)

	for {
		c, err := in.ReadByte()
		if err == io.EOF {
			break
		}
		if c != '\\' {
			out.WriteByte(c)
			continue
		}

		// c is now the first byte after the backslash
		c, err = in.ReadByte()
		if err == io.EOF {
			out.WriteByte('\\')
			break
		}

		// CSS 4.1.3 third bullet point: Rules for decoding backslashes.
		// We won't process comments, so we skip that for now.
		// First, special string rules:
		if isString {
			// If this is a string token, and the next thing is a newline
			// (LF or CRLF), then the whole thing didn't happen.
			if c == '\n' {
				continue
			}
			if c == '\r' {
				c, err = in.ReadByte()
				if err == io.EOF {
					out.WriteByte('\\')
					break
				}
				if c == '\n' {
					continue
				} else {
					// standard does not say what to do with backslash-CR
					// that is not followed by a LF. Go ahead and eat the
					// CR and return to normal processing.
					in.UnreadByte()
					continue
				}
			}
		}

		// Second, any non-hex digit, CR, LF, or FF gets replaced by the
		// literal character. CR, LF, or FF, if left unescaped, presumably
		// didn't make it this far to be decoded. So that just leaves the
		// hex digits and the not-hex-digits.
		switch {
		case isHexChar(c):
			// A hex specification is either 0-5 digits followed by
			// optional whitespace which will be eaten, or exactly six
			// digits.
			hexChars = hexChars[:0]
			hexChars = append(hexChars, c)

		HEXLOOP:
			for len(hexChars) < 6 {
				nextChar, err := in.ReadByte()
				if err == io.EOF {
					break HEXLOOP
				}

				switch {
				case isHexChar(nextChar):
					hexChars = append(hexChars, nextChar)
				case isWhitespace(nextChar):
					// this ends up eating the whitespace char
					break HEXLOOP
				default:
					// Non-space chars do not get eaten
					in.UnreadByte()
					break HEXLOOP
				}
			}

			// The rune this represents:
			r := decodeHex(hexChars)
			out.WriteRune(r)

		default:
			out.WriteByte(c)
		}

	}

	return out.String()
}

func isWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f'
}

func isHexChar(c byte) bool {
	return c >= '0' && c <= '9' ||
		c >= 'a' && c <= 'f' ||
		c >= 'A' && c <= 'F'
}

func decodeHex(in []byte) rune {
	val := rune(0)

	for _, c := range in {
		val = val << 4
		val = val + rune(fromHexChar(c))
	}

	return val
}

// fromHexChar copied from encoding/hex/hex.go, except this is guaranteed
// to only be called on hex chars, so no success flag.
func fromHexChar(c byte) byte {
	switch {
	case '0' <= c && c <= '9':
		return c - '0'
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10
	}
	// satisfies compiler that there is a return.
	return 0
}
