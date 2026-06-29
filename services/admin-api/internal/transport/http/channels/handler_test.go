package channels

import "testing"

func TestParseOptionalBoolParam(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *bool
		wantErr bool
	}{
		{name: "empty", input: "", want: nil},
		{name: "true", input: "true", want: boolPtr(true)},
		{name: "one", input: "1", want: boolPtr(true)},
		{name: "false", input: "false", want: boolPtr(false)},
		{name: "zero", input: "0", want: boolPtr(false)},
		{name: "invalid", input: "abc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseOptionalBoolParam("compatible", tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.want == nil {
				if got != nil {
					t.Fatalf("expected nil, got %v", *got)
				}
				return
			}
			if got == nil || *got != *tt.want {
				t.Fatalf("expected %v, got %v", *tt.want, got)
			}
		})
	}
}

func TestParseBoolParamWithDefault(t *testing.T) {
	got, err := parseBoolParamWithDefault("hidden", "", false)
	if err != nil || got {
		t.Fatalf("expected false default, got=%v err=%v", got, err)
	}
	got, err = parseBoolParamWithDefault("hidden", "1", false)
	if err != nil || !got {
		t.Fatalf("expected true, got=%v err=%v", got, err)
	}
	if _, err = parseBoolParamWithDefault("hidden", "invalid", false); err == nil {
		t.Fatal("expected error for invalid bool")
	}
}

func boolPtr(v bool) *bool { return &v }
