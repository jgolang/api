# API Developer Helper Library

## Overview

This library is designed to standardize communication for API requests and responses in a microframework. It provides a consistent structure for headers and content, ensuring uniformity across different endpoints. The library helps developers easily wrap their endpoint-specific logic within a predefined request and response format.

## Request Structure

The request JSON object consists of two main parts: `header` and `content`.

### Header

The `header` contains metadata about the request and the device making the request. This includes information like device type, brand, OS version, and security token.

```json
{
    "header": {
        "uuid": "2e67ee64-fb5e-11ed-be56-0242ac120003",
        "device_type": "user",
        "device_brand": "postman",
        "device_serial": "postman_device_serial",
        "device_id": "postman_device_id",
        "device_model": "postman",
        "os": "postman",
        "os_version": "0.0.0",
        "lang": "es",
        "timezone": "-6",
        "app_version": "1.3.0",
        "app_build_version": "0.1.0",
        "device_id": "",
        "device_serial": "",
        "lat": "",
        "lon": "",
        "token": "" // security token
    },
    "content": {
        // specific endpoint request object
    }
}
```

With the default `api.BlankSuccess = true`, success responses leave `title` and `message` empty. Error, warning, informative and custom response types always carry them, and with the default `api.TraceVisibility = 1` the message ends with ` (<event id>-<code>)` (an empty message gets no suffix).

### Content

The `content` part contains the actual data for the specific endpoint request. This is where the endpoint-specific request object goes.


## Query parameters

`api.QueryInt`, `api.QueryBool`, `api.QueryEnum` and `api.QueryString` read a query parameter and tell absent from invalid. `ok` is false when the parameter is absent; an invalid value returns `ok` true and a validation error (`codes.ValFormat`, or your code through `api.WithQueryCode`), which `api.WriteError` answers with 400.

```go
limit, ok, err := api.QueryInt(r, "limit", api.WithQueryCode(CodeLimitInvalid))
if err != nil {
	api.WriteError(w, r, err) // 400
	return
}
if !ok {
	limit = 20
}
```

## Writing errors

`api.WriteError(w, r, err)` answers with the status of the error's code kind (`jgolang/errors` v0.4.0+): validation 400, unauthorized 401, forbidden 403, not_found 404, conflict 409, and 500 for infrastructure or a code without a kind. The response code and message come from the same code, also behind `fmt.Errorf`. `api.StatusForKind` exposes the mapping.

```go
var ErrTaskNotFound = errors.Define(codes.New("PLN-111", "task not found").WithKind(codes.KindNotFound), "task not found")

func getTask(w http.ResponseWriter, r *http.Request) {
	task, err := tasks.Find(r.PathValue("id")) // your code; returns errors.Wrap(ErrTaskNotFound)
	if err != nil {
		api.WriteError(w, r, err) // 404
		return
	}
	api.SuccessWithContent(task).Write(w, r)
}
```

401 and 403 use the unauthorized title, like `Error401`/`Error403`; a nil error answers 500. `ExampleWriteError` in `example_write_error_test.go` is the compiled version.

## Response Structure

The response JSON object also consists of two main parts: `header` and `content`.

### Header

The `header` includes metadata about the response, such as the response status, messages, and security token.

```json
{
    "header": {
        "title": "",
        "message": "",
        "type": "success",
        "code": "000",
        "token": "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpZCI6IjNjMWM3ODBlLWQxMDAtNGEwZS05MTc3LTc1ZGRmY2Q0ZWU4MSIsInR5cGUiOiJhcHAiLCJleHAiOjE3MDkzOTQ0MTV9.f9I97DpJA1D2ahxq9-edCNnVOZVoLYBoQwuvAJf6F_8",
        "event_id": "f3c50980e8c71811b25b2319f0daf5a0",
        "action": "",
        "event_id": ""
    },
    "content": {
        // Specific endpoint response
    }
}
```

### Content

The `content` part contains the actual data for the specific endpoint response. This is where the endpoint-specific response object goes.

## Usage

### Implementing a Standard Request

To implement a standard request using this library, follow these steps:

1. **Create the Request Object:**
   - Fill in the `header` with the required metadata.
   - Add the specific endpoint request object within the `content`.

2. **Send the Request:**
   - Use the appropriate method (e.g., HTTP POST) to send the request to the endpoint.

### Example Request

```json
{
    "header": {
        "uuid": "2e67ee64-fb5e-11ed-be56-0242ac120003",
        "device_type": "user",
        "device_brand": "postman",
        "device_serial": "postman_device_serial",
        "device_id": "postman_device_id",
        "device_model": "postman",
        "os": "postman",
        "os_version": "0.0.0",
        "lang": "es",
        "timezone": "-6",
        "app_version": "1.3.0",
        "app_build_version": "0.1.0",
        "token": "your-security-token"
    },
    "content": {
        "example_key": "example_value"
    }
}
```

