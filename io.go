// Package tlnio implements tln's ToolResolver for input/output effects, so a
// tln host can perform write / print / read without I/O living in the language
// core. It is injected exactly like tln-mcp — core stays effect-free; every IO
// edge is a plugin:
//
//	r := tlnio.New()                       // stdout / stderr / stdin
//	tln.Run(ctx, prog, tln.WithToolResolver(r))
//
// It answers one server (default "io"). To run it alongside another resolver
// such as tln-mcp, chain them — unknown servers fall through:
//
//	r := tlnio.New(tlnio.WithFallback(mcp)) // "io" here, everything else → mcp
//	tln.Run(ctx, prog, tln.WithToolResolver(r))
//
// This is also the landing point for ported Prolog I/O (write/1, nl/0, format/2,
// read/1): those goals lower to Call(server="io", tool=…) instead of running on
// the tln-prolog engine.
package tlnio

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/opentalon/tln-language/pkg/tln"
)

// DefaultServer is the server name this resolver answers unless overridden with
// [WithServerName]. A program references it as `mcp "io" "write" { … }`.
const DefaultServer = "io"

// Resolver performs I/O effects for tln. It maps (server, tool, args) triples to
// writes and reads over configurable streams (default os.Stdout/os.Stderr/
// os.Stdin). Safe for concurrent use — every call is serialized so interleaved
// writes stay intact.
type Resolver struct {
	server   string
	mu       sync.Mutex
	out      io.Writer
	errOut   io.Writer
	in       *bufio.Reader
	fallback tln.ToolResolver
}

// Option configures a Resolver.
type Option func(*Resolver)

// WithServerName overrides the server name this resolver answers (default
// [DefaultServer]).
func WithServerName(name string) Option { return func(r *Resolver) { r.server = name } }

// WithWriter sets the destination for stdout-style tools (write/writeln/nl/
// print/format). Defaults to os.Stdout.
func WithWriter(w io.Writer) Option { return func(r *Resolver) { r.out = w } }

// WithErrorWriter sets the destination for the error-stream tools
// (write_err/eprintln). Defaults to os.Stderr.
func WithErrorWriter(w io.Writer) Option { return func(r *Resolver) { r.errOut = w } }

// WithReader sets the input stream read/read_line consume. Defaults to
// os.Stdin.
func WithReader(rd io.Reader) Option { return func(r *Resolver) { r.in = bufio.NewReader(rd) } }

// WithFallback delegates any call whose server is not this resolver's to next,
// so tlnio composes with another ToolResolver (e.g. tln-mcp) behind a single
// [tln.WithToolResolver].
func WithFallback(next tln.ToolResolver) Option { return func(r *Resolver) { r.fallback = next } }

// New builds a Resolver. With no options it reads/writes the process streams.
func New(opts ...Option) *Resolver {
	r := &Resolver{
		server: DefaultServer,
		out:    os.Stdout,
		errOut: os.Stderr,
		in:     bufio.NewReader(os.Stdin),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Compile-time proof the plugin satisfies tln's host callback contract.
var _ tln.ToolResolver = (*Resolver)(nil)

// Call implements tln.ToolResolver. It handles these tools on its server:
//
//	write / print      → args["text"] (or "value") to stdout, no newline
//	writeln / println  → the text plus a trailing newline
//	nl                 → a single newline
//	write_err / eprintln → to the error stream
//	format             → args["format"] with args["args"] ([]any), printf-style
//	read / read_line   → read one line from stdin, returned trimmed of its newline
//
// Every write tool returns the exact string emitted; read returns the line.
// Calls for a different server are delegated to the fallback if one is set,
// otherwise reported as an error so the engine can apply the call's on_error
// policy.
func (r *Resolver) Call(ctx context.Context, server, tool string, args map[string]any) (any, error) {
	if server != r.server {
		if r.fallback != nil {
			return r.fallback.Call(ctx, server, tool, args)
		}
		return nil, fmt.Errorf("tln-io: unknown server %q", server)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	switch tool {
	case "write", "print":
		return r.emit(r.out, text(args))
	case "writeln", "println":
		return r.emit(r.out, text(args)+"\n")
	case "nl":
		return r.emit(r.out, "\n")
	case "write_err", "eprintln":
		return r.emit(r.errOut, text(args)+"\n")
	case "format":
		s, err := format(args)
		if err != nil {
			return nil, err
		}
		return r.emit(r.out, s)
	case "read", "read_line":
		return r.readLine()
	default:
		return nil, fmt.Errorf("tln-io: unknown tool %q on server %q", tool, server)
	}
}

// emit writes s to w and returns it, so callers can bind the emitted text.
func (r *Resolver) emit(w io.Writer, s string) (any, error) {
	if _, err := io.WriteString(w, s); err != nil {
		return nil, fmt.Errorf("tln-io: write: %w", err)
	}
	return s, nil
}

// readLine returns the next input line without its trailing newline. A clean
// end of input (EOF with nothing buffered) is reported as io.EOF so the engine
// can treat it like Prolog's end_of_file.
func (r *Resolver) readLine() (any, error) {
	line, err := r.in.ReadString('\n')
	line = strings.TrimRight(line, "\r\n")
	switch {
	case err == io.EOF && line == "":
		return nil, io.EOF
	case err != nil && err != io.EOF:
		return nil, fmt.Errorf("tln-io: read: %w", err)
	default:
		return line, nil
	}
}

// text extracts the string to emit: prefer args["text"], else stringify
// args["value"], else empty.
func text(args map[string]any) string {
	if t, ok := args["text"]; ok {
		return toString(t)
	}
	if v, ok := args["value"]; ok {
		return toString(v)
	}
	return ""
}

// format renders args["format"] (a string) with args["args"] (a []any),
// printf-style.
func format(args map[string]any) (string, error) {
	f, ok := args["format"].(string)
	if !ok {
		return "", fmt.Errorf("tln-io: format requires a string \"format\" argument")
	}
	var fargs []any
	switch a := args["args"].(type) {
	case nil:
		// no arguments
	case []any:
		fargs = a
	default:
		fargs = []any{a} // a lone scalar is fine
	}
	return fmt.Sprintf(f, fargs...), nil
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
