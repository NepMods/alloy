package http

import (
	"alloy/internal/server"
	"alloy/models/apidocs"
)

func RouteDocs() []apidocs.RouteDoc {
	return []apidocs.RouteDoc{
		{
			Method:      "POST",
			Path:        "/auth/register",
			Summary:     "Register a new user",
			Auth:        "none",
			RequestBody: &server.CheckRequestHasNothing{},
			Response:    &server.CheckResponse{},
		},
	}
}
