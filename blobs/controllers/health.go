package controllers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/pinax-network/blob-service/blobs/services"
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
	// HeadRoot is the block root of the head slot.
	HeadRoot string `json:"head_root,omitempty"`
	// HeadTime is the head slot's block time as unix seconds.
	HeadTime int64 `json:"head_time,omitempty"`
	// DriftSeconds is how far the head's block time lags wall-clock time.
	DriftSeconds int64 `json:"drift_seconds,omitempty"`
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
	head, err := hc.blobsService.GetHead(ctx)
	if err != nil {
		response.Status = "error"
		response.Detail = err.Error()
	} else {
		response.Head = head.Slot
		response.HeadRoot = head.Root
		response.HeadTime = head.Timestamp
		if head.Timestamp > 0 {
			response.DriftSeconds = time.Now().Unix() - head.Timestamp
		}
	}

	c.JSON(http.StatusOK, response)
}

// Liveness
// @Summary Lightweight liveness check without querying the backend.
// @Tags health
// @Success 200
// @Router /health [head]
func (hc *HealthController) Liveness(c *gin.Context) {
	c.Status(http.StatusOK)
}
