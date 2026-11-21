# Legacy envelope and v1 APIs

This page documents the original wire format of `github.com/jgolang/api`, where the client sends device metadata inside the JSON `header` of every request, and the APIs that belong to that flow. It is still supported for existing clients. New services should use the flow in the [README](../README.md): plain JSON bodies, request metadata in HTTP headers, and the `Query*` / `WriteError` helpers.

Nothing on this page has been removed. Names marked deprecated in the Go docs keep working; their doc comments say what to use instead.

## Enabling the legacy envelope

The default request mode is plain JSON. Turn the envelope back on with an instance and wrap the router with its `Handler`:

```go
a := api.New(
	api.WithRequestMode(api.RequestModeEnvelope),   // or api.RequestModeAuto to accept both
	api.WithResponseMode(api.ResponseModeEnvelope), // the default
)
http.ListenAndServe(":8080", a.Handler(router))
```

## Request structure

The request JSON object has two parts: `header` and `content`.

### Header

The `header` carries metadata about the request and the device making it: device type, brand, OS version, app version, location and security token.

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
        "app_name": "my-app",
        "lat": "19.4326",
        "lon": "-99.1332",
        "token": "your-security-token"
    },
    "content": {
        "example_key": "example_value"
    }
}
```

`envelope.RequestInfo` is the Go type of this header. Each field fills a field of the request context (`api.Context(r.Context())`) and has an HTTP header equivalent (`token` is read, never echoed back in the response since v1.0.0). **The HTTP header wins**: a `header` field is used only when its HTTP header is absent.

| JSON `header` field | HTTP header | Request context field |
| --- | --- | --- |
| `uuid` | `X-Request-ID`, else `UUID` | `UUID` |
| `device_type` | `DeviceType` | `DeviceType` |
| `device_brand` | `DeviceBrand` | `DeviceBrand` |
| `device_model` | `DeviceModel` | `DeviceModel` |
| `os` | `DeviceOS` | `DeviceOS` |
| `os_version` | `OSVersion` | `OSVersion` |
| `timezone` | `OSTimezone` | `OSTimezone` |
| `lang` | `AppLanguage`, else `Accept-Language` | `AppLanguage` |
| `app_version` | `AppVersion` | `AppVersion` |
| `app_build_version` | `AppBuildVersion` | `AppBuildInfo` |
| `app_name` | `AppName` | `AppName` |
| `token` | `SecurityToken`, else `Authorization: Bearer <token>` | `SecurityToken` |
| `device_serial` | `DeviceSerial` | `DeviceSerial` (default receiver only) |
| `device_id` | `DeviceID` | `DeviceId` (default receiver only) |
| `lat` | `Latitude` | `Latitude` (default receiver only) |
| `lon` | `Longitude` | `Longitude` (default receiver only) |

### Content

The `content` part holds the endpoint request object. After `api.ProcessRequest`, `r.Body` contains only `content`, so the handler decodes it the same way in every request mode.

## Response structure

The envelope response mode (the default) writes a `header` with the response metadata and a `content` with the endpoint response. `envelope.ResponseInfo` is the Go type of the header.

```json
{
    "header": {
        "title": "Request Successful",
        "message": "The request was processed successfully. (f3c509-0000)",
        "type": "success",
        "code": "0000",
        "token": "your-security-token",
        "event_id": "f3c50980e8c71811b25b2319f0daf5a0",
        "action": "example_action",
        "info": {"key": "value"}
    },
    "content": {
        "example_response_key": "example_response_value"
    }
}
```

Empty header fields are omitted, except `type`. Since v1.0.0 the response `token` is sent only when the handler sets `SecurityToken`; the request token (`header.token` above, the `SecurityToken` or `Authorization: Bearer` header) is no longer echoed. With the default `api.BlankSuccess = true`, success responses leave `title` and `message` empty; see [Responses](../README.md#responses) for `api.TraceVisibility` and the codes.

## The v1 request flow

```go
handler := api.MiddlewaresChain(api.ProcessRequest)(func(w http.ResponseWriter, r *http.Request) {
	rctx := api.Context(r.Context())
	var req CreateTaskRequest
	if err := rctx.DecodeContent(&req); err != nil {
		api.Error400().Write(w, r)
		return
	}
	api.SuccessWithContent(map[string]any{"device": rctx.DeviceType}).Write(w, r)
})
http.HandleFunc("/api/example", handler)
```

`api.ProcessRequest` calls the instance's `core.APIRequestReceiver` (the registered one on the default instance) (through `api.ProcessBody`), which reads the body, merges the HTTP headers and the envelope `header` as described above, and returns the request context. The middleware then assigns the event ID, logs the request, stores the context in the request and replaces `r.Body` with the content.

### Request receivers

Two exported implementations of `core.APIRequestReceiver` exist. Both follow the request mode for `ProcessBody`.

- `api.RequestReceiver` is the default, wired in `api.go`. `ProcessBody` fills every field in the table above. `ProcessEncryptedBody` reads a JSON body `{"data": "...", "deviceUUID": "..."}` (`envelope.EncryptedBody`).
- `api.RequestReceiverV2` must be passed with `api.New(api.WithRequestReceiver(api.RequestReceiverV2{}))`, or registered on the default instance with the deprecated `api.RegisterNewAPIRequestReceiver(api.RequestReceiverV2{})`. `ProcessBody` does not fill `DeviceSerial`, `DeviceId`, `Latitude` or `Longitude`. `ProcessEncryptedBody` reads the form or query values `data` and `metadata`, both required; a missing one returns a `codes.ValRequired` error (400 through `api.WriteError`).

`api.ProcessEncryptedBody(r)` runs `ProcessEncryptedBody` on the instance's receiver (the registered one on the default instance) and returns an `*api.EncryptedRequest`; decrypting `Data` is left to the application.

### Method maps and the body middleware

`api.RequestBody` (`api.NewRequestBodyMiddleware(api.PPPMethodsKey)`) runs `ProcessBody` only for the methods in a named method map and answers `E002` when the body cannot be parsed (413 for a body over the 10 MiB limit). The maps are `api.PPPGMethodsKey` (POST, PUT, PATCH, GET), `api.PPPMethodsKey` (POST, PUT, PATCH) and `api.PPMethodsKey` (POST, PUT), plus `api.MethodPostKey`, `api.MethodGetKey` and `api.MethodPutKey` for single methods. `api.AddNewMapMethod` adds a map and `api.ValidateMethods` checks a method against one.

### `api.Response`-returning helpers

These helpers predate `api.WriteError` and the `Query*` helpers. They return a ready-to-write `api.Response` (an `api.Error`) instead of an `error`, and log the underlying error: with `api.PrintError` on the default instance, through the `api.WithLogger` logger (or not at all) on an `api.New` instance.

| Helper | Prefer in new code |
| --- | --- |
| `api.GetQueryParamValueString`, `api.GetQueryParamValueInt`, `api.GetQueryParamValueBool` (deprecated) | `api.QueryString`, `api.QueryInt`, `api.QueryBool` |
| `api.GetQueryParamValueInt64`, `api.GetQueryParamValueFloat64` | no `Query*` equivalent yet |
| `api.GetHeaderValueString`, `api.GetHeaderValueInt`, `…Int64`, `…Float64`, `…Bool` | `r.Header.Get` and `strconv` |
| `api.GetRouteVarValueString`, `api.GetRouteVarValueInt`, `…Int64`, `…Float64`, `…Bool` | `api.GetRouteVar` and `strconv` |
| `api.UnmarshalBody` | `json.NewDecoder(r.Body).Decode` or `api.Context(r.Context()).DecodeContent`, then `api.WriteError` |
| `api.RequestBasic` (`GetSecurityToken`, `GetUserID`, `GetEventID`, `UnmarshalBody`, …) | `api.Context(r.Context())` fields |

```go
limit, resp := api.GetQueryParamValueInt("limit", r)
if resp != nil {
	resp.Write(w, r) // 400
	return
}
```
