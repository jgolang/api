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

// SecurityTokenHeaderKey session ID header key.
var SecurityTokenHeaderKey = "SecurityToken"

// UserIDHeaderKey User ID header key.
var UserIDHeaderKey = "UserID"

// EventIDHeaderKey event ID header key.
var EventIDHeaderKey = "EventID"

// GetSecurityToken gets security token from user by header key layout.
func (request *RequestBasic) GetSecurityToken() Response {
	sessionID, response := GetHeaderValueString(SecurityTokenHeaderKey, request.HTTPReq)
	if response != nil {
		resp := response.(Error)
		resp.Title = "Request info error!"
		resp.Message = "The session id was not obtained"
		return response
	}
	request.SessionID = sessionID
	return nil
}

// GetUserID gets id user ID by header key layout.
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

// GetEventID gets event ID by header key layout.
func (request *RequestBasic) GetEventID() Response {
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

// UnmarshalBody parses request body to a struct.
func (request *RequestBasic) UnmarshalBody() Response {
	resp := UnmarshalBody(request.JSONStruct, request.HTTPReq)
	if resp != nil {
		return resp
	}
	return nil
}

// GetRequestBasicInfo gets session ID, user ID and event ID.
func (request *RequestBasic) GetRequestBasicInfo() Response {
	resp := request.GetSecurityToken()
	if resp != nil {
		return resp
	}
	resp = request.GetUserID()
	if resp != nil {
		return resp
	}
	resp = request.GetEventID()
	if resp != nil {
		return resp
	}
	return nil
}

// GetRequestFullInfo gets session ID, userID, event ID and unmarshal request body.
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
