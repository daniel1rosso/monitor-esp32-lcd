package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/application"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/domain"
	"github.com/daniel1rosso/monitor-esp32-lcd/backend/internal/ports"
	"github.com/gin-gonic/gin"
)

type serviceCreateRequest struct {
	ProductID string  `json:"product_id"`
	Key       string  `json:"key"`
	Name      string  `json:"name"`
	Kind      string  `json:"kind"`
	Endpoint  *string `json:"endpoint"`
	Enabled   *bool   `json:"enabled"`
}
type serviceUpdateRequest struct {
	Name     *string         `json:"name"`
	Kind     *string         `json:"kind"`
	Endpoint json.RawMessage `json:"endpoint"`
	Enabled  *bool           `json:"enabled"`
}

func (api *API) createService(c *gin.Context) {
	var request serviceCreateRequest
	if !api.decode(c, &request, 16<<10) {
		api.writeProblem(c, domain.ErrInvalid)
		return
	}
	enabled := true
	if request.Enabled != nil {
		enabled = *request.Enabled
	}
	value, err := api.app.CreateService(c.Request.Context(), application.ServiceInput{ProductID: request.ProductID, Key: request.Key, Name: request.Name, Kind: request.Kind, Endpoint: request.Endpoint, Enabled: enabled})
	if err != nil {
		api.writeProblem(c, err)
		return
	}
	c.JSON(http.StatusCreated, serviceDTO(value))
}
func (api *API) getService(c *gin.Context) {
	value, err := api.app.GetService(c.Request.Context(), c.Param("serviceId"))
	if err != nil {
		api.writeProblem(c, err)
		return
	}
	c.JSON(http.StatusOK, serviceDTO(value))
}
func (api *API) listServices(c *gin.Context) {
	items, next, err := api.app.ListServices(c.Request.Context(), queryLimit(c), c.Query("cursor"), c.Query("product_id"))
	if err != nil {
		api.writeProblem(c, err)
		return
	}
	values := make([]gin.H, len(items))
	for index, item := range items {
		values[index] = serviceDTO(item)
	}
	writePage(c, values, next)
}
func (api *API) updateService(c *gin.Context) {
	var request serviceUpdateRequest
	if !api.decode(c, &request, 16<<10) {
		api.writeProblem(c, domain.ErrInvalid)
		return
	}
	var endpoint **string
	if len(request.Endpoint) > 0 {
		var value *string
		if !bytes.Equal(request.Endpoint, []byte("null")) {
			var parsed string
			if err := json.Unmarshal(request.Endpoint, &parsed); err != nil {
				api.writeProblem(c, domain.ErrInvalid)
				return
			}
			value = &parsed
		}
		endpoint = &value
	}
	result, err := api.app.UpdateService(c.Request.Context(), c.Param("serviceId"), application.ServicePatch{Name: request.Name, Kind: request.Kind, Endpoint: endpoint, Enabled: request.Enabled})
	if err != nil {
		api.writeProblem(c, err)
		return
	}
	c.JSON(http.StatusOK, serviceDTO(result))
}
func (api *API) archiveService(c *gin.Context) {
	if err := api.app.ArchiveService(c.Request.Context(), c.Param("serviceId")); err != nil {
		api.writeProblem(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
func serviceDTO(value domain.Service) gin.H {
	return gin.H{"id": value.ID, "product_id": value.ProductID, "key": value.Key, "name": value.Name, "status": value.Status, "latency_ms": value.LatencyMS, "updated_at": value.UpdatedAt}
}

type alertCreateRequest struct {
	ProductID   *string         `json:"product_id"`
	ServiceID   *string         `json:"service_id"`
	Fingerprint string          `json:"fingerprint"`
	Priority    domain.Priority `json:"priority"`
	Title       string          `json:"title"`
	Message     string          `json:"message"`
	ExpiresAt   *time.Time      `json:"expires_at"`
}

func (api *API) createAlert(c *gin.Context) {
	var request alertCreateRequest
	if !api.decode(c, &request, 16<<10) {
		api.writeProblem(c, domain.ErrInvalid)
		return
	}
	value, err := api.app.CreateAlert(c.Request.Context(), application.AlertInput{ProductID: request.ProductID, ServiceID: request.ServiceID, Fingerprint: request.Fingerprint, Priority: request.Priority, Title: request.Title, Message: request.Message, ExpiresAt: request.ExpiresAt})
	if err != nil {
		api.writeProblem(c, err)
		return
	}
	c.JSON(http.StatusCreated, alertDTO(value))
}
func (api *API) listAlerts(c *gin.Context) {
	items, next, err := api.app.ListAlerts(c.Request.Context(), queryLimit(c), c.Query("cursor"), ports.AlertFilter{State: c.Query("state"), Priority: c.Query("priority"), ProductID: c.Query("product_id")})
	if err != nil {
		api.writeProblem(c, err)
		return
	}
	values := make([]gin.H, len(items))
	for index, item := range items {
		values[index] = alertDTO(item)
	}
	writePage(c, values, next)
}
func (api *API) acknowledgeAlert(c *gin.Context) {
	value, err := api.app.AcknowledgeAlert(c.Request.Context(), c.Param("alertId"))
	if err != nil {
		api.writeProblem(c, err)
		return
	}
	c.JSON(http.StatusOK, alertDTO(value))
}
func (api *API) resolveAlert(c *gin.Context) {
	value, err := api.app.ResolveAlert(c.Request.Context(), c.Param("alertId"))
	if err != nil {
		api.writeProblem(c, err)
		return
	}
	c.JSON(http.StatusOK, alertDTO(value))
}
func alertDTO(value domain.Alert) gin.H {
	return gin.H{"id": value.ID, "product_id": value.ProductID, "service_id": value.ServiceID, "fingerprint": value.Fingerprint, "priority": value.Priority, "state": value.State, "title": value.Title, "message": value.Message, "opened_at": value.OpenedAt, "acknowledged_at": value.AcknowledgedAt, "resolved_at": value.ResolvedAt, "updated_at": value.UpdatedAt}
}
func writePage(c *gin.Context, items any, next string) {
	var cursor any
	if next != "" {
		cursor = next
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "next_cursor": cursor, "has_more": next != ""})
}
