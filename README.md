# io-tln

[![CI](https://github.com/opentalon/io-tln/actions/workflows/ci.yml/badge.svg)](https://github.com/opentalon/io-tln/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

**Input/output plugin for [tln](https://github.com/opentalon/tln-language) — `write` / `print` / `read` as an engine plugin, injected exactly like [tln-mcp](https://github.com/opentalon/tln-mcp).**

tln core is effect-free: it decides *which* effects fire and hands them back as
data; performing them is a plugin's job. `io-tln` is the plugin for the most
basic effect — I/O. You write ordinary tln, calling the `io` server the same way
you'd call any tool:

```tln
detect "Overdue for service" {
  for records where type == "vehicle"
    and attr "km" > attr "last_service_km" + 20000
  flag matching items
  remediate {
    mcp "io" "writeln" { text "overdue: {item.id} at {attr.km} km" }
    if attr "priority" == "CRITICAL" {
      mcp "io" "eprintln" { text "CRITICAL: {item.id}" }
    }
  }
}
```

`format` is printf-style, and reads come back as a bound value:

```tln
mcp "io" "format" { format "vehicle %s at %d km" args [attr "id", attr "km"] }
mcp "io" "writeln" { text "loaded config" }
```

The host just installs the plugin once — same one-line wiring as tln-mcp:

```go
import (
    "github.com/opentalon/tln-language/pkg/tln"
    tlnio "github.com/opentalon/io-tln"
)

tln.Run(ctx, prog, tln.WithToolResolver(tlnio.New()))   // stdout / stderr / stdin
```

## Tools

| Tool | Effect | Returns |
|------|--------|---------|
| `write` / `print` | `args.text` (or `args.value`) to stdout, no newline | the emitted string |
| `writeln` / `println` | text + trailing newline | the emitted string |
| `nl` | a single newline | `"\n"` |
| `write_err` / `eprintln` | to the error stream | the emitted string |
| `format` | `args.format` with `args.args` (`[]any`), printf-style | the rendered string |
| `read` / `read_line` | one line from stdin, newline trimmed | the line; clean EOF → `io.EOF` |

## Composing with tln-mcp

A tln host installs one `ToolResolver`. To run I/O **and** MCP behind that single
seam, chain them — `io-tln` handles its own server and delegates the rest:

```go
mcp := tlnmcp.New(tlnmcp.WithServer("inventory", "https://mcp.example.com/rpc"))
r   := tlnio.New(tlnio.WithFallback(mcp))   // "io" → io-tln, everything else → mcp
tln.Run(ctx, prog, tln.WithToolResolver(r))
```

## Configuration

```go
tlnio.New(
    tlnio.WithServerName("io"),   // server name this resolver answers
    tlnio.WithWriter(w),          // stdout stream (default os.Stdout)
    tlnio.WithErrorWriter(we),    // error stream (default os.Stderr)
    tlnio.WithReader(rd),         // input stream (default os.Stdin)
    tlnio.WithFallback(next),     // delegate other servers to another resolver
)
```

Redirect the streams (e.g. `bytes.Buffer`) to capture output in tests, or point
them at files/pipes.

## Where it fits

`io-tln` is a **tool**-shaped plugin, alongside
[`tln-mcp`](https://github.com/opentalon/tln-mcp) — both are `ToolResolver`s.
Core stays a pure language + planner + SPIs; every IO edge is a plugin
([`tln-db`](https://github.com/opentalon/tln-db) = store,
[`tln-asp`](https://github.com/opentalon/tln-asp) = solver,
[`tln-prolog`](https://github.com/opentalon/tln-prolog) = reasoner).

It is also the landing point for **ported Prolog I/O**: `write/1`, `nl/0`,
`format/2`, and `read/1` lower to `io` tool calls here instead of running on the
tln-prolog engine — keeping side effects at the host boundary, out of the pure
resolver.

## License

Apache 2.0 — see [LICENSE](LICENSE).
