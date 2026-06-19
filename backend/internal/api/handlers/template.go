package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/rohit-bagade/notifyx/internal/domain"
	"github.com/rohit-bagade/notifyx/internal/repository/postgres"
	"github.com/rohit-bagade/notifyx/pkg/logger"
)

// TemplateService groups the repository the template handler needs.
type TemplateService struct {
	Templates *postgres.TemplateRepository
}

// TemplateHandler handles all /api/v1/templates routes.
type TemplateHandler struct {
	svc *TemplateService
	log *logger.Logger
}

func NewTemplateHandler(svc *TemplateService, log *logger.Logger) *TemplateHandler {
	return &TemplateHandler{svc: svc, log: log}
}

// --- request / response types ---

type templateRequest struct {
	Name    string `json:"name"`
	Channel string `json:"channel"`
	Subject string `json:"subject,omitempty"`
	Body    string `json:"body"`
}

func (r *templateRequest) Validate() error {
	if r.Name == "" {
		return errors.New("name is required")
	}
	if !isValidChannel(r.Channel) {
		return fmt.Errorf("invalid channel %q: must be one of email, push, sms, inapp", r.Channel)
	}
	if r.Body == "" {
		return errors.New("body is required")
	}
	if domain.Channel(r.Channel) == domain.ChannelEmail && r.Subject == "" {
		return errors.New("subject is required for channel email")
	}
	return nil
}

type templateListResponse struct {
	Templates []*domain.NotificationTemplate `json:"templates"`
}

// --- handlers ---

// Create POST /api/v1/templates
func (h *TemplateHandler) Create(c echo.Context) error {
	tenant, err := RequireTenant(c)
	if err != nil {
		return err
	}

	var req templateRequest
	if err := c.Bind(&req); err != nil {
		return errResponse(c, http.StatusBadRequest, "invalid request body")
	}
	if err := req.Validate(); err != nil {
		return errResponse(c, http.StatusUnprocessableEntity, err.Error())
	}

	t, err := h.svc.Templates.Create(c.Request().Context(), tenant.ID, req.Name, domain.Channel(req.Channel), req.Subject, req.Body)
	if err != nil {
		if errors.Is(err, postgres.ErrAlreadyExists) {
			return errResponse(c, http.StatusConflict, fmt.Sprintf("a template named %q already exists", req.Name))
		}
		h.log.Errorw("create template failed", "error", err)
		return errResponse(c, http.StatusInternalServerError, "failed to create template")
	}
	return c.JSON(http.StatusCreated, t)
}

// List GET /api/v1/templates
func (h *TemplateHandler) List(c echo.Context) error {
	tenant, err := RequireTenant(c)
	if err != nil {
		return err
	}

	templates, err := h.svc.Templates.ListByTenant(c.Request().Context(), tenant.ID)
	if err != nil {
		h.log.Errorw("list templates failed", "error", err)
		return errResponse(c, http.StatusInternalServerError, "failed to list templates")
	}
	return c.JSON(http.StatusOK, templateListResponse{Templates: templates})
}

// Get GET /api/v1/templates/:id
func (h *TemplateHandler) Get(c echo.Context) error {
	tenant, err := RequireTenant(c)
	if err != nil {
		return err
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errResponse(c, http.StatusBadRequest, "invalid template id")
	}

	t, err := h.svc.Templates.GetByID(c.Request().Context(), tenant.ID, id)
	if err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			return errResponse(c, http.StatusNotFound, "template not found")
		}
		h.log.Errorw("get template failed", "error", err)
		return errResponse(c, http.StatusInternalServerError, "failed to get template")
	}
	return c.JSON(http.StatusOK, t)
}

// Update PUT /api/v1/templates/:id
func (h *TemplateHandler) Update(c echo.Context) error {
	tenant, err := RequireTenant(c)
	if err != nil {
		return err
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errResponse(c, http.StatusBadRequest, "invalid template id")
	}

	var req templateRequest
	if err := c.Bind(&req); err != nil {
		return errResponse(c, http.StatusBadRequest, "invalid request body")
	}
	if err := req.Validate(); err != nil {
		return errResponse(c, http.StatusUnprocessableEntity, err.Error())
	}

	t, err := h.svc.Templates.Update(c.Request().Context(), tenant.ID, id, req.Name, domain.Channel(req.Channel), req.Subject, req.Body)
	if err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			return errResponse(c, http.StatusNotFound, "template not found")
		}
		if errors.Is(err, postgres.ErrAlreadyExists) {
			return errResponse(c, http.StatusConflict, fmt.Sprintf("a template named %q already exists", req.Name))
		}
		h.log.Errorw("update template failed", "error", err)
		return errResponse(c, http.StatusInternalServerError, "failed to update template")
	}
	return c.JSON(http.StatusOK, t)
}

// Delete DELETE /api/v1/templates/:id
func (h *TemplateHandler) Delete(c echo.Context) error {
	tenant, err := RequireTenant(c)
	if err != nil {
		return err
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errResponse(c, http.StatusBadRequest, "invalid template id")
	}

	if err := h.svc.Templates.Delete(c.Request().Context(), tenant.ID, id); err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			return errResponse(c, http.StatusNotFound, "template not found")
		}
		h.log.Errorw("delete template failed", "error", err)
		return errResponse(c, http.StatusInternalServerError, "failed to delete template")
	}
	return c.NoContent(http.StatusNoContent)
}
