# API Endpoint Spec Template

## REQ-001: Endpoint Definition

**Title**: [Endpoint name]
**Method**: GET | POST | PUT | PATCH | DELETE
**Path**: `/api/v1/[resource]`
**Content-Type**: application/json

### Request

```json
{
  "field": "type | description"
}
```

### Response (200)

```json
{
  "field": "type | description"
}
```

### Errors

| Code | Description |
|------|-------------|
| 400 | Invalid request body |
| 401 | Unauthenticated |
| 403 | Forbidden |
| 404 | Resource not found |
| 500 | Internal error |

### Scenario: Successful request

GIVEN a valid request payload
WHEN the endpoint is called
THEN it returns 200 with the expected response body

### Scenario: Validation error

GIVEN an invalid request payload (missing required field)
WHEN the endpoint is called
THEN it returns 400 with a validation error message

### Scenario: Authentication required

GIVEN no authentication token
WHEN the endpoint is called
THEN it returns 401

## REQ-002: Authorization

**Title**: Access control for [endpoint]
**Description**: [Who can access this endpoint and what roles are required]

GIVEN a user with role [role]
WHEN they call the endpoint
THEN they [can/cannot] access the resource

## REQ-003: Rate Limiting

**Title**: Rate limit for [endpoint]
**Description**: [Rate limit configuration]

GIVEN [N] requests in [time] window
WHEN the [N+1]th request is sent
THEN it returns 429 Too Many Requests
