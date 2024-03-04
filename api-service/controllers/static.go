package controllers

import (
	"blob-service/flags"

	"github.com/eosnationftw/eosn-base-api/response"
	"github.com/gin-gonic/gin"
)

type VersionResponse struct {
	Version  string          `json:"version"`
	Commit   string          `json:"commit"`
	Features []flags.Feature `json:"enabled_features" swaggertype:"array,string"`
}

// Version
// @Summary Returns the version, commit hash and enabled features of this API.
// @Tags version
// @Produce json
// @Success 200 {object} response.ApiDataResponse{data=VersionResponse}
// @Failure 500 {object} response.ApiErrorResponse
// @Router /version [get]
func Version(c *gin.Context) {
	response.OkDataResponse(c, &response.ApiDataResponse{Data: &VersionResponse{
		Version:  flags.GetVersion(),
		Commit:   flags.GetShortCommit(),
		Features: flags.GetEnabledFeatures(),
	}})
}
