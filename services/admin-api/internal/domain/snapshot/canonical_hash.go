package snapshot

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"
)

func CanonicalJSON(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, err
	}
	buf := &bytes.Buffer{}
	if err := writeCanonical(buf, decoded); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func HashCanonicalJSON(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func BuildConfigVersion(generatedAt time.Time, hash string) string {
	short := hash
	if len(short) > 8 {
		short = short[:8]
	}
	return generatedAt.UTC().Format("20060102150405") + "-" + short
}

func writeCanonical(buf *bytes.Buffer, v any) error {
	switch vv := v.(type) {
	case map[string]any:
		buf.WriteByte('{')
		keys := make([]string, 0, len(vv))
		for k := range vv {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			keyJSON, _ := json.Marshal(k)
			buf.Write(keyJSON)
			buf.WriteByte(':')
			if err := writeCanonical(buf, vv[k]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	case []any:
		buf.WriteByte('[')
		for i, item := range vv {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeCanonical(buf, item); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	case string:
		data, _ := json.Marshal(vv)
		buf.Write(data)
	case float64:
		buf.WriteString(strconv.FormatFloat(vv, 'f', -1, 64))
	case bool:
		if vv {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case nil:
		buf.WriteString("null")
	default:
		data, err := json.Marshal(vv)
		if err != nil {
			return fmt.Errorf("marshal canonical value: %w", err)
		}
		buf.Write(data)
	}
	return nil
}
