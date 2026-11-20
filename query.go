package api

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jgolang/errors"
	"github.com/jgolang/errors/codes"
)

// QueryOption changes how a typed query helper reports an invalid value.
type QueryOption func(*queryOptions)

type queryOptions struct {
	code codes.Coder
}

// WithQueryCode reports an invalid value with code instead of
// codes.ValFormat. WriteError answers 400 only when code has the validation
// kind (codes.KindValidation); a nil code is ignored.
func WithQueryCode(code codes.Coder) QueryOption {
	return func(options *queryOptions) {
		if code != nil {
			options.code = code
		}
	}
}

// QueryString returns the first value of the query parameter name. ok is
// false when the parameter is absent; a present but empty parameter returns ""
// with ok true. When the query string is malformed and name cannot be read,
// it returns ok true and a validation error, so a bad request is not taken for
// a missing parameter. Each call parses the query string.
func QueryString(r *http.Request, name string, opts ...QueryOption) (string, bool, error) {
	values, parseErr := url.ParseQuery(r.URL.RawQuery)
	if found, present := values[name]; present {
		return found[0], true, nil
	}
	if parseErr != nil {
		return "", true, invalidQuery(name, "readable (the query string is malformed)", opts)
	}
	return "", false, nil
}

// QueryInt parses the first value of the query parameter name with
// strconv.Atoi (signs allowed). ok is false when the parameter is absent. A
// present value that is empty or not an integer returns ok true and a
// validation error: codes.ValFormat, or the code from WithQueryCode.
func QueryInt(r *http.Request, name string, opts ...QueryOption) (int, bool, error) {
	raw, ok, err := QueryString(r, name, opts...)
	if !ok || err != nil {
		return 0, ok, err
	}
	value, parseErr := strconv.Atoi(raw)
	if parseErr != nil {
		return 0, true, invalidQuery(name, "an integer", opts)
	}
	return value, true, nil
}

// QueryBool parses the first value of the query parameter name with
// strconv.ParseBool. ok and the error follow QueryInt.
func QueryBool(r *http.Request, name string, opts ...QueryOption) (bool, bool, error) {
	raw, ok, err := QueryString(r, name, opts...)
	if !ok || err != nil {
		return false, ok, err
	}
	value, parseErr := strconv.ParseBool(raw)
	if parseErr != nil {
		return false, true, invalidQuery(name, "a boolean", opts)
	}
	return value, true, nil
}

// QueryEnum returns the first value of the query parameter name when it is
// one of allowed. ok and the error follow QueryInt. Document the same values
// with doc.QueryEnum.
func QueryEnum(r *http.Request, name string, allowed []string, opts ...QueryOption) (string, bool, error) {
	raw, ok, err := QueryString(r, name, opts...)
	if !ok || err != nil {
		return "", ok, err
	}
	for _, candidate := range allowed {
		if raw == candidate {
			return raw, true, nil
		}
	}
	return "", true, invalidQuery(name, "one of "+strings.Join(allowed, ", "), opts)
}

// invalidQuery builds the validation error for an invalid query parameter.
func invalidQuery(name string, expected string, opts []QueryOption) error {
	options := queryOptions{code: codes.ValFormat}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	return errors.NewC(options.code, "query parameter %q must be %s", name, expected)
}
