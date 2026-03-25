# css/scanner

A fast CSS3 tokenizer for Go.

This package tokenizes CSS input into a stream of typed tokens (identifiers, strings, numbers, dimensions, URLs, comments, etc.) following the CSS Syntax specification. It is intended to be used by a lexer or parser.

## Origin

Originally based on the [Gorilla CSS scanner](http://www.gorillatoolkit.org/pkg/css/scanner), significantly reworked by [thejerf/css](https://github.com/thejerf/css) (Barracuda Networks), then forked by [speedata](https://github.com/speedata) with further changes:

- CSS Syntax Level 3 support: custom properties (`--my-var`), signed numbers (`-42px`, `+3em`)
- Hand-written scanner replacing all regex-based tokenization (~10x faster)
- Support for `local()`, `format()`, and `tech()` function tokens

## Usage

```go
import "github.com/speedata/css/scanner"

s := scanner.New(input)
for {
    token := s.Next()
    if token.Type == scanner.EOF || token.Type == scanner.Error {
        break
    }
    // token.Type, token.Value, token.Line, token.Column
}
```

## Token types

| Token | Example input | `.Value` |
|-------|--------------|----------|
| `Ident` | `color`, `-webkit-foo`, `--my-var` | `color`, `-webkit-foo`, `--my-var` |
| `Function` | `rgb(` | `rgb` |
| `AtKeyword` | `@media` | `media` |
| `Hash` | `#fff` | `fff` |
| `String` | `"hello"` | `hello` |
| `Number` | `42`, `-3.14`, `+0.5` | `42`, `-3.14`, `+0.5` |
| `Percentage` | `50%` | `50` |
| `Dimension` | `12px`, `-1.5em` | `12px`, `-1.5em` |
| `URI` | `url('bg.png')` | `bg.png` |
| `Local` | `local('Font')` | `Font` |
| `Format` | `format('woff2')` | `woff2` |
| `Tech` | `tech('color-SVG')` | `color-SVG` |
| `UnicodeRange` | `U+0042` | `U+0042` |
| `S` | `   ` | `   ` |
| `Comment` | `/* text */` | ` text ` |
| `Delim` | `:`, `,`, `{` | `:`, `,`, `{` |

Tokens are post-processed to contain semantic values: CSS escapes are resolved, quotes and delimiters are stripped. Tokens can be re-emitted to valid CSS via `token.Emit(w)`.

## Error handling

Following the CSS specification, errors only occur for unclosed quotes or unclosed comments. Everything else is tokenizable; it is up to a parser to make sense of the token stream.

## License

BSD 3-Clause. See [LICENSE](LICENSE) for details.
