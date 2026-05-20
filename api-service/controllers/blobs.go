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
	_ "github.com/pinax-network/golang-base/response" // referenced by swagger annotations
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
//	@Param		indices		query	 	[]string 	false 	"Blob sidecar indices to return. Accepts repeated keys (?indices=0&indices=1) or comma-separated (?indices=0,1). Returns all if omitted." collectionFormat(multi)
//	@Success	200		{object}	dto.BlobSidecarsResponse "Successful response"
//	@Failure	400		{object}	response.ApiErrorResponse	"invalid_slot"	"Invalid block id"
//	@Failure	404		{object}	response.ApiErrorResponse	"slot_not_found"	"Slot not found"
//	@Failure	500		{object}	response.ApiErrorResponse
//	@Router		/eth/v1/beacon/blob_sidecars/{block_id} [get]
func (bc *BlobsController) BlobsByBlockId(c *gin.Context) {

	blockId := c.Param("block_id")
	indices := []uint32{}
	// Accept both comma-separated (?indices=1,2,3) and repeated-key
	// (?indices=1&indices=2) styles. Lighthouse uses the repeated form;
	// older clients of ours use the comma form.
	for _, raw := range c.QueryArray("indices") {
		for _, str := range strings.Split(raw, ",") {
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
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	slot, headSlot, err := bc.blobsService.GetSlotByBlockId(ctx, blockId)
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

	c.JSON(http.StatusOK, dto.BlobSidecarsResponse{
		ExecutionOptimistic: false,
		Finalized:           bc.blobsService.IsFinalized(slot.Slot, headSlot),
		Data:                resBlobs,
	})
}

// BlobsByBlockIdV2 implements the Beacon API v4.0.0 endpoint that replaces
// the deprecated /eth/v1/beacon/blob_sidecars/{block_id}. The response is a
// flat list of blob byte strings, ordered by KZG commitment order in the
// block, and filterable by versioned hash rather than blob index.
//
// The `finalized` flag is derived heuristically from the slot's distance
// from head (≥ chain.finality_lag slots → finalized) rather than from the
// consensus-layer finality checkpoint. This is accurate under normal
// conditions but can over-report during extended non-finality periods.
// `execution_optimistic` is always false — the backing sink only persists
// fully-validated data.
//
//	@Summary	Get Blobs by block id (Beacon API v4.0.0)
//	@Tags		blobs
//	@Produce	json
//	@Param		block_id			path		string		true	"Block identifier. Can be one of: 'head', slot number, or 0x-prefixed hex block root"
//	@Param		versioned_hashes	query		[]string	false	"0x-prefixed 32-byte versioned hashes to filter by. Accepts repeated keys (?versioned_hashes=0x01..&versioned_hashes=0x01..) or comma-separated. Returns all blobs if omitted." collectionFormat(multi)
//	@Success	200		{object}	dto.BlobsResponse	"Successful response"
//	@Failure	400		{object}	response.ApiErrorResponse	"invalid_slot or invalid_versioned_hash"
//	@Failure	404		{object}	response.ApiErrorResponse	"slot_not_found"
//	@Failure	500		{object}	response.ApiErrorResponse
//	@Router		/eth/v1/beacon/blobs/{block_id} [get]
func (bc *BlobsController) BlobsByBlockIdV2(c *gin.Context) {

	blockId := c.Param("block_id")

	hashFilter := map[[32]byte]struct{}{}
	// Accept both comma-separated (?versioned_hashes=0x01...,0x01...) and
	// repeated-key (?versioned_hashes=0x01...&versioned_hashes=0x01...)
	// styles. Lighthouse uses the repeated form.
	for _, raw := range c.QueryArray("versioned_hashes") {
		for _, str := range strings.Split(raw, ",") {
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
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	slot, headSlot, err := bc.blobsService.GetSlotByBlockId(ctx, blockId)
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
		Finalized:           bc.blobsService.IsFinalized(slot.Slot, headSlot),
		Data:                data,
	})
}
