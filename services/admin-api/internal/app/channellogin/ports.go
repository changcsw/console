package channellogin

import (
	"context"
	"net/http"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/domain/channel"
)

type Error struct {
	Status  int
	Code    string
	Message string
	Details []any
}

func (e *Error) Error() string { return e.Message }

const (
	codeValidation = "VALIDATION_FAILED"
	codeNotFound   = "NOT_FOUND"
	codeConflict   = "CONFLICT"
)

func validationErr(msg string, details ...any) *Error {
	if details == nil {
		details = []any{}
	}
	return &Error{Status: http.StatusBadRequest, Code: codeValidation, Message: msg, Details: details}
}

func notFoundErr(msg string) *Error {
	return &Error{Status: http.StatusNotFound, Code: codeNotFound, Message: msg, Details: []any{}}
}

func conflictErr(msg string) *Error {
	return &Error{Status: http.StatusConflict, Code: codeConflict, Message: msg, Details: []any{}}
}

type ValidationDetail struct {
	Field   string `json:"field"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

type GameChannelRepository interface {
	GetByID(ctx context.Context, id int64) (channel.GameMarketChannel, error)
}

type ChannelPolicyRepository interface {
	GetByChannelID(ctx context.Context, channelID string) (channel.ChannelPolicy, error)
}

type ChannelLoginTemplateRepository interface {
	GetPublishedByChannel(ctx context.Context, channelIDRef int64) (*channel.ChannelLoginTemplate, error)
	GetByChannelVersion(ctx context.Context, channelIDRef int64, version string) (*channel.ChannelLoginTemplate, error)
}

type ChannelLoginConfigRepository interface {
	GetByGameChannel(ctx context.Context, gameChannelID int64) (*channel.ChannelLoginConfig, error)
	Upsert(ctx context.Context, cfg *channel.ChannelLoginConfig) error
}

type Repositories struct {
	GameChannels GameChannelRepository
	Policies     ChannelPolicyRepository
	Templates    ChannelLoginTemplateRepository
	Configs      ChannelLoginConfigRepository
}

type TxManager interface {
	Repositories() Repositories
	InTx(ctx context.Context, fn func(Repositories) error) error
}

type Cipher interface {
	Encrypt(plain string) (string, error)
}

type FileStore interface {
	NormalizeReference(ctx context.Context, key string) (string, error)
}

type AuditSink = adminapp.AuditSink
type AuditEntry = adminapp.AuditEntry
