// Copyright 2012 The Gorilla Authors, Copyright 2015 Barracuda Networks,
// Copyright 2020-2026 Patrick Gundlach.
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package scanner tokenizes CSS input following the CSS Syntax specification.

To use it, create a new scanner for a given CSS string and call Next() until
the token returned has type scanner.EOF or scanner.Error:

	s := scanner.New(input)
	for {
		token := s.Next()
		if token.Type == scanner.EOF || token.Type == scanner.Error {
			break
		}
		// Use token.Type, token.Value, token.Line, token.Column
	}

Token values are post-processed to contain semantic content: CSS escapes are
resolved, quotes are stripped from strings, and delimiters are removed from
functions and URLs. Tokens can be re-emitted to valid CSS via token.Emit(w).

Following the CSS specification, an error can only occur when the scanner
finds an unclosed quote or unclosed comment. Everything else is tokenizable
and it is up to a parser to make sense of the token stream.
*/
package scanner
