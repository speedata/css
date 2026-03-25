// Copyright as given in CONTRIBUTORS
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scanner

import (
	"strings"
	"unicode/utf8"
)

// --------------------------------------------------------------------
// Character classification helpers
// --------------------------------------------------------------------

func isDigitByte(c byte) bool {
	return c >= '0' && c <= '9'
}

func isNmStartByte(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}

func isNmCharByte(c byte) bool {
	return isNmStartByte(c) || isDigitByte(c) || c == '-'
}

// isUpperHex returns true for digits and uppercase A-F only.
// Used for UnicodeRange which per spec accepts only uppercase hex.
func isUpperHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'F')
}

// startsWithFold checks if s starts with prefix, case-insensitive (ASCII only).
func startsWithFold(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		a, b := s[i], prefix[i]
		if a != b {
			if a >= 'A' && a <= 'Z' {
				a += 'a' - 'A'
			}
			if b >= 'A' && b <= 'Z' {
				b += 'a' - 'A'
			}
			if a != b {
				return false
			}
		}
	}
	return true
}

// --------------------------------------------------------------------
// Scanner
// --------------------------------------------------------------------

// New returns a new CSS scanner for the given input.
func New(input string) *Scanner {
	// Normalize newlines. Only allocate if the input contains \r.
	if strings.ContainsRune(input, '\r') {
		input = strings.ReplaceAll(input, "\r\n", "\n")
	}
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

// --------------------------------------------------------------------
// Scan length helpers
//
// These return byte lengths without modifying scanner state.
// The offset parameter is relative to s.pos.
// --------------------------------------------------------------------

// scanEscapeLen returns the byte length of an escape sequence starting at
// s.input[s.pos+offset] (which should be a backslash). Returns 0 if not
// a valid escape.
func (s *Scanner) scanEscapeLen(offset int) int {
	pos := s.pos + offset
	if pos >= len(s.input) || s.input[pos] != '\\' {
		return 0
	}
	pos++
	if pos >= len(s.input) {
		return 0 // lone backslash
	}
	c := s.input[pos]
	if isHexChar(c) {
		// Hex escape: 1-6 hex digits, optional single trailing whitespace.
		pos++
		for i := 1; i < 6 && pos < len(s.input) && isHexChar(s.input[pos]); i++ {
			pos++
		}
		if pos < len(s.input) && isWhitespace(s.input[pos]) {
			pos++
		}
		return pos - (s.pos + offset)
	}
	// Literal escape: any char in U+0020..U+007E or nonascii.
	if c >= 0x80 {
		_, w := utf8.DecodeRuneInString(s.input[pos:])
		return 1 + w
	}
	if c >= 0x20 && c <= 0x7e {
		return 2
	}
	return 0
}

// scanNameLen returns the byte length of consecutive nmchar characters
// starting at s.input[s.pos+offset]. nmchar = [a-zA-Z0-9_-] | nonascii | escape.
func (s *Scanner) scanNameLen(offset int) int {
	pos := s.pos + offset
	start := pos
	for pos < len(s.input) {
		c := s.input[pos]
		if isNmCharByte(c) {
			pos++
		} else if c >= 0x80 {
			_, w := utf8.DecodeRuneInString(s.input[pos:])
			pos += w
		} else if c == '\\' {
			n := s.scanEscapeLen(pos - s.pos)
			if n == 0 {
				break
			}
			pos += n
		} else {
			break
		}
	}
	return pos - start
}

// scanIdentLen returns the byte length of an identifier starting at
// s.input[s.pos+offset]. Supports CSS3 custom properties (--name).
// Returns 0 if no valid ident found.
func (s *Scanner) scanIdentLen(offset int) int {
	pos := s.pos + offset
	start := pos
	if pos >= len(s.input) {
		return 0
	}

	// Case 1: --{nmchar}+ (custom properties, requires at least one nmchar
	// so that "-->" is not consumed as ident).
	if pos+1 < len(s.input) && s.input[pos] == '-' && s.input[pos+1] == '-' {
		pos += 2
		n := s.scanNameLen(pos - s.pos)
		if n == 0 {
			return 0
		}
		return 2 + n
	}

	// Case 2: -?{nmstart}{nmchar}*
	if s.input[pos] == '-' {
		pos++
		if pos >= len(s.input) {
			return 0
		}
	}

	c := s.input[pos]
	if isNmStartByte(c) {
		pos++
	} else if c >= 0x80 {
		_, w := utf8.DecodeRuneInString(s.input[pos:])
		pos += w
	} else if c == '\\' {
		n := s.scanEscapeLen(pos - s.pos)
		if n == 0 {
			return 0
		}
		pos += n
	} else {
		return 0
	}

	pos += s.scanNameLen(pos - s.pos)
	return pos - start
}

// scanNumLen returns the byte length of a number (with optional sign)
// starting at s.input[s.pos+offset]. Returns 0 if no valid number found.
func (s *Scanner) scanNumLen(offset int) int {
	pos := s.pos + offset
	start := pos
	if pos >= len(s.input) {
		return 0
	}

	// Optional sign
	if s.input[pos] == '+' || s.input[pos] == '-' {
		pos++
	}

	// Integer part
	hasDigits := false
	for pos < len(s.input) && isDigitByte(s.input[pos]) {
		pos++
		hasDigits = true
	}

	// Decimal part
	if pos < len(s.input) && s.input[pos] == '.' {
		if pos+1 < len(s.input) && isDigitByte(s.input[pos+1]) {
			pos++ // consume dot
			for pos < len(s.input) && isDigitByte(s.input[pos]) {
				pos++
			}
			return pos - start
		}
		if !hasDigits {
			return 0
		}
		return pos - start
	}

	if !hasDigits {
		return 0
	}
	return pos - start
}

// scanStringLen returns the byte length of a quoted string (including quotes)
// starting at s.input[s.pos+offset], and whether the scan was successful.
func (s *Scanner) scanStringLen(offset int) (int, bool) {
	pos := s.pos + offset
	if pos >= len(s.input) {
		return 0, false
	}
	quote := s.input[pos]
	if quote != '"' && quote != '\'' {
		return 0, false
	}
	pos++
	for pos < len(s.input) {
		c := s.input[pos]
		if c == quote {
			return pos + 1 - (s.pos + offset), true
		}
		if c == '\\' {
			pos++
			if pos >= len(s.input) {
				return 0, false
			}
			nc := s.input[pos]
			if isHexChar(nc) {
				// Hex escape: up to 6 hex digits + optional whitespace.
				pos++
				for i := 1; i < 6 && pos < len(s.input) && isHexChar(s.input[pos]); i++ {
					pos++
				}
				if pos < len(s.input) && isWhitespace(s.input[pos]) {
					pos++
				}
			} else if nc == '\n' || nc == '\f' {
				pos++
			} else if nc == '\r' {
				pos++
				if pos < len(s.input) && s.input[pos] == '\n' {
					pos++
				}
			} else if nc >= 0x80 {
				_, w := utf8.DecodeRuneInString(s.input[pos:])
				pos += w
			} else {
				pos++
			}
			continue
		}
		if c == '\n' || c == '\r' || c == '\f' {
			return 0, false // unescaped newline terminates string (error)
		}
		if c >= 0x80 {
			_, w := utf8.DecodeRuneInString(s.input[pos:])
			pos += w
		} else {
			pos++
		}
	}
	return 0, false // unclosed string
}

// scanCommentLen returns the byte length of a /* ... */ comment starting at
// s.pos, and whether the scan was successful.
func (s *Scanner) scanCommentLen() (int, bool) {
	pos := s.pos
	if pos+1 >= len(s.input) || s.input[pos] != '/' || s.input[pos+1] != '*' {
		return 0, false
	}
	pos += 2
	for pos+1 < len(s.input) {
		if s.input[pos] == '*' && s.input[pos+1] == '/' {
			return pos + 2 - s.pos, true
		}
		pos++
	}
	return 0, false // unclosed comment
}

// scanWhitespaceLen returns the byte length of consecutive whitespace
// starting at s.input[s.pos+offset].
func (s *Scanner) scanWhitespaceLen(offset int) int {
	pos := s.pos + offset
	start := pos
	for pos < len(s.input) && isWhitespace(s.input[pos]) {
		pos++
	}
	return pos - start
}

// scanUnicodeRangeLen returns the byte length of a unicode range token
// starting at s.pos. Format: U+hex{1,6}(-hex{1,6})? or U+[hex?]{1,6}.
// Uses uppercase hex only, matching CSS spec. Returns 0 if invalid.
func (s *Scanner) scanUnicodeRangeLen() int {
	pos := s.pos
	if pos+2 >= len(s.input) {
		return 0
	}
	if (s.input[pos] != 'U' && s.input[pos] != 'u') || s.input[pos+1] != '+' {
		return 0
	}
	pos += 2

	if pos >= len(s.input) || (!isUpperHex(s.input[pos]) && s.input[pos] != '?') {
		return 0
	}

	// Consume hex digits and ? marks (up to 6 total).
	count := 0
	hasQuestion := false
	for count < 6 && pos < len(s.input) {
		c := s.input[pos]
		if isUpperHex(c) && !hasQuestion {
			pos++
			count++
		} else if c == '?' {
			pos++
			count++
			hasQuestion = true
		} else {
			break
		}
	}

	// If we had question marks, no range suffix allowed.
	if hasQuestion {
		return pos - s.pos
	}

	// Optional range: -hex{1,6}
	if pos < len(s.input) && s.input[pos] == '-' {
		rangeStart := pos
		pos++
		rangeCount := 0
		for rangeCount < 6 && pos < len(s.input) && isUpperHex(s.input[pos]) {
			pos++
			rangeCount++
		}
		if rangeCount == 0 {
			pos = rangeStart // no hex digits after -, back up
		}
	}

	return pos - s.pos
}

// scanFuncBodyLen scans the body of a url(), local(), format(), or tech()
// function. prefixLen is the byte length of the keyword+( prefix (relative
// to s.pos). Returns total byte length and success.
func (s *Scanner) scanFuncBodyLen(prefixLen int) (int, bool) {
	pos := s.pos + prefixLen

	// Skip leading whitespace.
	for pos < len(s.input) && isWhitespace(s.input[pos]) {
		pos++
	}
	if pos >= len(s.input) {
		return 0, false
	}

	// Try quoted string first.
	stringMatched := false
	if s.input[pos] == '"' || s.input[pos] == '\'' {
		n, ok := s.scanStringLen(pos - s.pos)
		if ok {
			pos += n
			stringMatched = true
		}
	}

	if !stringMatched {
		// Scan unquoted urlchars. Rewind to after prefix + whitespace.
		pos = s.pos + prefixLen
		for pos < len(s.input) && isWhitespace(s.input[pos]) {
			pos++
		}
		for pos < len(s.input) {
			c := s.input[pos]
			if c == ')' {
				break
			}
			// Whitespace (except tab) ends urlchar content.
			if c == ' ' || c == '\n' || c == '\r' || c == '\f' {
				break
			}
			// ASCII urlchar check: tab, !, #..~  (excludes " 0x22 and ) 0x29
			// which are caught above).
			if c == '\t' || c == '!' || (c >= '#' && c <= '~') {
				pos++
				continue
			}
			// Escape sequence inside URL.
			if c == '\\' {
				n := s.scanEscapeLen(pos - s.pos)
				if n > 0 {
					pos += n
					continue
				}
				break
			}
			// Non-ASCII: valid urlchar.
			if c >= 0x80 {
				_, w := utf8.DecodeRuneInString(s.input[pos:])
				pos += w
				continue
			}
			break
		}
	}

	// Skip trailing whitespace.
	for pos < len(s.input) && isWhitespace(s.input[pos]) {
		pos++
	}

	// Expect closing paren.
	if pos < len(s.input) && s.input[pos] == ')' {
		return pos + 1 - s.pos, true
	}
	return 0, false
}

// --------------------------------------------------------------------
// Token production
// --------------------------------------------------------------------

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

	input := s.input[s.pos:]
	switch input[0] {
	case '\t', '\n', '\f', '\r', ' ':
		n := s.scanWhitespaceLen(0)
		return s.emitToken(S, input[:n])

	case '"', '\'':
		n, ok := s.scanStringLen(0)
		if ok {
			return s.emitToken(String, input[:n])
		}
		s.err = &Token{Error, "unclosed quotation mark", s.row, s.col}
		return s.err

	case '#':
		n := s.scanNameLen(1)
		if n > 0 {
			return s.emitToken(Hash, input[:1+n])
		}
		return s.emitSimple(Delim, "#")

	case '.':
		if len(input) > 1 && isDigitByte(input[1]) {
			return s.scanNumericToken()
		}
		return s.emitSimple(Delim, ".")

	case '@':
		n := s.scanIdentLen(1)
		if n > 0 {
			return s.emitSimple(AtKeyword, input[:1+n])
		}
		return s.emitSimple(Delim, "@")

	case '+':
		if len(input) > 1 && isDigitByte(input[1]) {
			return s.scanNumericToken()
		}
		if len(input) > 2 && input[1] == '.' && isDigitByte(input[2]) {
			return s.scanNumericToken()
		}
		return s.emitSimple(Delim, "+")

	case '-':
		// Negative number: -42, -.5
		if len(input) > 1 && isDigitByte(input[1]) {
			return s.scanNumericToken()
		}
		if len(input) > 2 && input[1] == '.' && isDigitByte(input[2]) {
			return s.scanNumericToken()
		}
		// CDC: -->
		if len(input) >= 3 && input[1] == '-' && input[2] == '>' {
			return s.emitSimple(CDC, "-->")
		}
		// Ident or custom property: -webkit, --my-var
		n := s.scanIdentLen(0)
		if n > 0 {
			return s.scanIdentLikeToken(n)
		}
		return s.emitSimple(Delim, "-")

	case '/':
		if len(input) > 1 && input[1] == '*' {
			n, ok := s.scanCommentLen()
			if ok {
				return s.emitToken(Comment, input[:n])
			}
			s.err = &Token{Error, "unclosed comment", s.row, s.col}
			return s.err
		}
		return s.emitSimple(Delim, "/")

	case '\\':
		// Start of escape → ident.
		if s.scanEscapeLen(0) > 0 {
			n := s.scanIdentLen(0)
			if n > 0 {
				return s.scanIdentLikeToken(n)
			}
		}
		return s.emitSimple(Delim, "\\")

	case '~':
		return s.emitPrefixOrChar(Includes, "~=")
	case '|':
		return s.emitPrefixOrChar(DashMatch, "|=")
	case '^':
		return s.emitPrefixOrChar(PrefixMatch, "^=")
	case '$':
		return s.emitPrefixOrChar(SuffixMatch, "$=")
	case '*':
		return s.emitPrefixOrChar(SubstringMatch, "*=")
	case '<':
		return s.emitPrefixOrChar(CDO, "<!--")

	case ':', ',', ';', '%', '&', '=', '>', '(', ')', '[', ']', '{', '}':
		return s.emitSimple(Delim, string(input[0]))
	}

	c := input[0]

	// Digit → numeric token.
	if isDigitByte(c) {
		return s.scanNumericToken()
	}

	// Unicode range: U+xxxx (uppercase hex only per spec).
	if (c == 'U' || c == 'u') && len(input) > 2 && input[1] == '+' &&
		(isUpperHex(input[2]) || input[2] == '?') {
		n := s.scanUnicodeRangeLen()
		if n > 0 {
			return s.emitToken(UnicodeRange, input[:n])
		}
	}

	// Ident-like tokens: ident, function, url(), local(), format(), tech().
	if isNmStartByte(c) || c >= 0x80 {
		n := s.scanIdentLen(0)
		if n > 0 {
			return s.scanIdentLikeToken(n)
		}
	}

	// Fallback: single-character delimiter.
	r, width := utf8.DecodeRuneInString(input)
	token := &Token{Delim, string(r), s.row, s.col}
	s.col += width
	s.pos += width
	return token
}

