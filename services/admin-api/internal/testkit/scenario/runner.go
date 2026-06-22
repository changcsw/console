package scenario

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"
)

type CaseResult struct {
	Name      string
	Dimension string
	Passed    bool
	Message   string
	Status    int
	Duration  time.Duration
}

// RunCase 在进程内对 http.Handler 执行一个 case 并校验断言。
func RunCase(handler http.Handler, c Case) CaseResult {
	start := time.Now()
	res := CaseResult{Name: c.Name, Dimension: c.Dimension}

	var body *bytes.Reader
	if c.Request.Body != nil {
		raw, err := json.Marshal(c.Request.Body)
		if err != nil {
			res.Message = fmt.Sprintf("marshal body: %v", err)
			res.Duration = time.Since(start)
			return res
		}
		body = bytes.NewReader(raw)
	} else {
		body = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(c.Request.Method, c.Request.Path, body)
	if c.Request.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range c.Request.Headers {
		req.Header.Set(k, v)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	res.Status = rec.Code
	res.Duration = time.Since(start)

	if rec.Code != c.Expect.Status {
		res.Message = fmt.Sprintf("status: want %d, got %d (body: %s)", c.Expect.Status, rec.Code, truncate(rec.Body.String()))
		return res
	}

	if len(c.Expect.JSONContains) > 0 {
		var decoded any
		if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
			res.Message = fmt.Sprintf("body not JSON: %v", err)
			return res
		}
		for path, want := range c.Expect.JSONContains {
			got, ok := lookup(decoded, path)
			if !ok {
				res.Message = fmt.Sprintf("json path %q not found", path)
				return res
			}
			if !equalScalar(got, want) {
				res.Message = fmt.Sprintf("json path %q: want %v, got %v", path, want, got)
				return res
			}
		}
	}

	res.Passed = true
	return res
}

func truncate(s string) string {
	const max = 240
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
