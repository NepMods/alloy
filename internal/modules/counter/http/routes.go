package http

import (
	"alloy/internal/server"
	"alloy/models/apidocs"
)

func RouteDocs() []apidocs.RouteDoc {
	return []apidocs.RouteDoc{
		{
			Method:      "GET",
			Path:        "/v1/counter/add",
			Summary:     "Add Count and return",
			Auth:        "none",
			RequestBody: &server.CheckRequestHasNothing{},
			Response:    &server.CheckResponse{},
		},
	}
}
