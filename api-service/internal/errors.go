package internal

import (
	"net/http"

	"github.com/friendsofgo/errors"
	"github.com/gin-gonic/gin"
	"github.com/pinax-network/golang-base/helper"
	"github.com/pinax-network/golang-base/log"
	"github.com/pinax-network/golang-base/response"
	"go.uber.org/zap"
)

var (
	ErrSinkTimeout  = errors.New("timeout when trying to reach the sink service")
	ErrSlotNotFound = errors.New("slot not found")
	ErrInvalidSlot  = errors.New("invalid slot")
	ErrInvalidIndex = errors.New("invalid index")
)

const (
	NOT_FOUND_SLOT = "slot_not_found"
	INVALID_SLOT   = "invalid_slot"
	INVALID_INDEX  = "invalid_index"
	SINK_TIMEOUT   = "sink_timeout"
)

func WriteErrorResponse(c *gin.Context, err error) {

	// 404 NOT FOUND
	if errors.Is(err, ErrSlotNotFound) {
		helper.ReportPublicErrorAndAbort(c, response.NewApiErrorNotFound(NOT_FOUND_SLOT), err)
		return
	}

	// 400 BAD REQUEST
	if errors.Is(err, ErrInvalidSlot) {
		helper.ReportPublicErrorAndAbort(c, response.NewApiErrorBadRequest(INVALID_SLOT), err)
		return
	}
	if errors.Is(err, ErrInvalidIndex) {
		helper.ReportPublicErrorAndAbort(c, response.NewApiErrorBadRequest(INVALID_INDEX), err)
		return
	}

	// 504 GATEWAY TIMEOUT
	if errors.Is(err, ErrSinkTimeout) {
		helper.ReportPublicErrorAndAbort(c, response.NewApiError(http.StatusGatewayTimeout, SINK_TIMEOUT), err)
		return
	}

	log.Error("unknown error received, writing internal_server_error instead", zap.Error(err))
	helper.ReportPrivateErrorAndAbort(c, response.InternalServerError, err)
}
