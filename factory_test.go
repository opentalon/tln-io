package tlnio_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tlnio "github.com/opentalon/io-tln"
	"github.com/opentalon/tln-language/pkg/tln"
)

func TestFactory_SatisfiesPluginFactory(t *testing.T) {
	var _ tln.PluginFactory = tlnio.Factory
}

// TestFactory_FileSink: a `path` connector appends writes to that file, and the
// resolver answers the connector's own name.
func TestFactory_FileSink(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	r, err := tlnio.Factory(tln.ConnectorSpec{
		Name: "audit", Plugin: "io", Config: map[string]string{"path": path},
	})
	if err != nil {
		t.Fatalf("factory: %v", err)
	}
	if _, err := r.Call(context.Background(), "audit", "writeln", map[string]any{"text": "overdue: 501"}); err != nil {
		t.Fatalf("call: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sink: %v", err)
	}
	if !strings.Contains(string(b), "overdue: 501") {
		t.Fatalf("file sink missing the write, got %q", string(b))
	}
}

// TestFactory_AnswersOnlyItsConnector: the resolver serves the connector's name,
// not arbitrary servers.
func TestFactory_AnswersOnlyItsConnector(t *testing.T) {
	r, _ := tlnio.Factory(tln.ConnectorSpec{
		Name: "audit", Config: map[string]string{"path": filepath.Join(t.TempDir(), "x")},
	})
	if _, err := r.Call(context.Background(), "somewhere-else", "writeln", map[string]any{"text": "x"}); err == nil {
		t.Fatal("resolver must only answer its own connector name")
	}
}

// TestFactory_StderrStream builds a stderr-backed resolver without error.
func TestFactory_StderrStream(t *testing.T) {
	if _, err := tlnio.Factory(tln.ConnectorSpec{Name: "errs", Config: map[string]string{"stream": "stderr"}}); err != nil {
		t.Fatalf("stderr connector: %v", err)
	}
}
