package api

import (
	"net/http"
	"strconv"

	"github.com/jgolang/errors"
	"github.com/jgolang/errors/codes"
)

// QueryOption changes how a typed query helper reports an invalid value.
type QueryOption func(*queryOptions)

type queryOptions struct {
	code codes.Coder
}

// WithQueryCode reports an invalid value with code instead of
// codes.ValFormat. Give it a validation kind so WriteError answers 400.
func WithQueryCode(code codes.Coder) QueryOption {
	return func(options *queryOptions) {
		options.code = code
	}
}

// QueryString returns the first value of the query parameter name. ok is false
// when the parameter is absent; a present but empty parameter returns "" with
// ok true.
func QueryString(r *http.Request, name string) (value string, ok bool) {
	values, present := r.URL.Query()[name]
	if !present || len(values) == 0 {
		return "", false
	}
	return values[0], true
}

// QueryInt parses the query parameter name as an int. ok is false when the
// parameter is absent; a present value that is empty or not an integer returns
// ok true and a validation error (codes.ValFormat unless WithQueryCode).
func QueryInt(r *http.Request, name string, opts ...QueryOption) (value int, ok bool, err error) {
	raw, ok := QueryString(r, name)
	if !ok {
		return 0, false, nil
	}
	value, parseErr := strconv.Atoi(raw)
	if parseErr != nil {
		return 0, true, invalidQuery(name, "an integer", opts)
	}
	return value, true, nil
}

// QueryBool parses the query parameter name with strconv.ParseBool. ok and the
// error follow QueryInt.
func QueryBool(r *http.Request, name string, opts ...QueryOption) (value bool, ok bool, err error) {
	raw, ok := QueryString(r, name)
	if !ok {
		return false, false, nil
	}
	value, parseErr := strconv.ParseBool(raw)
	if parseErr != nil {
		return false, true, invalidQuery(name, "a boolean", opts)
	}
	return value, true, nil
}

// QueryEnum returns the query parameter name when it is one of allowed. ok and
// the error follow QueryInt. Document the same values with doc.QueryEnum.
func QueryEnum(r *http.Request, name string, allowed []string, opts ...QueryOption) (value string, ok bool, err error) {
	raw, ok := QueryString(r, name)
	if !ok {
		return "", false, nil
	}
	for _, candidate := range allowed {
		if raw == candidate {
			return raw, true, nil
		}
	}
	return "", true, invalidQuery(name, "one of the allowed values", opts)
}

// invalidQuery builds the validation error for an invalid query parameter.
func invalidQuery(name string, expected string, opts []QueryOption) error {
	options := queryOptions{code: codes.ValFormat}
	for _, opt := range opts {
		opt(&options)
	}
	return errors.NewC(options.code, "query parameter %q must be %s", name, expected)
}
