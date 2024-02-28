package controllers

import (
	"blob-service/dto"
	pbbl "blob-service/pb/pinax/ethereum/blobs/v1"
	"context"
	"encoding/binary"
	"fmt"
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

func (bc *BlobsController) parseBlockId(ctx context.Context, block_id string) (uint64, error) {
	if block_id == "head" {
		resp, err := bc.sinkClient.Get(ctx, &pbkv.GetRequest{Key: "head"})
		if err != nil {
			return 0, err
		}
		return binary.BigEndian.Uint64(resp.GetValue()), nil
	}

	if block_id[:2] == "0x" {
		resp, err := bc.sinkClient.Get(ctx, &pbkv.GetRequest{Key: "block_root:" + block_id})
		if err != nil {
			return 0, err
		}
		return binary.BigEndian.Uint64(resp.GetValue()), nil
	}

	return strconv.ParseUint(block_id, 10, 64)
}

// BlobsByBlockId
//
//	@Summary	Get Blobs by block id
//	@Tags		blobs
//	@Produce	json
//	@Param		block_id	path		string	true	"Block identifier. Can be one of: "head", <slot>, <hex encoded blockRoot with 0x prefix>."
//	@Success	200		{object}	response.ApiDataResponse{data=blobsBySlotRetType} "Successful response"
//	@Failure	400		{object}	response.ApiErrorResponse	"invalid_block_id"	"Invalid block_id
//	@Failure	404		{object}	response.ApiErrorResponse	"blobs_not_found"	"No blobs found"
//	@Failure	500		{object}	response.ApiErrorResponse
//	@Router		/eth/v1/beacon/blob_sidecars/{block_id} [get]
func (bc *BlobsController) BlobsByBlockId(c *gin.Context) {

	block_id := c.Param("block_id")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	slot, err := bc.parseBlockId(ctx, block_id)
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

	resp, err := bc.sinkClient.Get(ctx, &pbkv.GetRequest{Key: fmt.Sprintf("slot:%d", slot)})
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
