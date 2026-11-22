package envelope

// ProblemDetails documents the RFC 9457 problem details body that
// ResponseModeProblem writes for an error. Code and EventID are extension
// members. It is for documentation only and does not change the wire.
type ProblemDetails struct {
	Type    string `json:"type" example:"about:blank"`
	Title   string `json:"title,omitempty" example:"Not Found"`
	Status  int    `json:"status,omitempty" example:"404"`
	Detail  string `json:"detail,omitempty" example:"task not found"`
	Code    string `json:"code,omitempty" example:"TSK-404"`
	EventID string `json:"event_id,omitempty" example:"f716243f2c92df55fcd8f67018b1dcfb"`
}

// Problem returns a problem details body for documenting an error in
// ResponseModeProblem.
func Problem() ProblemDetails {
	return ProblemDetails{}
}
