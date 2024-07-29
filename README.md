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

### Content

The `content` part contains the actual data for the specific endpoint request. This is where the endpoint-specific request object goes.

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

## Contributing

If you have suggestions for how We could be improved, or want to report a bug, open an issue! We'd love all and any contributions.

For more, check out the [Contributing Guide](CONTRIBUTING.md).

## License

This project is licensed under the [MIT License](https://github.com/new-horizons-tech-group/golang-project-tmpl/blob/main/LICENSE).
