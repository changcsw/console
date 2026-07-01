package sync

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Section string
type SyncOp string
type SyncJobStatus string

const (
	SectionGame     Section = "game"
	SectionMarkets  Section = "markets"
	SectionLegal    Section = "legal"
	SectionChannels Section = "channels"
	SectionPackages Section = "packages"
	SectionProducts Section = "products"
	SectionCashier  Section = "cashier"
	SectionPayments Section = "payments"
	SectionConfig   Section = "config"

	OpAdd    SyncOp = "add"
	OpUpdate SyncOp = "update"
	OpDelete SyncOp = "delete"

	JobStatusPreviewed SyncJobStatus = "previewed"
	JobStatusSucceeded SyncJobStatus = "succeeded"
	JobStatusFailed    SyncJobStatus = "failed"

	sourceEnv = "sandbox"
	targetEnv = "production"
)

var allSections = []Section{
	SectionGame,
	SectionMarkets,
	SectionLegal,
	SectionChannels,
	SectionPackages,
	SectionProducts,
	SectionCashier,
	SectionPayments,
	SectionConfig,
}

var sectionDeps = map[Section][]Section{
	SectionGame:     {},
	SectionMarkets:  {SectionGame},
	SectionLegal:    {SectionGame},
	SectionChannels: {SectionGame, SectionMarkets},
	SectionPackages: {SectionChannels},
	SectionProducts: {SectionGame},
	SectionCashier:  {SectionGame},
	SectionPayments: {SectionChannels, SectionPackages, SectionProducts, SectionCashier},
	SectionConfig:   {SectionChannels, SectionPackages, SectionProducts, SectionCashier, SectionPayments},
}

type DiffSummary struct {
	Add    int `json:"add"`
	Update int `json:"update"`
	Delete int `json:"delete"`
}

type DiffChange struct {
	Op              SyncOp `json:"op"`
	EntityType      string `json:"entityType"`
	EntityKey       string `json:"entityKey"`
	FieldName       string `json:"fieldName"`
	SandboxValue    any    `json:"sandboxValue,omitempty"`
	ProductionValue any    `json:"productionValue,omitempty"`
	Masked          bool   `json:"masked"`
}

type DiffSection struct {
	Section      Section      `json:"section"`
	Summary      DiffSummary  `json:"summary"`
	Dependencies []Section    `json:"dependencies"`
	Changes      []DiffChange `json:"changes"`
}

type Preview struct {
	GameID           string        `json:"gameId"`
	SourceEnv        string        `json:"sourceEnv"`
	TargetEnv        string        `json:"targetEnv"`
	SourceHash       string        `json:"sourceHash"`
	TargetHashBefore string        `json:"targetHashBefore"`
	HasDiff          bool          `json:"hasDiff"`
	BaselineToken    string        `json:"baselineToken"`
	PreviewedAt      time.Time     `json:"previewedAt"`
	ExpiresAt        time.Time     `json:"expiresAt"`
	Sections         []DiffSection `json:"sections"`
}

type ExecuteSkippedDelete struct {
	Section   Section `json:"section"`
	EntityKey string  `json:"entityKey"`
	Reason    string  `json:"reason"`
}

type ExecuteSkipped struct {
	Deletes           []ExecuteSkippedDelete `json:"deletes"`
	UnselectedSection []Section              `json:"unselectedSections"`
}

type ExecuteResult struct {
	SyncJobID        int64                 `json:"syncJobId,string"`
	GameID           string                `json:"gameId"`
	SourceEnv        string                `json:"sourceEnv"`
	TargetEnv        string                `json:"targetEnv"`
	Status           SyncJobStatus         `json:"status"`
	SelectedSections []Section             `json:"selectedSections"`
	IncludeDeletes   bool                  `json:"includeDeletes"`
	SourceHash       string                `json:"sourceHash"`
	TargetHashBefore string                `json:"targetHashBefore"`
	TargetHashAfter  string                `json:"targetHashAfter"`
	AppliedSummary   map[Section]DiffSummary `json:"appliedSummary"`
	Skipped          ExecuteSkipped        `json:"skipped"`
	ExecutedAt       time.Time             `json:"executedAt"`
}

type JobItem struct {
	SyncJobID         int64      `json:"syncJobId,string"`
	GameID            string     `json:"gameId"`
	SourceEnv         string     `json:"sourceEnv"`
	TargetEnv         string     `json:"targetEnv"`
	Status            string     `json:"status"`
	IncludeDeletes    bool       `json:"includeDeletes"`
	OperatorID        int64      `json:"operatorId"`
	OperatorNote      string     `json:"operatorNote"`
	SourceHash        string     `json:"sourceHash"`
	TargetHashBefore  string     `json:"targetHashBefore"`
	TargetHashAfter   string     `json:"targetHashAfter"`
	ExecutedAt        *time.Time `json:"executedAt"`
	CreatedAt         time.Time  `json:"createdAt"`
}