### Implementing a Standard Response

To implement a standard response using this library, follow these steps:

1. **Create the Response Object:**
   - Fill in the `header` with the response metadata.
   - Add the specific endpoint response object within the `content`.

2. **Return the Response:**
   - Return the response object as a JSON response to the client.

### Example Response

```json
{
    "header": {
        "title": "Request Successful",
        "message": "The request was processed successfully.",
        "type": "success",
        "code": "000",
        "token": "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpZCI6IjNjMWM3ODBlLWQxMDAtNGEwZS05MTc3LTc1ZGRmY2Q0ZWU4MSIsInR5cGUiOiJhcHAiLCJleHAiOjE3MDkzOTQ0MTV9.f9I97DpJA1D2ahxq9-edCNnVOZVoLYBoQwuvAJf6F_8",
        "event_id": "f3c50980e8c71811b25b2319f0daf5a0",
        "action": "example_action"
    },
    "content": {
        "example_response_key": "example_response_value"
    }
}
```

## Golang Implementation Example

Below is an example of how to use this library in a Golang project to create a standard success response.
To use this library, you need to use the middleware function `api.ProcessRequest()`
> See other helpful midlewares in the file `./middleware.go`

### Example Usage in an API Handler

```go
package main

import (
	"net/http"

	"github.com/jgolang/api"
)

func handler(w http.ResponseWriter, r *http.Request) {
	response := api.Success200()
	response.Content = map[string]interface{}{
		"key": "value",
	}
	response.Write(w, r)
}

func main() {
    middlewaresChain := MiddlewaresChain(middleware.ProcessRequest)
	http.HandleFunc("/api/example", middlewaresChain(handler))
	http.ListenAndServe(":8080", nil)
}
```

In this example, the `handler` function creates a standard success response with a status code of 200 and some content. It then writes this response to the HTTP response writer. This ensures that all responses follow the same structure and include the necessary metadata.

## Router-agnostic OpenAPI documentation

The package includes a router-agnostic registration layer for OpenAPI 3 and Swagger UI. The native adapter uses only Go's standard `net/http` package, while adapters for Gin, Echo, chi, gorilla/mux, or other routers should live in the application that chooses those routers.

The philosophy is to keep `github.com/jgolang/api` small and predictable: prefer Go's standard library and `github.com/jgolang/...` dependencies, with any exception made explicitly.

```go
package main

import (
	"net/http"

	"github.com/jgolang/api"
	"github.com/jgolang/api/doc"
	"github.com/jgolang/api/envelope"
	"github.com/jgolang/api/stdadapter"
)

type CreateTaskRequest struct {
	Title string `json:"title"`
}

type TaskResponse struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

func createTask(w http.ResponseWriter, r *http.Request) {
	api.Success201().Write(w, r)
}

func listTasks(w http.ResponseWriter, r *http.Request) {
	api.Success200().Write(w, r)
}

func main() {
	docs := doc.New(doc.API{
		Title:   "Tasks API",
		Version: "1.0.0",
	})
	docs.AddSecurityScheme("bearerAuth", doc.BearerSecurity("JWT"))
	// Optional: require bearerAuth everywhere and mark exceptions with doc.Public().
	// docs.SetDefaultSecurity("bearerAuth")

	router := stdadapter.New(http.NewServeMux(), docs)

	api.Post(router, "/tasks", createTask,
		doc.Summary("Create task"),
		doc.Tags("tasks"),
		doc.Body(envelope.Request[CreateTaskRequest]()),
		doc.Security("bearerAuth"),
		doc.Status(201, envelope.Success[TaskResponse]()),
	)

	api.Get(router, "/tasks", listTasks,
		doc.Summary("List tasks"),
		doc.Tags("tasks"),
		doc.Status(200, envelope.Success[[]TaskResponse]()),
	)

	router.Handle("GET", "/openapi.json", doc.OpenAPIHandler(docs))
	router.Handle("GET", "/docs", doc.SwaggerUIHandler("/openapi.json"))

	http.ListenAndServe(":8080", router)
}
```

See [docs/openapi.md](docs/openapi.md) for security schemes, schema tags, response aliases, and external adapter examples.

## Trace IDs

The package does not depend on a tracing vendor. Applications can provide trace IDs through `api.RegisterTraceIDProvider(...)`; see [docs/tracing.md](docs/tracing.md).

## Contributing

If you have suggestions for how We could be improved, or want to report a bug, open an issue! We'd love all and any contributions.

For more, check out the [Contributing Guide](CONTRIBUTING.md).

## License

This project is licensed under the [MIT License](LICENSE).

## Support

If you find this repository helpful and would like to support its development, consider making a donation. Your contributions will help ensure the continued improvement and maintenance of this repository.

Thank you for your support!

[![ko-fi](https://www.ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/josuegiron)
