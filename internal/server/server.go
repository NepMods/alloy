package server

import "alloy/models/apidocs"

type CheckRequestHasNothing struct{}
type CheckResponse struct {
	Status string `json:"status"`
}

var serverApiDocs = []apidocs.RouteDoc{
	{
		Method:      "GET",
		Path:        "/",
		Summary:     "Main endpoint for the server, returns a simple status message",
		Auth:        "none",
		RequestBody: CheckRequestHasNothing{}, // no body expected, but using POST to avoid URL length limits with query params
		Response:    CheckResponse{Status: "ok"},
	},
	{
		Method:      "GET",
		Path:        "/health",
		Summary:     "Check the health of the server",
		Auth:        "none",
		RequestBody: CheckRequestHasNothing{}, // no body expected, but using POST to avoid URL length limits with query params
		Response:    CheckResponse{Status: "ok"},
	},
	{
		Method:      "GET",
		Path:        "/ready",
		Summary:     "Check if the server is ready to accept requests",
		Auth:        "none",
		RequestBody: CheckRequestHasNothing{},
		Response:    CheckResponse{Status: "ok"},
	},
}

func RouteDocs() []apidocs.RouteDoc {
	return serverApiDocs
}