type JobList struct {
	Items    []JobItem `json:"items"`
	Page     int       `json:"page"`
	PageSize int       `json:"pageSize"`
	Total    int       `json:"total"`
}

type EntityRecord struct {
	EntityType string
	EntityKey  string
	Fields     map[string]any
}

type BaselineTokenPayload struct {
	GameID           string    `json:"gameId"`
	SyncJobID        int64     `json:"syncJobId"`
	SourceEnv        string    `json:"sourceEnv"`
	TargetEnv        string    `json:"targetEnv"`
	SourceHash       string    `json:"sourceHash"`
	TargetHashBefore string    `json:"targetHashBefore"`
	PreviewedAt      time.Time `json:"previewedAt"`
	ExpiresAt        time.Time `json:"expiresAt"`
	Nonce            string    `json:"nonce"`
}

func DefaultSourceEnv() string { return sourceEnv }
func DefaultTargetEnv() string { return targetEnv }

func (s Section) IsKnown() bool {
	_, ok := sectionDeps[s]
	return ok
}

func (s Section) Dependencies() []Section {
	out := make([]Section, len(sectionDeps[s]))
	copy(out, sectionDeps[s])
	return out
}

func AllSections() []Section {
	result := make([]Section, len(allSections))
	copy(result, allSections)
	return result
}

func ParseSections(values []string, requireExplicit bool) ([]Section, error) {
	if len(values) == 0 {
		if requireExplicit {
			return nil, fmt.Errorf("selected_sections is required")
		}
		return AllSections(), nil
	}
	seen := make(map[Section]struct{}, len(values))
	result := make([]Section, 0, len(values))
	for _, raw := range values {
		section := Section(strings.TrimSpace(raw))
		if !section.IsKnown() {
			return nil, fmt.Errorf("unknown section: %s", raw)
		}
		if _, exists := seen[section]; exists {
			continue
		}
		seen[section] = struct{}{}
		result = append(result, section)
	}
	return result, nil
}

func SortSectionsByTopo(selected []Section) []Section {
	seen := make(map[Section]struct{}, len(selected))
	for _, s := range selected {
		seen[s] = struct{}{}
	}
	out := make([]Section, 0, len(selected))
	for _, s := range allSections {
		if _, ok := seen[s]; ok {
			out = append(out, s)
		}
	}
	return out
}

func BuildBaselineToken(payload BaselineTokenPayload, secret []byte) (string, error) {
	data, err := CanonicalJSON(payload)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(data)
	sig := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(data) + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func ParseAndVerifyBaselineToken(token string, secret []byte, now time.Time) (BaselineTokenPayload, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 2 {
		return BaselineTokenPayload{}, fmt.Errorf("invalid token format")
	}
	payloadRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return BaselineTokenPayload{}, fmt.Errorf("decode payload: %w", err)
	}
	sigRaw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return BaselineTokenPayload{}, fmt.Errorf("decode signature: %w", err)
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(payloadRaw)
	if !hmac.Equal(sigRaw, mac.Sum(nil)) {
		return BaselineTokenPayload{}, fmt.Errorf("invalid token signature")
	}
	var payload BaselineTokenPayload
	if err := json.Unmarshal(payloadRaw, &payload); err != nil {
		return BaselineTokenPayload{}, fmt.Errorf("decode payload json: %w", err)
	}
	if now.UTC().After(payload.ExpiresAt.UTC()) {
		return BaselineTokenPayload{}, fmt.Errorf("token expired")
	}
	return payload, nil
}

func GenerateNonce() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func DiffEntities(section Section, sandbox, production []EntityRecord, explicitMaskedFields map[string]struct{}) DiffSection {
	sMap := make(map[string]EntityRecord, len(sandbox))
	pMap := make(map[string]EntityRecord, len(production))
	keys := make([]string, 0, len(sandbox)+len(production))
	seen := map[string]struct{}{}
	for _, row := range sandbox {
		sMap[row.EntityType+"|"+row.EntityKey] = row
		if _, ok := seen[row.EntityType+"|"+row.EntityKey]; !ok {
			seen[row.EntityType+"|"+row.EntityKey] = struct{}{}
			keys = append(keys, row.EntityType+"|"+row.EntityKey)
		}
	}
	for _, row := range production {
		pMap[row.EntityType+"|"+row.EntityKey] = row
		if _, ok := seen[row.EntityType+"|"+row.EntityKey]; !ok {
			seen[row.EntityType+"|"+row.EntityKey] = struct{}{}
			keys = append(keys, row.EntityType+"|"+row.EntityKey)
		}
	}
	sort.Strings(keys)
	out := DiffSection{
		Section:      section,
		Dependencies: section.Dependencies(),
		Changes:      []DiffChange{},
	}
	for _, key := range keys {
		s, hasS := sMap[key]
		p, hasP := pMap[key]
		switch {
		case hasS && !hasP:
			addFields, addMasked := maskRowFields(s.Fields, explicitMaskedFields)
			out.Summary.Add++
			out.Changes = append(out.Changes, DiffChange{
				Op:           OpAdd,
				EntityType:   s.EntityType,
				EntityKey:    s.EntityKey,
				FieldName:    "*",
				SandboxValue: addFields,
				Masked:       addMasked,
			})
		case !hasS && hasP:
			delFields, delMasked := maskRowFields(p.Fields, explicitMaskedFields)
			out.Summary.Delete++
			out.Changes = append(out.Changes, DiffChange{
				Op:              OpDelete,
				EntityType:      p.EntityType,
				EntityKey:       p.EntityKey,
				FieldName:       "*",
				ProductionValue: delFields,
				Masked:          delMasked,
			})
		default:
			fieldNames := unionKeys(s.Fields, p.Fields)
			for _, field := range fieldNames {
				sVal, sOK := s.Fields[field]
				pVal, pOK := p.Fields[field]
				if !sOK {
					sVal = nil
				}
				if !pOK {
					pVal = nil
				}
				if deepEqualJSON(sVal, pVal) {
					continue
				}
				masked := isMaskedField(field, explicitMaskedFields)
				if masked {
					sVal = "masked"
					pVal = "masked"
				}
				out.Summary.Update++
				out.Changes = append(out.Changes, DiffChange{
					Op:              OpUpdate,
					EntityType:      s.EntityType,
					EntityKey:       s.EntityKey,
					FieldName:       field,
					SandboxValue:    sVal,
					ProductionValue: pVal,
					Masked:          masked,
				})
			}
		}
	}
	return out
}

