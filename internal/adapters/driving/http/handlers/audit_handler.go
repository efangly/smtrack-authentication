package handlers

import (
	"strconv"

	"github.com/efangly/smtrack-authentication/internal/adapters/driving/http/dto"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/gofiber/fiber/v3"
)

// AuditHandler handles audit log HTTP requests
type AuditHandler struct {
	auditService ports.AuditService
}

// NewAuditHandler creates a new audit handler
func NewAuditHandler(auditService ports.AuditService) *AuditHandler {
	return &AuditHandler{auditService: auditService}
}

// FindAll handles listing audit logs, optionally filtered by query params.
func (h *AuditHandler) FindAll(c fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.Query("offset"))

	filter := ports.AuditFilter{
		Entity:   c.Query("entity"),
		Action:   c.Query("action"),
		EntityID: c.Query("entityId"),
		ActorID:  c.Query("actorId"),
		Limit:    limit,
		Offset:   offset,
	}

	logs, err := h.auditService.FindAll(c.Context(), filter)
	if err != nil {
		return serviceError(c, fiber.StatusBadRequest, err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(dto.FromAuditLogs(logs)))
}
