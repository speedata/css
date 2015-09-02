// Copyright 2012 The Gorilla Authors, Copyright 2015 Barracuda Networks.
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scanner

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// Type is an integer that identifies the type of the token. Only the types
// defined as variables in the package may be used.
type Type struct {
	t int
}

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
// Scanner flags.
var Error = Type{0}
var EOF = Type{1}

// From now on, only tokens from the CSS specification.
var Ident = Type{2}
var AtKeyword = Type{3}
var String = Type{4}
var Hash = Type{5}
var Number = Type{6}
var Percentage = Type{7}
var Dimension = Type{8}
var URI = Type{9}
var UnicodeRange = Type{10}
var CDO = Type{11}
var CDC = Type{12}
var S = Type{13}
var Comment = Type{14}
var Function = Type{15}
var Includes = Type{16}
var DashMatch = Type{17}
var PrefixMatch = Type{18}
var SuffixMatch = Type{19}
var SubstringMatch = Type{20}
var Delim = Type{21}
var BOM = Type{22}

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

// For those types of tokens that need to have their representation
// normalized to contain the semantic contents of the token, rather than
// the literal contents of the token, this performs that act.
func (t *Token) normalize() {
	switch t.Type {
	case Ident:
		t.Value = unbackslash(t.Value, false)
	case AtKeyword:
		t.Value = unbackslash(t.Value[1:], false)
	case String:
		t.Value = unbackslash(t.Value[1:len(t.Value)-1], true)
	case Hash:
		t.Value = unbackslash(t.Value[1:], false)
	case Percentage:
		t.Value = t.Value[0 : len(t.Value)-1]
	case Dimension:
		t.Value = unbackslash(t.Value, false)
	case CDO:
		t.Value = ""
	case CDC:
		t.Value = ""
	case URI:
		// this is a strict parser; only u, r, l, followed by a paren with
		// no whitespace, is accepted.
		trimmed := strings.TrimSpace(t.Value[4 : len(t.Value)-1])
		if trimmed == "" {
			t.Value = ""
			return
		}
		lastIdx := len(trimmed) - 1
		if trimmed[0] == '\'' && trimmed[lastIdx] == '\'' {
			fmt.Printf("Trimming: %q\n", trimmed)
			trimmed = trimmed[1:lastIdx]
		} else if trimmed[0] == '"' && trimmed[lastIdx] == '"' {
			trimmed = trimmed[1:lastIdx]
		}
		t.Value = unbackslash(trimmed, false)
	case Comment:
		t.Value = t.Value[2 : len(t.Value)-2]
	case Function:
		t.Value = unbackslash(t.Value[0:len(t.Value)-1], false)
	case Includes:
		t.Value = ""
	case DashMatch:
		t.Value = ""
	case PrefixMatch:
		t.Value = ""
	case SuffixMatch:
		t.Value = ""
	case SubstringMatch:
		t.Value = ""
	}
}

func wr(w io.Writer, strs ...string) (err error) {
	for _, str := range strs {
		_, err = w.Write([]byte(str))
		if err != nil {
			return
		}
	}
	return
}

// Emit will write a string representation of the given token to the target
// io.Writer. An error will be returned if you either try to emit Error or
// EOF, or if the Writer returns an error.
//
// Emit will make many small writes to the io.Writer.
//
// Emit assumes you have not set the token's .Value to an invalid value for
// many of these; for instance, if you manually take a Number token and set
// its .Value to "sometext", you will emit something that is not a number.
func (t *Token) Emit(w io.Writer) (err error) {
	switch t.Type {
	case Error:
		return errors.New("can not emit an error token")
	case EOF:
		return errors.New("can not emit an EOF")
	case Ident:
		err = wr(w, backslashifyIdent(t.Value))
	case AtKeyword:
		err = wr(w, "@", backslashifyIdent(t.Value))
	case String:
		err = wr(w, "\"", backslashifyString(t.Value), "\"")
	case Hash:
		err = wr(w, "#", backslashifyIdent(t.Value))
	case Number:
		err = wr(w, t.Value)
	case Percentage:
		err = wr(w, t.Value, "%")
	case Dimension:
		err = wr(w, t.Value)
	case URI:
		err = wr(w, "url('", backslashifyString(t.Value), "')")
	case UnicodeRange:
		err = wr(w, t.Value)
	case CDO:
		err = wr(w, "<!--")
	case CDC:
		err = wr(w, "-->")
	case S:
		err = wr(w, t.Value)
	case Comment:
		err = wr(w, "/*", t.Value, "*/")
	case Function:
		err = wr(w, backslashifyIdent(t.Value), "(")
	case Includes:
		err = wr(w, "~=")
	case DashMatch:
		err = wr(w, "|=")
	case PrefixMatch:
		err = wr(w, "^=")
	case SuffixMatch:
		err = wr(w, "$=")
	case SubstringMatch:
		err = wr(w, "*=")
	case Delim:
		err = wr(w, t.Value)
	case BOM:
		err = wr(w, "\ufeff")
	}

	return
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

func backslashifyString(s string) string {
	res := bytes.NewBuffer(make([]byte, 0, len(s)+32))
	b := []byte(s)
	for {
		r, size := utf8.DecodeRune(b)
		if size == 0 {
			break
		}
		b = b[size:]
		switch {
		case r == '"':
			res.WriteRune('\\')
			res.WriteRune(r)
		case r >= '#':
			res.WriteRune(r)
		case r == '\t' || r == '!':
			res.WriteRune(r)
		default:
			res.WriteRune('\\')
			res.WriteRune(r)
		}
	}
	return res.String()
}

func backslashifyIdent(s string) string {
	res := bytes.NewBuffer(make([]byte, 0, len(s)+32))
	b := []byte(s)
	for {
		r, size := utf8.DecodeRune(b)
		if size == 0 {
			break
		}
		b = b[size:]
		if !(r >= 'a' && r <= 'z') &&
			!(r >= 'A' && r <= 'Z') &&
			r != '_' && r != '-' &&
			r <= 255 {
			res.WriteRune('\\')
			res.WriteRune(r)
		} else {
			res.WriteRune(r)
		}
	}
	return res.String()
}

func isWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f'
}

func isHexChar(c byte) bool {
	return c >= '0' && c <= '9' ||
		c >= 'a' && c <= 'f' ||
		c >= 'A' && c <= 'F'
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

// as mentioned in fromHexChar, by construction, we know this is being
// called only with hex values, and only in quantities that fit into the
// rune type. C&P at your own peril. :)
func decodeHex(in []byte) rune {
	val := rune(0)

	for _, c := range in {
		val = val << 4
		val = val + rune(fromHexChar(c))
	}

	return val
}
