package controllers

import (
	"blob-service/dto"
	"blob-service/internal"
	"blob-service/services"
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pinax-network/golang-base/response"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type BlobsController struct {
	blobsService *services.BlobsService
}

func NewBlobsController(blobsService *services.BlobsService) *BlobsController {
	return &BlobsController{blobsService: blobsService}
}

// BlobsByBlockId
//
//	@Summary	Get Blobs by block id
//	@Tags		blobs
//	@Produce	json
//	@Param		block_id	path		string		true	"Block identifier. Can be one of: 'head', slot number, hex encoded blockRoot with 0x prefix"
//	@Param		indices		query	 	[]string 	false 	"Array of indices for blob sidecars to request for in the specified block. Returns all blob sidecars in the block if not specified."
//	@Success	200		{object}	response.ApiDataResponse{data=[]dto.Blob} "Successful response"
//	@Failure	400		{object}	response.ApiErrorResponse	"invalid_slot"	"Invalid block id"
//	@Failure	404		{object}	response.ApiErrorResponse	"slot_not_found"	"Slot not found"
//	@Failure	500		{object}	response.ApiErrorResponse
//	@Router		/eth/v1/beacon/blob_sidecars/{block_id} [get]
func (bc *BlobsController) BlobsByBlockId(c *gin.Context) {

	blockId := c.Param("block_id")
	indices := []uint32{}
	for _, str := range strings.Split(c.Query("indices"), ",") {
		if str == "" {
			continue
		}
		i, err := strconv.ParseUint(str, 10, 32)
		if err != nil {
			internal.WriteErrorResponse(c, internal.ErrInvalidIndex)
			return
		}
		indices = append(indices, uint32(i))
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	slot, err := bc.blobsService.GetSlotByBlockId(ctx, blockId)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			internal.WriteErrorResponse(c, internal.ErrSinkTimeout)
			return
		}
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.NotFound {
			internal.WriteErrorResponse(c, internal.ErrSlotNotFound)
			return
		}
		internal.WriteErrorResponse(c, err)
		return
	}

	resBlobs := []*dto.Blob{}
	for _, blob := range slot.Blobs {
		if len(indices) == 0 || internal.Contains(indices, blob.Index) {
			resBlobs = append(resBlobs, dto.NewBlob(blob, slot))
		}
	}

	response.OkDataResponse(c, &response.ApiDataResponse{Data: resBlobs})
}

// BlobsByBlockIdV2 implements the Beacon API v4.0.0 endpoint that replaces
// the deprecated /eth/v1/beacon/blob_sidecars/{block_id}. The response is a
// flat list of blob byte strings, ordered by KZG commitment order in the
// block, and filterable by versioned hash rather than blob index.
//
//	@Summary	Get Blobs by block id (Beacon API v4.0.0)
//	@Tags		blobs
//	@Produce	json
//	@Param		block_id			path		string		true	"Block identifier. Can be one of: 'head', slot number, or 0x-prefixed hex block root"
//	@Param		versioned_hashes	query		[]string	false	"Comma-separated list of 0x-prefixed 32-byte versioned hashes. Returns all blobs in the block if not specified."
//	@Success	200		{object}	dto.BlobsResponse	"Successful response"
//	@Failure	400		{object}	response.ApiErrorResponse	"invalid_slot or invalid_versioned_hash"
//	@Failure	404		{object}	response.ApiErrorResponse	"slot_not_found"
//	@Failure	500		{object}	response.ApiErrorResponse
//	@Router		/eth/v1/beacon/blobs/{block_id} [get]
func (bc *BlobsController) BlobsByBlockIdV2(c *gin.Context) {

	blockId := c.Param("block_id")

	hashFilter := map[[32]byte]struct{}{}
	for _, str := range strings.Split(c.Query("versioned_hashes"), ",") {
		if str == "" {
			continue
		}
		h, err := internal.ParseVersionedHash(str)
		if err != nil {
			internal.WriteErrorResponse(c, internal.ErrInvalidVersionedHash)
			return
		}
		hashFilter[h] = struct{}{}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	slot, err := bc.blobsService.GetSlotByBlockId(ctx, blockId)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			internal.WriteErrorResponse(c, internal.ErrSinkTimeout)
			return
		}
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.NotFound {
			internal.WriteErrorResponse(c, internal.ErrSlotNotFound)
			return
		}
		internal.WriteErrorResponse(c, err)
		return
	}

	data := []dto.HexBytes{}
	for _, blob := range slot.Blobs {
		if len(hashFilter) > 0 {
			vh := internal.VersionedHashFromCommitment(blob.KzgCommitment)
			if _, ok := hashFilter[vh]; !ok {
				continue
			}
		}
		data = append(data, dto.HexBytes(blob.Blob))
	}

	c.JSON(http.StatusOK, dto.BlobsResponse{
		ExecutionOptimistic: false,
		Finalized:           false,
		Data:                data,
	})
}
