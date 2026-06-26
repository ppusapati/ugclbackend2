# Frontend API Contract Note

These frontend clients now align with the backend performance contract for dynamic forms and submissions.

## Form fetch endpoints

- `GET /business/{businessCode}/forms`
- `GET /business/{businessCode}/forms/{formCode}`
- `GET /admin/app-forms`

These endpoints may return `ETag` headers.
Clients should send `If-None-Match` on repeat requests and reuse cached bodies when the backend responds with `304 Not Modified`.

## Submission list endpoints

- `GET /business/{businessCode}/forms/{formCode}/submissions`

This endpoint now defaults to cursor pagination for hot paths.
Pagination responses may include:

- `submissions`
- `count`
- `limit`
- `has_more`
- `next_cursor`

Legacy `limit` and `offset` usage is still supported for compatibility, but new callers should prefer `cursor` pagination when available.
