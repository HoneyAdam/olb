package yaml

import (
	"testing"
)

func TestNestedMultiSequence(t *testing.T) {
	data := []byte(`listeners:
  - name: http
    address: ":8080"
    routes:
      - path: /
        pool: web
  - name: https
    address: ":8443"
    tls: true
    routes:
      - path: /
        pool: web
pools:
  - name: web
    algorithm: round_robin
`)

	type Route struct {
		Path string `yaml:"path"`
		Pool string `yaml:"pool"`
	}

	type Listener struct {
		Name    string  `yaml:"name"`
		Address string  `yaml:"address"`
		TLS     bool    `yaml:"tls"`
		Routes  []Route `yaml:"routes"`
	}

	type Pool struct {
		Name      string `yaml:"name"`
		Algorithm string `yaml:"algorithm"`
	}

	type Config struct {
		Listeners []Listener `yaml:"listeners"`
		Pools     []Pool     `yaml:"pools"`
	}

	// First, debug the token stream
	tokens, err := Tokenize(string(data))
	if err != nil {
		t.Fatalf("Tokenize error: %v", err)
	}
	t.Log("=== TOKEN STREAM ===")
	for i, tok := range tokens {
		t.Logf("  %3d: %-10s value=%-20q line=%d col=%d", i, tok.Type, tok.Value, tok.Line, tok.Col)
	}

	// Then debug the AST
	node, err := Parse(string(data))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	t.Log("=== AST ===")
	printNode(t, node, 0)

	// Now test decoding
	var cfg Config
	err = Unmarshal(data, &cfg)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	t.Logf("Listeners: %d, Pools: %d", len(cfg.Listeners), len(cfg.Pools))
	for i, l := range cfg.Listeners {
		t.Logf("  Listener[%d]: name=%s address=%s tls=%v routes=%d", i, l.Name, l.Address, l.TLS, len(l.Routes))
	}
	for i, p := range cfg.Pools {
		t.Logf("  Pool[%d]: name=%s algorithm=%s", i, p.Name, p.Algorithm)
	}

	if len(cfg.Listeners) != 2 {
		t.Errorf("Expected 2 listeners, got %d", len(cfg.Listeners))
	}
	if len(cfg.Pools) != 1 {
		t.Errorf("Expected 1 pool, got %d", len(cfg.Pools))
	}
}
