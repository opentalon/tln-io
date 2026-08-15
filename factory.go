package tlnio

import (
	"fmt"
	"os"

	"github.com/opentalon/tln-language/pkg/tln"
)

// Factory builds an io resolver from a connector's config, so io-tln can be
// loaded by name from a `mod.tln` / connector (ADR 0012, ADR 0013):
//
//	connector "audit" via io { path "/var/log/tln/audit.log" }
//	connector "errs"  via io { stream "stderr" }
//	connector "io"    via io { }                                # stdout
//
// The returned resolver answers the connector's own server name, so the
// program calls it as `tool "<that name>" "writeln" { … }`.
func Factory(spec tln.ConnectorSpec) (tln.ToolResolver, error) {
	opts := []Option{WithServerName(spec.Name)}
	switch {
	case spec.Config["path"] != "":
		f, err := os.OpenFile(spec.Config["path"], os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, fmt.Errorf("tln-io: open %q: %w", spec.Config["path"], err)
		}
		opts = append(opts, WithWriter(f))
	case spec.Config["stream"] == "stderr":
		opts = append(opts, WithWriter(os.Stderr))
	}
	return New(opts...), nil
}

// Factory satisfies tln.PluginFactory.
var _ tln.PluginFactory = Factory
