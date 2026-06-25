package http

import (
	"alloy/internal/server"
	"alloy/models/apidocs"
)

func RouteDocs() []apidocs.RouteDoc {
	return []apidocs.RouteDoc{
		{
			Method:      "GET",
			Path:        "/helloworld",
			Summary:     "SayHelloWorld",
			Auth:        "none",
			RequestBody: &server.CheckRequestHasNothing{},
			Response:    &server.CheckResponse{},
		},
	}
}
