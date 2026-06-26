package http

import "alloy/models/apidocs"

func RouteDocs() []apidocs.RouteDoc {
	return []apidocs.RouteDoc{
		{
			Method:  "POST",
			Path:    "/v1/auth/register",
			Summary: "Register a new user",
			Auth:    "none",
		},
		{
			Method:  "POST",
			Path:    "/v1/auth/login",
			Summary: "Log in with email and password",
			Auth:    "none",
		},
	}
}
