# 0007 — Component responses come from route options

- Status: accepted (2026-11-25); the limit on options with several responses documented in v1.8.1
- Initiative: api docs match the wire (epic "errors from codes", story 2)
- PRs: #110 (commit `d5afdc2`), #113 (the documented limit)
- Release: v1.6.0 (MINOR: `Docs.AddResponse`, `doc.ResponseRef`, `RouteResponse.Ref`, `Components.Responses`, `ResponseObject.Ref`)

## Context

A response shared by many routes (a 404, a 401, an error response built by an instance) was written in full under every operation, so the spec repeated it and an edit had to be made per route. OpenAPI has `components.responses` for this, referenced with `$ref`.

The bodies worth sharing are mostly built by an instance: `(*api.API).DocError` and `DocErrors` return a `doc.RouteOption` shaped by the instance's response mode and formatter (ADR 0004). `doc` must not import `api` (ADR 0001 §3), so a component API that took anything other than a `RouteOption` could not reuse them.

## Decision

1. **A component is registered from a route option.** `(*doc.Docs).AddResponse(name string, response doc.RouteOption)` applies the option to a scratch `doc.Route` when it is called and keeps a copy of the one response it added. Any response option works: `doc.ResponseSchema`, `doc.ResponseExamples`, `a.DocError`, or `a.DocErrors` over codes of one status. The instance gets no new method.
2. **Exactly one response.** An option that adds no response or more than one registers nothing, without an error or log, since picking one of several would be arbitrary. `a.DocRejections()` (four statuses) and `a.DocErrors` over codes of several statuses are such options. An empty name or a nil option is ignored too, and adding a name again replaces its response.
3. **The referencing route chooses the status.** `doc.ResponseRef(status, name)` writes `{"$ref": "#/components/responses/<name>"}` under the route's `status`. The option's own status is not where the component is documented; it only shapes the component as the option builds it, for example its default description (the status text). Like every response option, a later option for the same status replaces it.
4. **Same rules as an operation response.** Components are written with the code that writes operation responses (description, headers, schema, examples), their Go body types go to `components.schemas`, and a spec that does not call the new names is unchanged.
5. **References are not checked.** A `ResponseRef` to a name never added is written as is; the spec validation in `contracttest` catches the dangling `$ref` (owner, story 3.2 Q2).

## Consequences

- One call turns an instance-built error response into a component: `docs.AddResponse("TaskNotFound", a.DocError(http.StatusNotFound, "Task not found"))`, then `doc.ResponseRef(http.StatusNotFound, "TaskNotFound")` on each route.
- A component's description can name a status other than the one a route documents it under (a 404 option referenced as 410 keeps "Not Found"); the author sets a description when that matters.
- An option with several responses cannot be a component, silently. Since v1.8.1 the `AddResponse` godoc, the README and `docs/openapi.md` say so: such responses are added to each route instead. v1.8.1 also made `DocErrors` document every status its codes are sent with, so two codes that share an example name but not a status now add two responses and no longer register a component.
- A struct that embeds `doc.ResponseObject` now marshals through its `MarshalJSON`.

## Alternatives

- **`AddResponse(name string, response doc.RouteResponse)`.** An explicit struct, but reusing an instance error response would need the instance to expose its `RouteResponse` or a new method per builder. Owner chose the option form (story 3.2 Q1).
- **Use the option's status.** The component would carry a status that OpenAPI does not store on it, and the same body could not be referenced under two statuses.
- **Keep the first response, or one component per response, for options with several.** The first would be arbitrary; several components need a design for referencing them together. The owner kept the limit documented only (v1.8.1, YAGNI-01).
