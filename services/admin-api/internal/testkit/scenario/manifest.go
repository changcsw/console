// Package scenario loads and runs backend API scenario manifests
// against an in-process http.Handler.
package scenario

import (
	"errors"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

type Manifest struct {
	Module string `yaml:"module"`
	Cases  []Case `yaml:"cases"`
}

type Case struct {
	Name      string  `yaml:"name"`
	Dimension string  `yaml:"dimension"`
	Request   Request `yaml:"request"`
	Expect    Expect  `yaml:"expect"`
}

type Request struct {
	Method  string            `yaml:"method"`
	Path    string            `yaml:"path"`
	Headers map[string]string `yaml:"headers"`
	Body    map[string]any    `yaml:"body"`
}

type Expect struct {
	Status       int            `yaml:"status"`
	JSONContains map[string]any `yaml:"jsonContains"`
}

func LoadManifest(path string) (Manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read %s: %w", path, err)
	}
	defer f.Close()

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)

	var m Manifest
	if err := dec.Decode(&m); err != nil && !errors.Is(err, io.EOF) {
		return Manifest{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if m.Module == "" {
		return Manifest{}, fmt.Errorf("%s: missing module", path)
	}

	seen := make(map[string]struct{}, len(m.Cases))
	for i, c := range m.Cases {
		if c.Name == "" {
			return Manifest{}, fmt.Errorf("%s: case[%d] missing name", path, i)
		}
		if _, dup := seen[c.Name]; dup {
			return Manifest{}, fmt.Errorf("%s: duplicate case name %q", path, c.Name)
		}
		seen[c.Name] = struct{}{}
		if c.Request.Method == "" || c.Request.Path == "" {
			return Manifest{}, fmt.Errorf("%s: case %q missing request method/path", path, c.Name)
		}
		if c.Expect.Status == 0 {
			return Manifest{}, fmt.Errorf("%s: case %q missing expect.status", path, c.Name)
		}
	}
	return m, nil
}
