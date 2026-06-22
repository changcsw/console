package scenario

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadManifestParsesCases(t *testing.T) {
	dir := t.TempDir()
	yaml := `module: smoke
cases:
  - name: healthz_ok
    dimension: S1
    request:
      method: GET
      path: /healthz
    expect:
      status: 200
      jsonContains:
        status: ok
`
	path := filepath.Join(dir, "smoke.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}

	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if m.Module != "smoke" || len(m.Cases) != 1 {
		t.Fatalf("unexpected manifest: %+v", m)
	}
	c := m.Cases[0]
	if c.Name != "healthz_ok" || c.Request.Method != "GET" || c.Request.Path != "/healthz" {
		t.Fatalf("unexpected case: %+v", c)
	}
	if c.Expect.Status != 200 || c.Expect.JSONContains["status"] != "ok" {
		t.Fatalf("unexpected expect: %+v", c.Expect)
	}
}

func TestLoadManifestRejectsEmptyCaseName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte("module: x\ncases:\n  - request:\n      method: GET\n      path: /\n    expect:\n      status: 200\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadManifest(path); err == nil {
		t.Fatal("expected validation error for empty case name")
	}
}

func TestLoadManifestValidationErrors(t *testing.T) {
	bodies := map[string]string{
		"missing module": "cases:\n  - name: a\n    request:\n      method: GET\n      path: /\n    expect:\n      status: 200\n",
		"missing method": "module: m\ncases:\n  - name: a\n    request:\n      path: /\n    expect:\n      status: 200\n",
		"missing path":   "module: m\ncases:\n  - name: a\n    request:\n      method: GET\n    expect:\n      status: 200\n",
		"missing status": "module: m\ncases:\n  - name: a\n    request:\n      method: GET\n      path: /\n    expect: {}\n",
		"duplicate name": "module: m\ncases:\n  - name: a\n    request:\n      method: GET\n      path: /\n    expect:\n      status: 200\n  - name: a\n    request:\n      method: GET\n      path: /x\n    expect:\n      status: 200\n",
		"unknown field":  "module: m\ncases:\n  - name: a\n    reqeust:\n      method: GET\n      path: /\n    expect:\n      status: 200\n",
	}
	for name, body := range bodies {
		body := body
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			p := filepath.Join(dir, "m.yaml")
			if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadManifest(p); err == nil {
				t.Fatalf("expected error for %q", name)
			}
		})
	}
}
