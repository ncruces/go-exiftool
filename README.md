# go-exiftool: ExifTool in Go (zeroperl + wazero)

[![Go Reference](https://pkg.go.dev/badge/github.com/lbe/go-exiftool.svg)](https://pkg.go.dev/github.com/lbe/go-exiftool)

Go library that runs **[ExifTool](https://exiftool.org/)** in-process: embedded **Perl** via **[zeroperl](https://github.com/uswriting/zeroperl)** (Perl compiled to **WebAssembly**), executed with **[wazero](https://wazero.io/)**. You do **not** need a system `exiftool` binary or a separate Perl install.

Upstream ExifTool (Phil Harvey) and metadata: [exiftool.org](https://exiftool.org/), [exiftool/exiftool](https://github.com/exiftool/exiftool).

## Why this shape

- **Self-contained** — Perl stdlib, ExifTool, and the interpreter ship inside the module (`embed/`).
- **Same behavior everywhere** — one WASM stack on Linux, macOS, Windows, etc.
- **Trade-off** — cold start is heavy (on the order of **~7–10 seconds** to compile/instantiate WASM and init Perl). For many operations, use **`NewServer`** and **`Server.Command`** so ExifTool stays open (`-stay_open`) and work is amortized.

## Requirements

- **Go 1.26+**
- Dependency: `github.com/tetratelabs/wazero` (see `go.mod`).

## Usage

```go
// One-shot: paths are relative to the process working directory.
out, err := exiftool.Command(nil, "-json", "photo.jpg")

// Explicit tree at /work (e.g. tests or isolated FS).
out, err := exiftool.Run(ctx, os.DirFS("/path/to/photos"), "-json", "/work/photo.jpg")

// Batch: one WASM/Perl startup, many commands.
e, err := exiftool.NewServer("-fast")
if err != nil { /* ... */ }
defer e.Shutdown()
out, err = e.Command("-Artist", "photo.jpg")
```

Package docs, API details, and filesystem semantics (`Command` vs `Run`, temp dir, `Arg1` / `Config`): run **`go doc -all`** or open [pkg.go.dev](https://pkg.go.dev/github.com/lbe/go-exiftool).

## Development

### Tests

```bash
go test ./... -timeout 10m   # includes WASM tests; allow several minutes
```

### Refreshing `embed/` from zeroperl

The WASM module, Perl install prefix, and minified ExifTool script come from **[zeroperl](https://github.com/uswriting/zeroperl)**. Follow that repository’s **Build** section (Docker or Apple Container): build the image, run the container, and copy **`/artifacts`** into a host directory (as shown there, e.g. `./output/`).

From the build output directory, install these into **this** repo under `embed/`:

| Artifact from zeroperl output | Path in go-exiftool |
|-------------------------------|---------------------|
| `zeroperl.wasm` | `embed/zeroperl.wasm` |
| `perl-wasi-prefix/` (entire tree) | `embed/perl-wasi-prefix/` |
| `exiftool.min.pl` | `embed/exiftool.min.pl` |

Use the **`zeroperl.wasm`** artifact (reactor **with** asyncify) unless you intentionally switch runtimes. If you disable ExifTool in zeroperl (`BUILD_EXIFTOOL=false`), you must supply `exiftool.min.pl` yourself.

zeroperl’s README also documents **build arguments** (`PERL_VERSION`, `EXIFTOOL_VERSION`, `BUILD_EXIFTOOL`, memory/stack, etc.). If you change **`PERL_VERSION`**, update the hard-coded **`PERL5LIB`** paths in `exiftool.go` and `server.go` so the version segment (e.g. `5.42.0`) matches the tree under `embed/perl-wasi-prefix/lib/`.

## License

See [LICENSE](LICENSE). ExifTool and embedded components carry their own licenses under `embed/`.
