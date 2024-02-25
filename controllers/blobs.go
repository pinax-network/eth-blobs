package controllers

import (
	pbbmsrv "blob-service/pb"
	"context"
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
	NOT_FOUND_BLOBS = "not_found_blobs" // no blobs found
)

type BlobsController struct {
	sinkClient pbkv.KvClient
}

func NewBlobsController(sinkClient pbkv.KvClient) *BlobsController {
	return &BlobsController{sinkClient: sinkClient}
}

type BlobsResponse struct {
	Blobs []*pbbmsrv.Blob `json:"blobs"`
}

// BlobsBySlot
//
//	@Summary	Get Blobs by slot number
//	@Tags		blobs
//	@Produce	json
//	@Param		slot	path		string	true	"Slot Number"
//	@Success	200		{object}	response.ApiDataResponse{data=BlobsResponse}
//	@Failure	404		{object}	response.ApiErrorResponse	"No blobs in this slot"
//	@Failure	500		{object}	response.ApiErrorResponse
//	@Router		/blobs/by_slot/{slot} [get]
func (bc *BlobsController) BlobsBySlot(c *gin.Context) {

	slot := c.Param("slot")

	key := "slot:" + slot

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	resp, err := bc.sinkClient.Get(ctx, &pbkv.GetRequest{Key: key})
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

	blobs := &pbbmsrv.Blobs{}
	err = proto.Unmarshal(resp.GetValue(), blobs)
	if err != nil {
		helper.ReportPublicErrorAndAbort(c, response.InternalServerError, err)
		return
	}

	response.OkDataResponse(c, &response.ApiDataResponse{Data: &BlobsResponse{
		Blobs: blobs.Blobs,
	}})
}
