package api

import "github.com/jgolang/api/core"

// ResponseFormatter doc ...
type ResponseFormatter struct{}

// Format doc ...
func (f ResponseFormatter) Format(data core.ResponseData) *core.ResponseFormatted {
	return &core.ResponseFormatted{
		StatusCode: data.StatusCode,
		Headers:    data.Headers,
		Data: JSONResponse{
			Content: data.Data,
			Info: JSONResponseInfo{
				Title:          data.Title,
				Message:        data.Message,
				Type:           data.ResponseType,
				Action:         data.Action,
				SessionID:      data.SessionID,
				ErrorCode:      data.ErrorCode,
				AdditionalInfo: data.AdditionalInfo,
			},
		},
	}
}
