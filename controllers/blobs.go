package controllers

import (
	"blob-service/dto"
	pbbl "blob-service/pb/pinax/ethereum/blobs/v1"
	"context"
	"strconv"
	"time"

	pbkv "github.com/streamingfast/substreams-sink-kv/pb/substreams/sink/kv/v1"

	"github.com/eosnationftw/eosn-base-api/helper"
	"github.com/eosnationftw/eosn-base-api/response"
	"github.com/gin-gonic/gin"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	NOT_FOUND_BLOBS = "blobs_not_found" // no blobs found
	INVALID_SLOT    = "invalid_slot"    // invalid slot
)

type BlobsController struct {
	sinkClient pbkv.KvClient
}

func NewBlobsController(sinkClient pbkv.KvClient) *BlobsController {
	return &BlobsController{sinkClient: sinkClient}
}

type blobsBySlotRetType []*dto.Blob

// BlobsBySlot
//
//	@Summary	Get Blobs by slot
//	@Tags		blobs
//	@Produce	json
//	@Param		slot	path		string	true	"Slot Number"
//	@Success	200		{object}	response.ApiDataResponse{data=blobsBySlotRetType} "Sulccessful response"
//	@Failure	400		{object}	response.ApiErrorResponse	"invalid_slot"	"Invalid slot
//	@Failure	404		{object}	response.ApiErrorResponse	"blobs_not_found"	"No blobs found"
//	@Failure	500		{object}	response.ApiErrorResponse
//	@Router		/blobs/by_slot/{slot} [get]
func (bc *BlobsController) BlobsBySlot(c *gin.Context) {

	slot := c.Param("slot")

	if _, err := strconv.Atoi(slot); err != nil {
		helper.ReportPublicErrorAndAbort(c, response.NewApiErrorBadRequest(INVALID_SLOT), err)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	resp, err := bc.sinkClient.Get(ctx, &pbkv.GetRequest{Key: "slot:" + slot})
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			helper.ReportPublicErrorAndAbort(c, response.GatewayTimeout, err)
			return
		}
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.NotFound {
			helper.ReportPublicErrorAndAbort(c, response.NewApiErrorNotFound(NOT_FOUND_BLOBS), err)
			return
		}
		helper.ReportPublicErrorAndAbort(c, response.BadGateway, err)
		return
	}

	blobs := &pbbl.Blobs{}
	err = proto.Unmarshal(resp.GetValue(), blobs)
	if err != nil {
		helper.ReportPublicErrorAndAbort(c, response.InternalServerError, err)
		return
	}

	resBlobs := []*dto.Blob{}
	for _, blob := range blobs.Blobs {
		resBlobs = append(resBlobs, dto.NewBlob(blob))
	}

	response.OkDataResponse(c, &response.ApiDataResponse{Data: resBlobs})
}
