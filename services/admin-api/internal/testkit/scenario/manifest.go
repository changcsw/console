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
	// RequiresDB 标记该 case 需要真实 PG + 全装配（ready=true）才能运行。
	// 进程内 httptest harness（无 DSN，降级 ready=false）会跳过此类 case，
	// 仅校验 manifest 可解析；连库 harness 落地后改由其执行。
	RequiresDB bool `yaml:"requiresDB"`
	// Fixture 声明该 case 所需的 fixtures 子集（如 common/auth/base）。
	// 当前进程内 harness 不消费，仅作前向声明（见 03-testing §4.1/§7）。
	Fixture string `yaml:"fixture"`
	// Auth 声明该 case 的鉴权身份（如 {role: super_admin}），连库 harness 用以签发令牌。
	Auth map[string]string `yaml:"auth"`
	// Note 人类可读说明（如「manifest 已声明但运行需 PG」）。
	Note string `yaml:"note"`
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
