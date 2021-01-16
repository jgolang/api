package api

import (
	"net/http"
)

// RequestBasic doc ...
type RequestBasic struct {
	JSONStruct interface{}
	SessionID  string
	UserID     string
	EventID    string
	HTTPReq    *http.Request
}

// SessionIDHeaderKey doc ...
var SessionIDHeaderKey = "SessionID"

// UserIDHeaderKey doc ...
var UserIDHeaderKey = "UserID"

// EventIDHeaderKey doc ..
var EventIDHeaderKey = "EventID"

//GetSessionID get session from user
func (request *RequestBasic) GetSessionID() Response {
	sessionID, response := GetHeaderValueString(SessionIDHeaderKey, request.HTTPReq)
	if response != nil {
		resp := response.(Error)
		resp.Title = "Request info error!"
		resp.Message = "The session id was not obtained"
		return response
	}
	request.SessionID = sessionID
	return nil
}

//GetUserID get id user session
func (request *RequestBasic) GetUserID() Response {
	userID, response := GetHeaderValueString(UserIDHeaderKey, request.HTTPReq)
	if response != nil {
		resp := response.(Error)
		resp.Title = "Request info error!"
		resp.Message = "The user id was not obtained"
		return response
	}
	request.UserID = userID
	return nil
}

//GetTraceID doc
func (request *RequestBasic) GetTraceID() Response {
	eventID, response := GetHeaderValueString(EventIDHeaderKey, request.HTTPReq)
	if response != nil {
		resp := response.(Error)
		resp.Title = "Request info error!"
		resp.Message = "The event id was not obtained"
		return response
	}
	request.EventID = eventID
	return nil
}

// UnmarshalBody doc ...
func (request *RequestBasic) UnmarshalBody() Response {
	resp := UnmarshalBody(request.JSONStruct, request.HTTPReq)
	if resp != nil {
		return resp
	}
	return nil
}

// GetRequestBasicInfo ..
func (request *RequestBasic) GetRequestBasicInfo() Response {
	resp := request.GetSessionID()
	if resp != nil {
		return resp
	}
	resp = request.GetUserID()
	if resp != nil {
		return resp
	}
	resp = request.GetTraceID()
	if resp != nil {
		return resp
	}
	return nil
}

// GetRequestFullInfo ..
func (request *RequestBasic) GetRequestFullInfo() Response {
	resp := request.GetRequestBasicInfo()
	if resp != nil {
		return resp
	}
	resp = request.UnmarshalBody()
	if resp != nil {
		return resp
	}
	return nil
}
