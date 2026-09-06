# mdv

Fork of [rsc.io/cmd/mdweb](https://pkg.go.dev/rsc.io/cmd/mdweb).
Serves Markdown from a directory as HTML on localhost.

`*-talk.md` files stay slide presentations (one `#` heading per slide,
speaker notes after `---`). Other `.md` files get a reading page
(paper / sepia / dark, type size, TOC, file list). Passing a path
opens it in the browser.

## Install

Go 1.24 or later. `$GOPATH/bin` (or `$HOME/go/bin`) must be on `PATH`.

```
go install github.com/drgo/mdv@latest
```

From a checkout:

```
git clone https://github.com/DrGo/mdv
cd mdv
go install .
```

## Usage

```
mdv [-a addr] [-r root]
mdv [-a addr] path
```

`-a` sets the listen address (default `localhost:8780`).
`-r` sets the directory to serve (default `.`).

If `path` is given, mdv opens it in the browser. The serve root stays
the current directory (or `-r`) when `path` is under it; a path outside
the root becomes the new root.

Press `?` in a talk for the presentation keys.