func HashEntitySets(sets map[Section][]EntityRecord) (string, error) {
	m := make(map[string]any, len(sets))
	for _, section := range allSections {
		items := sets[section]
		sort.Slice(items, func(i, j int) bool {
			li := items[i].EntityType + "|" + items[i].EntityKey
			lj := items[j].EntityType + "|" + items[j].EntityKey
			return li < lj
		})
		arr := make([]map[string]any, 0, len(items))
		for _, item := range items {
			arr = append(arr, map[string]any{
				"entityType": item.EntityType,
				"entityKey":  item.EntityKey,
				"fields":     NormalizeForHash(item.Fields),
			})
		}
		m[string(section)] = arr
	}
	raw, err := CanonicalJSON(m)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func NormalizeForHash(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		key := strings.ToLower(strings.TrimSpace(k))
		switch key {
		case "id", "created_at", "updated_at", "createdat", "updatedat":
			continue
		default:
			out[k] = v
		}
	}
	return out
}

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

func unionKeys(a, b map[string]any) []string {
	set := map[string]struct{}{}
	for k := range a {
		set[k] = struct{}{}
	}
	for k := range b {
		set[k] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func deepEqualJSON(a, b any) bool {
	ab, errA := CanonicalJSON(a)
	bb, errB := CanonicalJSON(b)
	if errA != nil || errB != nil {
		return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
	}
	return bytes.Equal(ab, bb)
}

type MissingDependencyDetail struct {
	Section           Section `json:"section"`
	MissingDependency Section `json:"missingDependency"`
	EntityKey         string  `json:"entityKey"`
}

// ValidateSectionDependencies 校验所选 section 的前置依赖在 production 已存在（或本次同批选中）。
func ValidateSectionDependencies(selected []Section, productionSets map[Section][]EntityRecord) []MissingDependencyDetail {
	selectedSet := make(map[Section]struct{}, len(selected))
	for _, sec := range selected {
		selectedSet[sec] = struct{}{}
	}
	prodKeys := make(map[Section]map[string]struct{}, len(allSections))
	for sec, records := range productionSets {
		keys := make(map[string]struct{}, len(records))
		for _, row := range records {
			keys[row.EntityKey] = struct{}{}
		}
		prodKeys[sec] = keys
	}
	var missing []MissingDependencyDetail
	for _, sec := range selected {
		for _, dep := range sec.Dependencies() {
			if _, ok := selectedSet[dep]; ok {
				continue
			}
			keys := prodKeys[dep]
			if len(keys) == 0 {
				missing = append(missing, MissingDependencyDetail{
					Section:           sec,
					MissingDependency: dep,
					EntityKey:         "*",
				})
			}
		}
	}
	return missing
}

// maskRowFields 对整行 add/delete 的字段集做字段级脱敏兜底：
// 命中密文字段的值以 "masked" 回填（绝不回明文/密文），返回是否发生脱敏。
func maskRowFields(fields map[string]any, explicit map[string]struct{}) (map[string]any, bool) {
	if fields == nil {
		return nil, false
	}
	out := make(map[string]any, len(fields))
	masked := false
	for k, v := range fields {
		if isMaskedField(k, explicit) {
			out[k] = "masked"
			masked = true
			continue
		}
		out[k] = v
	}
	return out, masked
}

func isMaskedField(field string, explicit map[string]struct{}) bool {
	key := strings.ToLower(strings.TrimSpace(field))
	if strings.HasSuffix(key, "_ciphertext") || strings.Contains(key, "secret") {
		return true
	}
	_, ok := explicit[field]
	return ok
}
