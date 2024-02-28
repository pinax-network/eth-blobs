package controllers

import (
	"blob-service/dto"
	"blob-service/internal"
	pbbl "blob-service/pb/pinax/ethereum/blobs/v1"
	"context"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
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
	NOT_FOUND_SLOT = "slot_not_found" // no slot found
	INVALID_SLOT   = "invalid_slot"   // invalid slot
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
//	@Param		block_id	path		string		true	"Block identifier. Can be one of: 'head', slot number, hex encoded blockRoot with 0x prefix"
//	@Param		indices		query	 	[]string 	false 	"Array of indices for blob sidecars to request for in the specified block. Returns all blob sidecars in the block if not specified."
//	@Success	200		{object}	response.ApiDataResponse{data=blobsBySlotRetType} "Successful response"
//	@Failure	400		{object}	response.ApiErrorResponse	"invalid_slot"	"Invalid block id
//	@Failure	404		{object}	response.ApiErrorResponse	"slot_not_found"	"Slot not found"
//	@Failure	500		{object}	response.ApiErrorResponse
//	@Router		/eth/v1/beacon/blob_sidecars/{block_id} [get]
func (bc *BlobsController) BlobsByBlockId(c *gin.Context) {

	blockId := c.Param("block_id")
	indices := strings.Split(c.Query("indices"), ",")
	if len(indices) == 1 && indices[0] == "" {
		indices = []string{}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	slotNum, err := bc.parseBlockId(ctx, blockId)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			helper.ReportPublicErrorAndAbort(c, response.GatewayTimeout, err)
			return
		}
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.NotFound {
			helper.ReportPublicErrorAndAbort(c, response.NewApiErrorNotFound(NOT_FOUND_SLOT), err)
			return
		}
		helper.ReportPublicErrorAndAbort(c, response.BadGateway, err)
		return
	}

	resp, err := bc.sinkClient.Get(ctx, &pbkv.GetRequest{Key: fmt.Sprintf("slot:%d", slotNum)})
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			helper.ReportPublicErrorAndAbort(c, response.GatewayTimeout, err)
			return
		}
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.NotFound {
			helper.ReportPublicErrorAndAbort(c, response.NewApiErrorNotFound(NOT_FOUND_SLOT), err)
			return
		}
		helper.ReportPublicErrorAndAbort(c, response.BadGateway, err)
		return
	}

	slot := &pbbl.Slot{}
	err = proto.Unmarshal(resp.GetValue(), slot)
	if err != nil {
		helper.ReportPublicErrorAndAbort(c, response.InternalServerError, err)
		return
	}

	resBlobs := []*dto.Blob{}
	for _, blob := range slot.Blobs {
		if len(indices) == 0 || internal.Contains(indices, fmt.Sprintf("%d", blob.Index)) {
			resBlobs = append(resBlobs, dto.NewBlob(blob, slot))
		}
	}

	response.OkDataResponse(c, &response.ApiDataResponse{Data: resBlobs})
}
