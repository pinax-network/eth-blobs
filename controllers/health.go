package controllers

import (
	"blob-service/services"
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type HealthController struct {
	blobsService *services.BlobsService
}

func NewHealthController(blobsService *services.BlobsService) *HealthController {
	return &HealthController{blobsService: blobsService}
}

type HealthResponse struct {
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
	Head   uint64 `json:"head,omitempty"`
}

// Health
// @Summary Returns health status of this API.
// @Tags health
// @Produce json
// @Success 200 {object} HealthResponse
// @Failure 500 {object} response.ApiErrorResponse
// @Router /health [get]
func (hc *HealthController) Health(c *gin.Context) {

	ctx, cancel := context.WithTimeout(c.Request.Context(), 1*time.Second)
	defer cancel()

	response := &HealthResponse{Status: "ok"}
	slotNum, err := hc.blobsService.GetSlotNumber(ctx, "head")
	if err != nil {
		response.Status = "error"
		response.Detail = err.Error()
	} else {
		response.Head = slotNum
	}

	c.JSON(http.StatusOK, response)
}
