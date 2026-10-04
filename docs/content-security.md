# Content Markdown and untrusted input

Gx renders Markdown at build time from repository files. The compiler uses
goldmark, so content pages support CommonMark and GFM (SI-12).

Package `gx` holds no Markdown renderer and no Markdown dependency. The
`gx/content` package renders the body of a content entry, and the compiler
does the same work during a build.

An app that renders Markdown from user input must pass it through a
sanitizer first. Gx does not ship one: untrusted Markdown is an app
concern.
