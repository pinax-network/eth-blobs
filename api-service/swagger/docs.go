// Package swagger Code generated by swaggo/swag. DO NOT EDIT
package swagger

import "github.com/swaggo/swag"

const docTemplate = `{
    "schemes": {{ marshal .Schemes }},
    "swagger": "2.0",
    "info": {
        "description": "{{escape .Description}}",
        "title": "{{.Title}}",
        "contact": {},
        "version": "{{.Version}}"
    },
    "host": "{{.Host}}",
    "basePath": "{{.BasePath}}",
    "paths": {
        "/eth/v1/beacon/blob_sidecars/{block_id}": {
            "get": {
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "blobs"
                ],
                "summary": "Get Blobs by block id",
                "parameters": [
                    {
                        "type": "string",
                        "description": "Block identifier. Can be one of: 'head', slot number, hex encoded blockRoot with 0x prefix",
                        "name": "block_id",
                        "in": "path",
                        "required": true
                    },
                    {
                        "type": "array",
                        "items": {
                            "type": "string"
                        },
                        "collectionFormat": "csv",
                        "description": "Array of indices for blob sidecars to request for in the specified block. Returns all blob sidecars in the block if not specified.",
                        "name": "indices",
                        "in": "query"
                    }
                ],
                "responses": {
                    "200": {
                        "description": "Successful response",
                        "schema": {
                            "allOf": [
                                {
                                    "$ref": "#/definitions/response.ApiDataResponse"
                                },
                                {
                                    "type": "object",
                                    "properties": {
                                        "data": {
                                            "type": "array",
                                            "items": {
                                                "$ref": "#/definitions/dto.Blob"
                                            }
                                        }
                                    }
                                }
                            ]
                        }
                    },
                    "400": {
                        "description": "invalid_slot\"\t\"Invalid block id",
                        "schema": {
                            "$ref": "#/definitions/response.ApiErrorResponse"
                        }
                    },
                    "404": {
                        "description": "slot_not_found\"\t\"Slot not found",
                        "schema": {
                            "$ref": "#/definitions/response.ApiErrorResponse"
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "$ref": "#/definitions/response.ApiErrorResponse"
                        }
                    }
                }
            }
        },
        "/health": {
            "get": {
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "health"
                ],
                "summary": "Returns health status of this API.",
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/controllers.HealthResponse"
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "$ref": "#/definitions/response.ApiErrorResponse"
                        }
                    }
                }
            }
        },
        "/version": {
            "get": {
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "version"
                ],
                "summary": "Returns the version, commit hash and enabled features of this API.",
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "allOf": [
                                {
                                    "$ref": "#/definitions/response.ApiDataResponse"
                                },
                                {
                                    "type": "object",
                                    "properties": {
                                        "data": {
                                            "$ref": "#/definitions/controllers.VersionResponse"
                                        }
                                    }
                                }
                            ]
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "$ref": "#/definitions/response.ApiErrorResponse"
                        }
                    }
                }
            }
        }
    },
    "definitions": {
        "controllers.HealthResponse": {
            "type": "object",
            "properties": {
                "detail": {
                    "type": "string"
                },
                "head": {
                    "type": "integer"
                },
                "status": {
                    "type": "string"
                }
            }
        },
        "controllers.VersionResponse": {
            "type": "object",
            "properties": {
                "commit": {
                    "type": "string"
                },
                "enabled_features": {
                    "type": "array",
                    "items": {
                        "type": "string"
                    }
                },
                "version": {
                    "type": "string"
                }
            }
        },
        "dto.Blob": {
            "type": "object",
            "properties": {
                "blob": {
                    "type": "array",
                    "items": {
                        "type": "integer"
                    }
                },
                "index": {
                    "type": "integer"
                },
                "kzg_commitment": {
                    "type": "array",
                    "items": {
                        "type": "integer"
                    }
                },
                "kzg_commitment_inclusion_proof": {
                    "type": "array",
                    "items": {
                        "type": "array",
                        "items": {
                            "type": "integer"
                        }
                    }
                },
                "kzg_proof": {
                    "type": "array",
                    "items": {
                        "type": "integer"
                    }
                },
                "signed_block_header": {
                    "$ref": "#/definitions/dto.SignedBlockHeader"
                }
            }
        },
        "dto.Message": {
            "type": "object",
            "properties": {
                "body_root": {
                    "type": "array",
                    "items": {
                        "type": "integer"
                    }
                },
                "parent_root": {
                    "type": "array",
                    "items": {
                        "type": "integer"
                    }
                },
                "proposer_index": {
                    "type": "integer"
                },
                "slot": {
                    "type": "integer"
                },
                "state_root": {
                    "type": "array",
                    "items": {
                        "type": "integer"
                    }
                }
            }
        },
        "dto.SignedBlockHeader": {
            "type": "object",
            "properties": {
                "message": {
                    "$ref": "#/definitions/dto.Message"
                },
                "signature": {
                    "type": "array",
                    "items": {
                        "type": "integer"
                    }
                }
            }
        },
        "response.ApiDataResponse": {
            "type": "object",
            "properties": {
                "data": {},
                "meta": {}
            }
        },
        "response.ApiError": {
            "type": "object",
            "properties": {
                "code": {
                    "type": "string"
                },
                "detail": {
                    "type": "string"
                },
                "status": {
                    "type": "integer"
                }
            }
        },
        "response.ApiErrorResponse": {
            "type": "object",
            "properties": {
                "errors": {
                    "type": "array",
                    "items": {
                        "$ref": "#/definitions/response.ApiError"
                    }
                }
            }
        }
    }
}`

// SwaggerInfo holds exported Swagger Info so clients can modify it
var SwaggerInfo = &swag.Spec{
	Version:          "1.0",
	Host:             "localhost:8080",
	BasePath:         "",
	Schemes:          []string{"http", "https"},
	Title:            "Ethereum Blobs REST API",
	Description:      "Use this API to get EIP-4844 blobs as a drop-in replacement for Consensus Layer clients API.",
	InfoInstanceName: "swagger",
	SwaggerTemplate:  docTemplate,
	LeftDelim:        "{{",
	RightDelim:       "}}",
}

func init() {
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}
