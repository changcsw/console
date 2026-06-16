package cashier

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/csw/console/services/admin-api/internal/app/command"
	domaincashier "github.com/csw/console/services/admin-api/internal/domain/cashier"
)

type TemplateVersionService interface {
	CopyToDraft(context.Context, command.CopyTemplateVersionCommand) (domaincashier.TemplateVersion, error)
}

type TemplateVersionHandler struct {
	service TemplateVersionService
}

func NewTemplateVersionHandler(service TemplateVersionService) *TemplateVersionHandler {
	return &TemplateVersionHandler{service: service}
}

func (h *TemplateVersionHandler) CopyToDraft(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	templateID, sourceVersion, err := parseTemplateVersionCopyPath(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := h.service.CopyToDraft(r.Context(), command.CopyTemplateVersionCommand{
		TemplateID:    templateID,
		SourceVersion: sourceVersion,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(result)
}

func parseTemplateVersionCopyPath(path string) (string, int, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 8 {
		return "", 0, fmt.Errorf("unexpected path: %s", path)
	}

	if parts[0] != "api" || parts[1] != "admin" || parts[2] != "cashier" || parts[3] != "templates" || parts[5] != "versions" || parts[7] != "copy-to-draft" {
		return "", 0, fmt.Errorf("unexpected path: %s", path)
	}

	version, err := strconv.Atoi(parts[6])
	if err != nil {
		return "", 0, fmt.Errorf("invalid version: %w", err)
	}

	return parts[4], version, nil
}
