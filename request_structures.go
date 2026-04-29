package api

import "github.com/jgolang/api/envelope"

// JSONRequest struct used to parse the request content section.
type JSONRequest = envelope.JSONRequest

// JSONRequestInfo request info section fields for encrypted requests.
type JSONRequestInfo = envelope.RequestInfo

// JSONEncryptedBody struct used to parse the encrypted request and response body.
type JSONEncryptedBody = envelope.EncryptedBody
