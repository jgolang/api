package api

import "github.com/jgolang/api/core"

// ResponseFormatter implementation of core.APIResponseFormatter interface.
type ResponseFormatter struct{}

// Format the response body information.
func (formatter ResponseFormatter) Format(data core.ResponseData) *core.ResponseFormatted {
	return &core.ResponseFormatted{
		HTTPStatusCode: data.HTTPStatusCode,
		Headers:        data.Headers,
		Body: JSONResponse{
			Content: data.Content,
			Header: JSONResponseInfo{
				Title:   data.Title,
				Message: data.Message,
				Type:    data.ResponseType,
				Action:  data.Actions,
				Token:   data.SecurityToken,
				Code:    string(data.ResponseCode),
				EventID: data.EventID,
				Info:    data.Info,
			},
		},
	}
}
