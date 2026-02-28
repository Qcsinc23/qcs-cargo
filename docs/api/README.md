# QCS Cargo API Docs

This folder contains the OpenAPI 3.1 spec for core API endpoints:

- `openapi.yaml`

## Validate the spec

From repo root:

```bash
npm install --no-save @redocly/cli @apidevtools/swagger-cli
npx swagger-cli validate docs/api/openapi.yaml
npx redocly lint docs/api/openapi.yaml
```

## Render docs locally

Option 1 (Redoc static HTML):

```bash
npx redocly build-docs docs/api/openapi.yaml -o docs/api/redoc.html
```

Open `docs/api/redoc.html` in a browser.

Option 2 (Swagger UI with Docker):

```bash
docker run --rm -p 8081:8080 \
  -e SWAGGER_JSON=/spec/openapi.yaml \
  -v "$PWD/docs/api":/spec \
  swaggerapi/swagger-ui
```

Then open `http://localhost:8081`.

## Notes

- This spec documents behavior currently implemented in Go handlers under `internal/api`.
- Authenticated routes use JWT bearer tokens (`Authorization: Bearer <token>`).
- Auth refresh/logout also use `qcs_refresh` cookie; CSRF origin/referer checks apply when that cookie is sent on mutating requests.
