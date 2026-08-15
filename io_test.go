package tlnio_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	tlnio "github.com/opentalon/tln-io"
)

func TestWriteVariants(t *testing.T) {
	var out bytes.Buffer
	r := tlnio.New(tlnio.WithWriter(&out))
	ctx := context.Background()

	got, err := r.Call(ctx, "io", "write", map[string]any{"text": "hello"})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if got != "hello" {
		t.Errorf("write should return the emitted text, got %v", got)
	}
	if _, err := r.Call(ctx, "io", "nl", nil); err != nil {
		t.Fatalf("nl: %v", err)
	}
	if _, err := r.Call(ctx, "io", "writeln", map[string]any{"text": "world"}); err != nil {
		t.Fatalf("writeln: %v", err)
	}
	if out.String() != "hello\nworld\n" {
		t.Errorf("stream mismatch: %q", out.String())
	}
}

func TestFormatAndValueStringify(t *testing.T) {
	var out bytes.Buffer
	r := tlnio.New(tlnio.WithWriter(&out))
	ctx := context.Background()

	// format with a []any argument list
	if _, err := r.Call(ctx, "io", "format", map[string]any{
		"format": "vehicle %s at %d km",
		"args":   []any{"truck-1", 45000},
	}); err != nil {
		t.Fatalf("format: %v", err)
	}
	// non-string value is stringified
	if _, err := r.Call(ctx, "io", "write", map[string]any{"value": 42}); err != nil {
		t.Fatalf("write value: %v", err)
	}
	if out.String() != "vehicle truck-1 at 45000 km42" {
		t.Errorf("stream mismatch: %q", out.String())
	}
}

func TestReadLineAndEOF(t *testing.T) {
	r := tlnio.New(tlnio.WithReader(strings.NewReader("first\nsecond\n")))
	ctx := context.Background()

	for _, want := range []string{"first", "second"} {
		got, err := r.Call(ctx, "io", "read_line", nil)
		if err != nil {
			t.Fatalf("read_line: %v", err)
		}
		if got != want {
			t.Errorf("want %q, got %v", want, got)
		}
	}
	// third read hits EOF
	if _, err := r.Call(ctx, "io", "read", nil); !errors.Is(err, io.EOF) {
		t.Errorf("want io.EOF at end of input, got %v", err)
	}
}

// stubResolver records the last call so we can prove fallback delegation.
type stubResolver struct{ server, tool string }

func (s *stubResolver) Call(_ context.Context, server, tool string, _ map[string]any) (any, error) {
	s.server, s.tool = server, tool
	return "delegated", nil
}

func TestFallbackDelegatesUnknownServer(t *testing.T) {
	stub := &stubResolver{}
	r := tlnio.New(tlnio.WithFallback(stub))
	ctx := context.Background()

	got, err := r.Call(ctx, "inventory", "lookup", map[string]any{"id": 1})
	if err != nil {
		t.Fatalf("fallback: %v", err)
	}
	if got != "delegated" || stub.server != "inventory" || stub.tool != "lookup" {
		t.Errorf("call was not delegated to fallback: got=%v stub=%+v", got, stub)
	}
}

func TestUnknownServerWithoutFallbackErrors(t *testing.T) {
	r := tlnio.New()
	if _, err := r.Call(context.Background(), "nope", "write", nil); err == nil {
		t.Fatal("expected error for unknown server with no fallback")
	}
}

func TestUnknownToolErrors(t *testing.T) {
	r := tlnio.New(tlnio.WithWriter(&bytes.Buffer{}))
	if _, err := r.Call(context.Background(), "io", "teleport", nil); err == nil {
		t.Fatal("expected error for unknown tool")
	}
}