// scanNumericToken scans a Number, Percentage, or Dimension token.
func (s *Scanner) scanNumericToken() *Token {
	input := s.input[s.pos:]
	numLen := s.scanNumLen(0)
	if numLen == 0 {
		// Shouldn't happen if called correctly; emit as delimiter.
		r, width := utf8.DecodeRuneInString(input)
		token := &Token{Delim, string(r), s.row, s.col}
		s.col += width
		s.pos += width
		return token
	}

	// Check for percentage.
	if s.pos+numLen < len(s.input) && s.input[s.pos+numLen] == '%' {
		return s.emitToken(Percentage, input[:numLen+1])
	}

	// Check for dimension (number followed by ident unit).
	identLen := s.scanIdentLen(numLen)
	if identLen > 0 {
		return s.emitToken(Dimension, input[:numLen+identLen])
	}

	return s.emitToken(Number, input[:numLen])
}

// scanIdentLikeToken scans an Ident, Function, URI, Local, Format, or Tech
// token. identLen is the pre-computed byte length of the identifier portion.
func (s *Scanner) scanIdentLikeToken(identLen int) *Token {
	input := s.input[s.pos:]

	// Check if followed by '(' → function or special function.
	if s.pos+identLen < len(s.input) && s.input[s.pos+identLen] == '(' {
		name := input[:identLen]
		prefixLen := identLen + 1 // ident + opening paren

		// Special functions (case-insensitive): url(), local(), format(), tech().
		if identLen == 3 && startsWithFold(name, "url") {
			if n, ok := s.scanFuncBodyLen(prefixLen); ok {
				return s.emitToken(URI, input[:n])
			}
		}
		if identLen == 5 && startsWithFold(name, "local") {
			if n, ok := s.scanFuncBodyLen(prefixLen); ok {
				return s.emitToken(Local, input[:n])
			}
		}
		if identLen == 6 && startsWithFold(name, "format") {
			if n, ok := s.scanFuncBodyLen(prefixLen); ok {
				return s.emitToken(Format, input[:n])
			}
		}
		if identLen == 4 && startsWithFold(name, "tech") {
			if n, ok := s.scanFuncBodyLen(prefixLen); ok {
				return s.emitToken(Tech, input[:n])
			}
		}

		// Generic function.
		return s.emitToken(Function, input[:prefixLen])
	}

	return s.emitToken(Ident, input[:identLen])
}

// --------------------------------------------------------------------
// Position tracking and token emission
// --------------------------------------------------------------------

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
	token.normalize()
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
	token.normalize()
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
