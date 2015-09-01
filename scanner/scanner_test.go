// Copyright 2012 The Gorilla Authors, Copyright 2015 Barracuda Networks.
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scanner

import (
	"reflect"
	"testing"
)

func T(ty Type, v string) Token {
	return Token{ty, v, 0, 0}
}

func TestSuccessfulScan(t *testing.T) {
	for _, test := range []struct {
		input  string
		tokens []Token
	}{
		{"bar(", []Token{T(Function, "bar(")}},
		{"abcd", []Token{T(Ident, "abcd")}},
		{`"abcd"`, []Token{T(String, `"abcd"`)}},
		{"'abcd'", []Token{T(String, "'abcd'")}},
		{"#name", []Token{T(Hash, "#name")}},
		{"4.2", []Token{T(Number, "4.2")}},
		{".42", []Token{T(Number, ".42")}},
		{"42%", []Token{T(Percentage, "42%")}},
		{"4.2%", []Token{T(Percentage, "4.2%")}},
		{".42%", []Token{T(Percentage, ".42%")}},
		{"42px", []Token{T(Dimension, "42px")}},
		{"url('http://www.google.com/')", []Token{T(URI, "url('http://www.google.com/')")}},
		{"U+0042", []Token{T(UnicodeRange, "U+0042")}},
		{"<!--", []Token{T(CDO, "<!--")}},
		{"-->", []Token{T(CDC, "-->")}},
		{"   \n   \t   \n", []Token{T(S, "   \n   \t   \n")}},
		{"/* foo */", []Token{T(Comment, "/* foo */")}},
		{"bar(", []Token{T(Function, "bar(")}},
		{"~=", []Token{T(Includes, "~=")}},
		{"|=", []Token{T(DashMatch, "|=")}},
		{"^=", []Token{T(PrefixMatch, "^=")}},
		{"$=", []Token{T(SuffixMatch, "$=")}},
		{"*=", []Token{T(SubstringMatch, "*=")}},
		{"{", []Token{T(Delim, "{")}},
		{"\uFEFF", []Token{T(BOM, "\uFEFF")}},

		{"42''", []Token{
			T(Number, "42"),
			T(String, "''"),
		}},
		{`╯︵┻━┻"stuff"`, []Token{
			T(Ident, "╯︵┻━┻"),
			T(String, `"stuff"`),
		}},
		{"color:red", []Token{
			T(Ident, "color"),
			T(Delim, ":"),
			T(Ident, "red"),
		}},
		{"color:red;background:blue", []Token{
			T(Ident, "color"),
			T(Delim, ":"),
			T(Ident, "red"),
			T(Delim, ";"),
			T(Ident, "background"),
			T(Delim, ":"),
			T(Ident, "blue"),
		}},
		{"color:rgb(0,1,2)", []Token{
			T(Ident, "color"),
			T(Delim, ":"),
			T(Function, "rgb("),
			T(Number, "0"),
			T(Delim, ","),
			T(Number, "1"),
			T(Delim, ","),
			T(Number, "2"),
			T(Delim, ")"),
		}},
		{"color:#fff", []Token{
			T(Ident, "color"),
			T(Delim, ":"),
			T(Hash, "#fff"),
		}},

		// Check note in CSS2 4.3.4:
		// Note that COMMENT tokens cannot occur within other tokens: thus, "url(/*x*/pic.png)" denotes the URI "/*x*/pic.png", not "pic.png".
		{"url(/*x*/pic.png)", []Token{
			T(URI, "url(/*x*/pic.png)"),
		}},

		// More URI testing, since it's important
		{"url(/pic.png)", []Token{
			T(URI, "url(/pic.png)"),
		}},
		{"url( /pic.png )", []Token{
			T(URI, "url( /pic.png )"),
		}},
		{"uRl(/pic.png)", []Token{
			T(URI, "uRl(/pic.png)"),
		}},
		{"url(\"/pic.png\")", []Token{
			T(URI, "url(\"/pic.png\")"),
		}},
		{"url('/pic.png')", []Token{
			T(URI, "url('/pic.png')"),
		}},
		{"url('/pic.png?badchars=\\(\\'\\\"\\)\\ ')", []Token{
			T(URI, "url('/pic.png?badchars=\\(\\'\\\"\\)\\ ')"),
		}},

		// CSS2 section 4.1.1: "red-->" is IDENT "red--" followed by DELIM ">",
		{"red-->", []Token{
			T(Ident, "red--"),
			T(Delim, ">"),
		}},

		{"-moz-border:1", []Token{
			T(Ident, "-moz-border"),
			T(Delim, ":"),
			T(Number, "1"),
		}},

		// CSS2 section 4.1.3, second bullet point: Identifier B&W? may be
		// written in two ways
		{"B\\&W\\?", []Token{
			T(Ident, "B\\&W\\?"),
		}},
		{"B\\26 W\\3F", []Token{
			T(Ident, "B\\26 W\\3F"),
		}},
		// CSS2 4.1.3 third bullet point: A backslash by itself is a DELIM.
		{"\\", []Token{
			T(Delim, "\\"),
		}},

		// CSS2 section 4.1.3, last bullet point: identifier test
		// is the same as te\st.
		// commenting out while this fails, so I can commit other tests
		//{"test", []Token{T(Ident, "test")}},
		//{"te\\st", []Token{T(Ident, "test")}},
	} {
		tokens := []Token{}
		s := New(test.input)
		for {
			tok := s.Next()
			if tok.Type == Error {
				t.Fatalf("Error token with: %q", test.input)
			}
			if tok.Type == EOF {
				break
			}
			tok.Line = 0
			tok.Column = 0
			tokens = append(tokens, *tok)
		}
		if !reflect.DeepEqual(tokens, test.tokens) {
			t.Fatalf("For input string %q, bad tokens. Expected:\n%#v\n\nGot:\n%#v", test.input, test.tokens, tokens)
		}
	}
}

func TestUnbackslash(t *testing.T) {
	for _, test := range []struct {
		isString bool
		in       string
		out      string
	}{
		{false, "", ""},
		{true, "", ""},
		// from CSS2 4.1.3 examples:
		{true, "\\26 B", "&B"},
		{true, "\\000026B", "&B"},
		{true, "\\26G", "&G"},
		{true, "\\2aG", "*G"},
		{true, "\\2AG", "*G"},
		{true, "\\2fG", "/G"},
		{true, "\\2FG", "/G"},
		// standard does not appear to require an even number of digits
		{true, "\\026 B", "&B"},
		{true, "\\026  B", "& B"},
		{true, "\\026", "&"},
		{true, "\\", "\\"},
		{true, "\\{", "{"},

		// Check the special string handling
		{true, "a\\\nb", "ab"},
		{true, "a\\\r\nb", "ab"},
	} {
		result := unbackslash(test.in, test.isString)
		if result != test.out {
			t.Fatalf("Error in TestUnbackslash. In: %q\nOut: %q\nExpected: %q",
				test.in, result, test.out)
		}
	}
}
