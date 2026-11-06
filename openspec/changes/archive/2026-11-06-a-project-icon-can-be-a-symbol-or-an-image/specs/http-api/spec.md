## ADDED Requirements

### Requirement: Project icon endpoint

A project read SHALL carry `icon`, `icon_name` and `icon_image` as present, the last being the
digest of an uploaded image. Create and update SHALL accept `icon_name` and `icon_file`, an object
of `content_type` and base64 `data`, beside `icon`. The API SHALL expose
`GET /api/v1/projects/{ref}/icon`, answering with the image bytes, their content type,
`X-Content-Type-Options: nosniff`, a sandboxing content security policy and an ETag, and with not
found when the project has no uploaded icon.

#### Scenario: Upload and fetch over HTTP

- **WHEN** a client patches a project with `icon_file` holding a PNG and then requests `GET /api/v1/projects/{ref}/icon`
- **THEN** the patch answers with a project carrying `icon_image`, and the fetch answers 200 with the same bytes and `Content-Type: image/png`

#### Scenario: A project without an image

- **WHEN** a client requests the icon of a project whose icon is text
- **THEN** the response is 404 in the error envelope
