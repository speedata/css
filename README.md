css
===

A CSS3 tokenizer.

This is gratefully forked from the [Gorilla CSS
scanner](http://www.gorillatoolkit.org/pkg/css/scanner), and had
significant and __BACKWARDS-INCOMPATIBLE__ changes applied to it.

In particular, the tokens emitted by this scanner are
post-processed into their "actual" value... that is, the CSS identifiers
`test` and `te\st` will both yield an Ident token containing `test`.
The URL token will contain the literal URL, with the CSS encoding processed
away. Etc. Code to correctly emit legal tokens has also been added.

I've also taken the liberty of exporting the `TokenType`, which turns out
to be pretty useful for external processors. To reduce code stuttering, the
Tokens have been renamed to remove the `Token` prefix, and `TokenChar` is
now `TokenDelim`, as that is what CSS calls it.

As of this writing, my personal focus has been on using this to scan
HTML-style style tags as correctly as possible. I can't vouch for whether
this scans CSS itself any better.

I intend in the spirit of open source to offer a PR when this is done back
to the original GitHub project, and I expect it to be rejected for being
too large a backwards-incompatible change, equally in the spirit of open
source. So I suppose if you need what this package is doing, but need to
submit a fix, you can PR it here.