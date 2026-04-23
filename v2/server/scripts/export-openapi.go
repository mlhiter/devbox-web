package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	syaml "sigs.k8s.io/yaml"
)

const (
	serverJSONName = "openapi-v2-server.json"
	serverYAMLName = "openapi-v2-server.yaml"
)

func main() {
	spec := buildSpec()

	jsonBytes, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		fail("marshal OpenAPI JSON", err)
	}
	jsonBytes = append(jsonBytes, '\n')

	yamlBytes, err := syaml.JSONToYAML(jsonBytes)
	if err != nil {
		fail("convert OpenAPI JSON to YAML", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		fail("resolve working directory", err)
	}

	docsDir := filepath.Clean(filepath.Join(wd, "..", "..", "docs"))
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		fail("create docs directory", err)
	}

	jsonPath := filepath.Join(docsDir, serverJSONName)
	yamlPath := filepath.Join(docsDir, serverYAMLName)

	if err := os.WriteFile(jsonPath, jsonBytes, 0o644); err != nil {
		fail("write JSON document", err)
	}
	if err := os.WriteFile(yamlPath, yamlBytes, 0o644); err != nil {
		fail("write YAML document", err)
	}

	fmt.Printf("Generated %s\n", jsonPath)
	fmt.Printf("Generated %s\n", yamlPath)
}

func fail(action string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", action, err)
	os.Exit(1)
}

func buildSpec() map[string]any {
	return map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":   "Devbox REST API (v2/server)",
			"version": "1.0.0",
			"description": "OpenAPI contract for the standalone Devbox REST API served by `v2/server`.\n\n" +
				"## Authentication\n\n" +
				"All business endpoints require `Authorization: Bearer <JWT>`.\n" +
				"The JWT must use `HS256` and include a `namespace` claim. Namespace is derived from the token only.\n\n" +
				"## Response Model\n\n" +
				"Successful JSON responses use the envelope `{ code, message, data }`.\n" +
				"Error responses use `{ code, message }`.\n" +
				"`GET /api/v1/devbox/{name}/files/download` returns a binary stream on success and JSON on error.\n\n" +
				"## Gateway Proxy\n\n" +
				"The runtime gateway proxy path is config-driven and intentionally not modeled as an OpenAPI path here.\n" +
				"Use `GET /api/v1/devbox/{name}` and read `data.gateway.url` plus `data.gateway.token` for app access.",
		},
		"servers": []any{
			map[string]any{
				"url":         "http://localhost:8090",
				"description": "Local development",
			},
			map[string]any{
				"url":         "https://devbox-server.example.com",
				"description": "Example production",
			},
			map[string]any{
				"url":         "{baseUrl}",
				"description": "Custom",
				"variables": map[string]any{
					"baseUrl": map[string]any{
						"default":     "https://devbox-server.example.com",
						"description": "Base URL of the Devbox API server",
					},
				},
			},
		},
		"tags": []any{
			map[string]any{
				"name":        "System",
				"description": "Health and system-level endpoints.",
			},
			map[string]any{
				"name":        "Devbox",
				"description": "Devbox lifecycle, info, and command execution endpoints.",
			},
			map[string]any{
				"name":        "Files",
				"description": "Binary file upload and download endpoints tunneled through the Devbox SDK server.",
			},
		},
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"bearerAuth": map[string]any{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
					"description":  "HS256 JWT. Must include the `namespace` claim.",
				},
			},
			"parameters": map[string]any{
				"DevboxName": map[string]any{
					"name":        "name",
					"in":          "path",
					"required":    true,
					"description": "Devbox name. The server validates it as a Kubernetes DNS1123 subdomain.",
					"schema": map[string]any{
						"type":      "string",
						"minLength": 1,
						"maxLength": 253,
						"pattern":   "^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$",
						"example":   "demo-devbox",
					},
				},
				"UpstreamID": map[string]any{
					"name":        "upstreamID",
					"in":          "query",
					"required":    false,
					"description": "Optional label value used to filter Devboxes by `devbox.sealos.io/upstream-id`.",
					"schema": map[string]any{
						"type":    "string",
						"example": "session-123",
					},
				},
				"FilePath": map[string]any{
					"name":        "path",
					"in":          "query",
					"required":    true,
					"description": "Absolute or relative file path inside the Devbox container.",
					"schema": map[string]any{
						"type":      "string",
						"minLength": 1,
						"example":   "/home/devbox/project/app.txt",
					},
				},
				"TransferTimeoutSeconds": map[string]any{
					"name":        "timeoutSeconds",
					"in":          "query",
					"required":    false,
					"description": "Transfer timeout in seconds. Defaults to 300. Allowed range: 1 to 3600.",
					"schema": map[string]any{
						"type":    "integer",
						"minimum": 1,
						"maximum": 3600,
						"default": 300,
						"example": 300,
					},
				},
				"ContainerName": map[string]any{
					"name":        "container",
					"in":          "query",
					"required":    false,
					"description": "Optional container name. Defaults to the first container in the Devbox pod.",
					"schema": map[string]any{
						"type":    "string",
						"example": "devbox",
					},
				},
				"FileMode": map[string]any{
					"name":        "mode",
					"in":          "query",
					"required":    false,
					"description": "Optional octal file mode applied after upload, for example `644` or `0644`.",
					"schema": map[string]any{
						"type":    "string",
						"pattern": "^[0-7]{3,4}$",
						"example": "0644",
					},
				},
				"DownloadFilename": map[string]any{
					"name":        "filename",
					"in":          "query",
					"required":    false,
					"description": "Optional filename override for the `Content-Disposition` response header.",
					"schema": map[string]any{
						"type":    "string",
						"example": "app.txt",
					},
				},
			},
			"schemas": map[string]any{
				"ErrorResponse": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]any{
						"code": map[string]any{
							"type":    "integer",
							"example": 400,
						},
						"message": map[string]any{
							"type":    "string",
							"example": "invalid request body: unexpected EOF",
						},
					},
					"required": []any{"code", "message"},
				},
				"HealthzData": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]any{
						"status": map[string]any{
							"type":    "string",
							"example": "healthy",
						},
					},
					"required": []any{"status"},
				},
				"CreateDevboxLabel": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]any{
						"key": map[string]any{
							"type":    "string",
							"example": "app.kubernetes.io/component",
						},
						"value": map[string]any{
							"type":    "string",
							"example": "runtime",
						},
					},
					"required": []any{"key", "value"},
				},
				"CreateDevboxKubeAccess": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]any{
						"enabled": map[string]any{
							"type":    "boolean",
							"example": true,
						},
						"roleTemplate": map[string]any{
							"type":    "string",
							"enum":    []any{"view", "edit", "admin"},
							"example": "edit",
						},
					},
				},
				"CreateDevboxRequest": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]any{
						"name": map[string]any{
							"type":      "string",
							"minLength": 1,
							"maxLength": 253,
							"pattern":   "^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$",
							"example":   "demo-devbox",
						},
						"image": map[string]any{
							"type":    "string",
							"example": "registry.example.com/devbox/runtime:custom-v2",
						},
						"labels": map[string]any{
							"type":  "array",
							"items": schemaRef("CreateDevboxLabel"),
						},
						"env": map[string]any{
							"type":                 "object",
							"additionalProperties": map[string]any{"type": "string"},
							"example": map[string]any{
								"FOO":      "bar",
								"NODE_ENV": "production",
							},
						},
						"kubeAccess": schemaRef("CreateDevboxKubeAccess"),
						"upstreamID": map[string]any{
							"type":    "string",
							"example": "session-123",
						},
						"pauseAt": map[string]any{
							"type":    "string",
							"format":  "date-time",
							"example": "2026-03-03T09:00:00Z",
						},
						"archiveAfterPauseTime": map[string]any{
							"type":    "string",
							"example": "24h",
						},
					},
					"required": []any{"name"},
				},
				"CreateDevboxData": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]any{
						"name": map[string]any{
							"type":    "string",
							"example": "demo-devbox",
						},
						"namespace": map[string]any{
							"type":    "string",
							"example": "ns-test",
						},
						"state": map[string]any{
							"type":    "string",
							"example": "Running",
						},
					},
					"required": []any{"name", "namespace", "state"},
				},
				"DevboxStateSummary": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]any{
						"spec": map[string]any{
							"type":    "string",
							"example": "Running",
						},
						"status": map[string]any{
							"type":    "string",
							"example": "Running",
						},
						"phase": map[string]any{
							"type":    "string",
							"example": "Running",
						},
					},
					"required": []any{"spec", "status", "phase"},
				},
				"ListDevboxItem": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]any{
						"name": map[string]any{
							"type":    "string",
							"example": "demo-devbox",
						},
						"creationTimestamp": map[string]any{
							"type":    "string",
							"format":  "date-time",
							"example": "2026-03-02T08:20:30Z",
						},
						"deletionTimestamp": map[string]any{
							"type":    []any{"string", "null"},
							"format":  "date-time",
							"example": nil,
						},
						"state": schemaRef("DevboxStateSummary"),
					},
					"required": []any{"name", "creationTimestamp", "deletionTimestamp", "state"},
				},
				"ListDevboxesData": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]any{
						"items": map[string]any{
							"type":  "array",
							"items": schemaRef("ListDevboxItem"),
						},
					},
					"required": []any{"items"},
				},
				"RefreshPauseAtRequest": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]any{
						"pauseAt": map[string]any{
							"type":    "string",
							"format":  "date-time",
							"example": "2026-03-03T12:00:00Z",
						},
					},
					"required": []any{"pauseAt"},
				},
				"RefreshPauseAtData": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]any{
						"name": map[string]any{
							"type":    "string",
							"example": "demo-devbox",
						},
						"namespace": map[string]any{
							"type":    "string",
							"example": "ns-test",
						},
						"pauseAt": map[string]any{
							"type":    "string",
							"format":  "date-time",
							"example": "2026-03-03T12:00:00Z",
						},
						"refreshedAt": map[string]any{
							"type":    "string",
							"format":  "date-time",
							"example": "2026-03-03T08:00:00Z",
						},
					},
					"required": []any{"name", "namespace", "pauseAt", "refreshedAt"},
				},
				"SSHInfo": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]any{
						"user": map[string]any{
							"type":    "string",
							"example": "devbox",
						},
						"host": map[string]any{
							"type":    "string",
							"example": "staging-usw-1.sealos.io",
						},
						"port": map[string]any{
							"type":    "integer",
							"example": 2233,
						},
						"target": map[string]any{
							"type":    "string",
							"example": "devbox@staging-usw-1.sealos.io -p 2233",
						},
						"link": map[string]any{
							"type":    "string",
							"example": "ssh://devbox@staging-usw-1.sealos.io:2233",
						},
						"command": map[string]any{
							"type":    "string",
							"example": "ssh -i <private-key-file> devbox@staging-usw-1.sealos.io -p 2233",
						},
						"privateKeyEncoding": map[string]any{
							"type":    "string",
							"example": "base64",
						},
						"privateKeyBase64": map[string]any{
							"type":    "string",
							"example": "<base64-private-key>",
						},
					},
					"required": []any{
						"user",
						"host",
						"port",
						"target",
						"link",
						"command",
						"privateKeyEncoding",
						"privateKeyBase64",
					},
				},
				"GatewayInfo": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]any{
						"url": map[string]any{
							"type":    "string",
							"example": "https://devbox-gateway.staging-usw-1.sealos.io/codex/demo-unique-id",
						},
						"token": map[string]any{
							"type":    "string",
							"example": "<signed-gateway-jwt>",
						},
						"port": map[string]any{
							"type":    "integer",
							"example": 1317,
						},
						"uniqueID": map[string]any{
							"type":    "string",
							"example": "demo-unique-id",
						},
					},
					"required": []any{"url", "token", "port"},
				},
				"GetDevboxInfoData": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]any{
						"name": map[string]any{
							"type":    "string",
							"example": "demo-devbox",
						},
						"creationTimestamp": map[string]any{
							"type":    "string",
							"format":  "date-time",
							"example": "2026-02-26T07:20:30Z",
						},
						"deletionTimestamp": map[string]any{
							"type":    []any{"string", "null"},
							"format":  "date-time",
							"example": nil,
						},
						"state": schemaRef("DevboxStateSummary"),
						"ssh":   schemaRef("SSHInfo"),
						"gateway": map[string]any{
							"oneOf": []any{
								schemaRef("GatewayInfo"),
								map[string]any{"type": "null"},
							},
						},
					},
					"required": []any{"name", "creationTimestamp", "deletionTimestamp", "state", "ssh"},
				},
				"StateMutationData": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]any{
						"name": map[string]any{
							"type":    "string",
							"example": "demo-devbox",
						},
						"namespace": map[string]any{
							"type":    "string",
							"example": "ns-test",
						},
						"state": map[string]any{
							"type":    "string",
							"example": "Paused",
						},
					},
					"required": []any{"name", "namespace", "state"},
				},
				"DestroyData": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]any{
						"name": map[string]any{
							"type":    "string",
							"example": "demo-devbox",
						},
						"namespace": map[string]any{
							"type":    "string",
							"example": "ns-test",
						},
						"status": map[string]any{
							"type":    "string",
							"example": "deletion requested",
						},
					},
					"required": []any{"name", "namespace", "status"},
				},
				"ExecRequest": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]any{
						"command": map[string]any{
							"type":     "array",
							"minItems": 1,
							"items": map[string]any{
								"type":      "string",
								"minLength": 1,
							},
							"example": []any{"sh", "-lc", "whoami"},
						},
						"stdin": map[string]any{
							"type":    "string",
							"example": "echo hello from stdin\n",
						},
						"cwd": map[string]any{
							"type":    "string",
							"default": "/home/devbox/workspace",
							"example": "/home/devbox/workspace",
						},
						"timeoutSeconds": map[string]any{
							"type":    "integer",
							"minimum": 1,
							"maximum": 600,
							"default": 30,
							"example": 30,
						},
						"container": map[string]any{
							"type":    "string",
							"example": "devbox",
						},
					},
					"required": []any{"command"},
				},
				"ExecData": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]any{
						"podName": map[string]any{
							"type":    "string",
							"example": "demo-devbox-7d9b5f7f45-s6j8n",
						},
						"namespace": map[string]any{
							"type":    "string",
							"example": "ns-test",
						},
						"container": map[string]any{
							"type":    "string",
							"example": "devbox",
						},
						"command": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "string",
							},
							"example": []any{"sh", "-lc", "whoami"},
						},
						"exitCode": map[string]any{
							"type":        "integer",
							"description": "May be `-1` when the SDK server does not report an exit code.",
							"example":     0,
						},
						"stdout": map[string]any{
							"type":    "string",
							"example": "devbox\n",
						},
						"stderr": map[string]any{
							"type":    "string",
							"example": "",
						},
						"executedAt": map[string]any{
							"type":    "string",
							"format":  "date-time",
							"example": "2026-03-03T10:10:10Z",
						},
					},
					"required": []any{"podName", "namespace", "container", "command", "exitCode", "stdout", "stderr", "executedAt"},
				},
				"UploadData": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]any{
						"name": map[string]any{
							"type":    "string",
							"example": "demo-devbox",
						},
						"namespace": map[string]any{
							"type":    "string",
							"example": "ns-test",
						},
						"podName": map[string]any{
							"type":    "string",
							"example": "demo-devbox-7d9b5f7f45-s6j8n",
						},
						"container": map[string]any{
							"type":    "string",
							"example": "devbox",
						},
						"path": map[string]any{
							"type":    "string",
							"example": "/home/devbox/project/a.txt",
						},
						"sizeBytes": map[string]any{
							"type":    "integer",
							"example": 42,
						},
						"mode": map[string]any{
							"type":    "string",
							"example": "0644",
						},
						"uploadedAt": map[string]any{
							"type":    "string",
							"format":  "date-time",
							"example": "2026-03-03T10:20:30Z",
						},
						"timeoutSecond": map[string]any{
							"type":    "integer",
							"example": 300,
						},
					},
					"required": []any{"name", "namespace", "podName", "container", "path", "sizeBytes", "mode", "uploadedAt", "timeoutSecond"},
				},
			},
		},
		"security": []any{
			map[string]any{
				"bearerAuth": []any{},
			},
		},
		"paths": map[string]any{
			"/healthz": map[string]any{
				"get": map[string]any{
					"tags":        []any{"System"},
					"operationId": "getHealthz",
					"summary":     "Health check",
					"description": "Returns the API server health status. This endpoint is not authenticated.",
					"security":    []any{},
					"responses": map[string]any{
						"200": jsonResponse(
							"Healthy response.",
							successEnvelopeSchema(schemaRef("HealthzData")),
							map[string]any{
								"healthy": example("Healthy server", map[string]any{
									"code":    200,
									"message": "ok",
									"data": map[string]any{
										"status": "healthy",
									},
								}),
							},
							nil,
						),
					},
				},
			},
			"/api/v1/devbox": map[string]any{
				"post": map[string]any{
					"tags":        []any{"Devbox"},
					"operationId": "createDevbox",
					"summary":     "Create a Devbox",
					"description": "Creates a Devbox CR in the namespace carried by the JWT `namespace` claim.",
					"requestBody": jsonRequestBody(
						"Devbox creation payload.",
						schemaRef("CreateDevboxRequest"),
						true,
						map[string]any{
							"basic": example("Create with lifecycle and kube access", map[string]any{
								"name":                  "demo-devbox",
								"image":                 "registry.example.com/devbox/runtime:custom-v2",
								"upstreamID":            "session-123",
								"pauseAt":               "2026-03-03T09:00:00Z",
								"archiveAfterPauseTime": "24h",
								"kubeAccess": map[string]any{
									"enabled":      true,
									"roleTemplate": "edit",
								},
								"env": map[string]any{
									"FOO":      "bar",
									"NODE_ENV": "production",
								},
								"labels": []any{
									map[string]any{
										"key":   "app.kubernetes.io/component",
										"value": "runtime",
									},
								},
							}),
						},
					),
					"responses": map[string]any{
						"201": jsonResponse(
							"Devbox created successfully.",
							successEnvelopeSchema(schemaRef("CreateDevboxData")),
							map[string]any{
								"created": example("Created", map[string]any{
									"code":    201,
									"message": "ok",
									"data": map[string]any{
										"name":      "demo-devbox",
										"namespace": "ns-test",
										"state":     "Running",
									},
								}),
							},
							nil,
						),
						"400": jsonErrorResponse(
							"Invalid request parameters.",
							map[string]any{
								"invalidPauseAt": example("Invalid pauseAt", map[string]any{
									"code":    400,
									"message": "invalid pauseAt: must be RFC3339 format",
								}),
								"invalidRoleTemplate": example("Invalid kube access role", map[string]any{
									"code":    400,
									"message": "invalid kubeAccess.roleTemplate: must be one of view, edit, admin",
								}),
							},
						),
						"401": unauthorizedResponse(),
						"409": jsonErrorResponse(
							"Devbox already exists.",
							map[string]any{
								"conflict": example("Existing Devbox", map[string]any{
									"code":    409,
									"message": "devbox already exists",
								}),
							},
						),
						"500": jsonErrorResponse(
							"Failed to create the Devbox.",
							map[string]any{
								"createFailed": example("Controller failure", map[string]any{
									"code":    500,
									"message": "create devbox failed: some backend error",
								}),
							},
						),
					},
				},
				"get": map[string]any{
					"tags":        []any{"Devbox"},
					"operationId": "listDevboxes",
					"summary":     "List Devboxes",
					"description": "Lists Devboxes in the namespace from the JWT. Optionally filters by `upstreamID`.",
					"parameters": []any{
						paramRef("UpstreamID"),
					},
					"responses": map[string]any{
						"200": jsonResponse(
							"Devbox list.",
							successEnvelopeSchema(schemaRef("ListDevboxesData")),
							map[string]any{
								"items": example("List response", map[string]any{
									"code":    200,
									"message": "ok",
									"data": map[string]any{
										"items": []any{
											map[string]any{
												"name":              "demo-devbox",
												"creationTimestamp": "2026-03-02T08:20:30Z",
												"deletionTimestamp": nil,
												"state": map[string]any{
													"spec":   "Running",
													"status": "Running",
													"phase":  "Running",
												},
											},
										},
									},
								}),
							},
							nil,
						),
						"400": jsonErrorResponse(
							"Invalid query parameters.",
							map[string]any{
								"invalidUpstreamID": example("Invalid upstreamID", map[string]any{
									"code":    400,
									"message": "invalid upstreamID: a valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character",
								}),
							},
						),
						"401": unauthorizedResponse(),
						"500": jsonErrorResponse(
							"Failed to list Devboxes.",
							map[string]any{
								"listFailed": example("List failure", map[string]any{
									"code":    500,
									"message": "list devbox failed: some backend error",
								}),
							},
						),
					},
				},
			},
			"/api/v1/devbox/{name}": map[string]any{
				"get": map[string]any{
					"tags":        []any{"Devbox"},
					"operationId": "getDevboxInfo",
					"summary":     "Get Devbox info",
					"description": "Returns Devbox state, SSH connection info, and gateway access info when the gateway route is configured.",
					"parameters": []any{
						paramRef("DevboxName"),
					},
					"responses": map[string]any{
						"200": jsonResponse(
							"Devbox info.",
							successEnvelopeSchema(schemaRef("GetDevboxInfoData")),
							map[string]any{
								"info": example("Info with gateway", map[string]any{
									"code":    200,
									"message": "ok",
									"data": map[string]any{
										"name":              "demo-devbox",
										"creationTimestamp": "2026-02-26T07:20:30Z",
										"deletionTimestamp": nil,
										"state": map[string]any{
											"spec":   "Running",
											"status": "Running",
											"phase":  "Running",
										},
										"ssh": map[string]any{
											"user":               "devbox",
											"host":               "staging-usw-1.sealos.io",
											"port":               2233,
											"target":             "devbox@staging-usw-1.sealos.io -p 2233",
											"link":               "ssh://devbox@staging-usw-1.sealos.io:2233",
											"command":            "ssh -i <private-key-file> devbox@staging-usw-1.sealos.io -p 2233",
											"privateKeyEncoding": "base64",
											"privateKeyBase64":   "<base64-private-key>",
										},
										"gateway": map[string]any{
											"url":      "https://devbox-gateway.staging-usw-1.sealos.io/codex/demo-unique-id",
											"token":    "<signed-gateway-jwt>",
											"port":     1317,
											"uniqueID": "demo-unique-id",
										},
									},
								}),
							},
							nil,
						),
						"401": unauthorizedResponse(),
						"404": jsonErrorResponse(
							"Devbox not found.",
							map[string]any{
								"notFound": example("Missing Devbox", map[string]any{
									"code":    404,
									"message": "devbox not found",
								}),
							},
						),
						"500": jsonErrorResponse(
							"Failed to assemble Devbox info.",
							map[string]any{
								"privateKeyMissing": example("Secret issue", map[string]any{
									"code":    500,
									"message": "get devbox private key failed: some backend error",
								}),
							},
						),
					},
				},
				"delete": map[string]any{
					"tags":        []any{"Devbox"},
					"operationId": "destroyDevbox",
					"summary":     "Destroy a Devbox",
					"description": "Requests deletion of the Devbox resource.",
					"parameters": []any{
						paramRef("DevboxName"),
					},
					"responses": map[string]any{
						"200": jsonResponse(
							"Deletion requested.",
							successEnvelopeSchema(schemaRef("DestroyData")),
							map[string]any{
								"deleted": example("Delete request accepted", map[string]any{
									"code":    200,
									"message": "ok",
									"data": map[string]any{
										"name":      "demo-devbox",
										"namespace": "ns-test",
										"status":    "deletion requested",
									},
								}),
							},
							nil,
						),
						"401": unauthorizedResponse(),
						"404": jsonErrorResponse(
							"Devbox not found.",
							map[string]any{
								"notFound": example("Missing Devbox", map[string]any{
									"code":    404,
									"message": "devbox not found",
								}),
							},
						),
						"500": jsonErrorResponse(
							"Failed to destroy Devbox.",
							map[string]any{
								"destroyFailed": example("Delete failure", map[string]any{
									"code":    500,
									"message": "destroy devbox failed: some backend error",
								}),
							},
						),
					},
				},
			},
			"/api/v1/devbox/{name}/pause/refresh": map[string]any{
				"post": map[string]any{
					"tags":        []any{"Devbox"},
					"operationId": "refreshDevboxPauseAt",
					"summary":     "Refresh pauseAt",
					"description": "Updates the `pauseAt` lifecycle timestamp on an existing Devbox.",
					"parameters": []any{
						paramRef("DevboxName"),
					},
					"requestBody": jsonRequestBody(
						"New `pauseAt` timestamp.",
						schemaRef("RefreshPauseAtRequest"),
						true,
						map[string]any{
							"basic": example("Refresh pauseAt", map[string]any{
								"pauseAt": "2026-03-03T12:00:00Z",
							}),
						},
					),
					"responses": map[string]any{
						"200": jsonResponse(
							"pauseAt updated.",
							successEnvelopeSchema(schemaRef("RefreshPauseAtData")),
							map[string]any{
								"refreshed": example("Refreshed", map[string]any{
									"code":    200,
									"message": "ok",
									"data": map[string]any{
										"name":        "demo-devbox",
										"namespace":   "ns-test",
										"pauseAt":     "2026-03-03T12:00:00Z",
										"refreshedAt": "2026-03-03T08:00:00Z",
									},
								}),
							},
							nil,
						),
						"400": jsonErrorResponse(
							"Invalid request body.",
							map[string]any{
								"pauseAtRequired": example("Missing pauseAt", map[string]any{
									"code":    400,
									"message": "pauseAt is required",
								}),
							},
						),
						"401": unauthorizedResponse(),
						"404": jsonErrorResponse(
							"Devbox not found.",
							map[string]any{
								"notFound": example("Missing Devbox", map[string]any{
									"code":    404,
									"message": "devbox not found",
								}),
							},
						),
						"500": jsonErrorResponse(
							"Failed to update pauseAt.",
							map[string]any{
								"refreshFailed": example("Update failure", map[string]any{
									"code":    500,
									"message": "refresh devbox pauseAt failed: some backend error",
								}),
							},
						),
					},
				},
			},
			"/api/v1/devbox/{name}/pause": map[string]any{
				"post": map[string]any{
					"tags":        []any{"Devbox"},
					"operationId": "pauseDevbox",
					"summary":     "Pause a Devbox",
					"description": "Sets the Devbox desired state to `Paused`.",
					"parameters": []any{
						paramRef("DevboxName"),
					},
					"responses": map[string]any{
						"200": jsonResponse(
							"Devbox paused.",
							successEnvelopeSchema(schemaRef("StateMutationData")),
							map[string]any{
								"paused": example("Paused", map[string]any{
									"code":    200,
									"message": "ok",
									"data": map[string]any{
										"name":      "demo-devbox",
										"namespace": "ns-test",
										"state":     "Paused",
									},
								}),
							},
							nil,
						),
						"401": unauthorizedResponse(),
						"404": jsonErrorResponse(
							"Devbox not found.",
							map[string]any{
								"notFound": example("Missing Devbox", map[string]any{
									"code":    404,
									"message": "devbox not found",
								}),
							},
						),
						"500": jsonErrorResponse(
							"Failed to pause Devbox.",
							map[string]any{
								"pauseFailed": example("Pause failure", map[string]any{
									"code":    500,
									"message": "pause devbox failed: some backend error",
								}),
							},
						),
					},
				},
			},
			"/api/v1/devbox/{name}/resume": map[string]any{
				"post": map[string]any{
					"tags":        []any{"Devbox"},
					"operationId": "resumeDevbox",
					"summary":     "Resume a Devbox",
					"description": "Sets the Devbox desired state to `Running`.",
					"parameters": []any{
						paramRef("DevboxName"),
					},
					"responses": map[string]any{
						"200": jsonResponse(
							"Devbox resumed.",
							successEnvelopeSchema(schemaRef("StateMutationData")),
							map[string]any{
								"running": example("Running", map[string]any{
									"code":    200,
									"message": "ok",
									"data": map[string]any{
										"name":      "demo-devbox",
										"namespace": "ns-test",
										"state":     "Running",
									},
								}),
							},
							nil,
						),
						"401": unauthorizedResponse(),
						"404": jsonErrorResponse(
							"Devbox not found.",
							map[string]any{
								"notFound": example("Missing Devbox", map[string]any{
									"code":    404,
									"message": "devbox not found",
								}),
							},
						),
						"500": jsonErrorResponse(
							"Failed to resume Devbox.",
							map[string]any{
								"resumeFailed": example("Resume failure", map[string]any{
									"code":    500,
									"message": "resume devbox failed: some backend error",
								}),
							},
						),
					},
				},
			},
			"/api/v1/devbox/{name}/exec": map[string]any{
				"post": map[string]any{
					"tags":        []any{"Devbox"},
					"operationId": "execDevboxCommand",
					"summary":     "Execute a command in the Devbox pod",
					"description": "Runs a synchronous command through the in-pod Devbox SDK server.",
					"parameters": []any{
						paramRef("DevboxName"),
					},
					"requestBody": jsonRequestBody(
						"Command execution payload.",
						schemaRef("ExecRequest"),
						true,
						map[string]any{
							"basic": example("Simple command", map[string]any{
								"command":        []any{"sh", "-lc", "whoami"},
								"timeoutSeconds": 30,
							}),
							"withStdin": example("Command with stdin", map[string]any{
								"command": []any{"python3", "-c", "import sys; print(sys.stdin.read())"},
								"stdin":   "hello from stdin\n",
								"cwd":     "/home/devbox/workspace",
							}),
						},
					),
					"responses": map[string]any{
						"200": jsonResponse(
							"Command executed.",
							successEnvelopeSchema(schemaRef("ExecData")),
							map[string]any{
								"ok": example("Successful exec", map[string]any{
									"code":    200,
									"message": "ok",
									"data": map[string]any{
										"podName":    "demo-devbox-7d9b5f7f45-s6j8n",
										"namespace":  "ns-test",
										"container":  "devbox",
										"command":    []any{"sh", "-lc", "whoami"},
										"exitCode":   0,
										"stdout":     "devbox\n",
										"stderr":     "",
										"executedAt": "2026-03-03T10:10:10Z",
									},
								}),
							},
							nil,
						),
						"400": jsonErrorResponse(
							"Invalid command request.",
							map[string]any{
								"commandRequired": example("Missing command", map[string]any{
									"code":    400,
									"message": "command is required",
								}),
								"timeoutInvalid": example("Invalid timeout", map[string]any{
									"code":    400,
									"message": "timeoutSeconds must be in [1, 600]",
								}),
								"containerInvalid": example("Unknown container", map[string]any{
									"code":    400,
									"message": "container \"sidecar\" not found",
								}),
							},
						),
						"401": unauthorizedResponse(),
						"404": jsonErrorResponse(
							"Devbox pod not found.",
							map[string]any{
								"podNotFound": example("Pod missing", map[string]any{
									"code":    404,
									"message": "devbox pod not found",
								}),
							},
						),
						"409": jsonErrorResponse(
							"Devbox pod is not in a runnable state.",
							map[string]any{
								"podNotRunning": example("Pod not running", map[string]any{
									"code":    409,
									"message": "devbox pod is not running: Pending",
								}),
							},
						),
						"500": jsonErrorResponse(
							"Command execution failed.",
							map[string]any{
								"execFailed": example("SDK server error", map[string]any{
									"code":    500,
									"message": "exec command failed: some upstream error",
								}),
							},
						),
						"504": jsonErrorResponse(
							"Command execution timed out.",
							map[string]any{
								"timeout": example("Exec timeout", map[string]any{
									"code":    504,
									"message": "exec command timeout",
								}),
							},
						),
					},
				},
			},
			"/api/v1/devbox/{name}/files/upload": map[string]any{
				"post": map[string]any{
					"tags":        []any{"Files"},
					"operationId": "uploadDevboxFile",
					"summary":     "Upload a file to the Devbox",
					"description": "Streams raw file bytes to the in-pod Devbox SDK server. Use query parameters for destination path and optional mode.",
					"parameters": []any{
						paramRef("DevboxName"),
						paramRef("FilePath"),
						paramRef("TransferTimeoutSeconds"),
						paramRef("FileMode"),
						paramRef("ContainerName"),
					},
					"requestBody": map[string]any{
						"required": true,
						"content": map[string]any{
							"application/octet-stream": map[string]any{
								"schema": map[string]any{
									"type":   "string",
									"format": "binary",
								},
							},
						},
					},
					"responses": map[string]any{
						"200": jsonResponse(
							"File uploaded.",
							successEnvelopeSchema(schemaRef("UploadData")),
							map[string]any{
								"uploaded": example("Uploaded file", map[string]any{
									"code":    200,
									"message": "ok",
									"data": map[string]any{
										"name":          "demo-devbox",
										"namespace":     "ns-test",
										"podName":       "demo-devbox-7d9b5f7f45-s6j8n",
										"container":     "devbox",
										"path":          "/home/devbox/project/a.txt",
										"sizeBytes":     42,
										"mode":          "0644",
										"uploadedAt":    "2026-03-03T10:20:30Z",
										"timeoutSecond": 300,
									},
								}),
							},
							nil,
						),
						"400": jsonErrorResponse(
							"Invalid upload request.",
							map[string]any{
								"pathRequired": example("Missing path", map[string]any{
									"code":    400,
									"message": "path is required",
								}),
								"modeInvalid": example("Invalid mode", map[string]any{
									"code":    400,
									"message": "invalid mode: must be octal digits like 644 or 0644",
								}),
								"emptyBody": example("Empty request body", map[string]any{
									"code":    400,
									"message": "request body is empty",
								}),
							},
						),
						"401": unauthorizedResponse(),
						"404": jsonErrorResponse(
							"Pod or target file endpoint not found.",
							map[string]any{
								"podNotFound": example("Pod missing", map[string]any{
									"code":    404,
									"message": "devbox pod not found",
								}),
							},
						),
						"409": jsonErrorResponse(
							"Devbox pod is not in a runnable state.",
							map[string]any{
								"podNotRunning": example("Pod not running", map[string]any{
									"code":    409,
									"message": "devbox pod is not running: Pending",
								}),
							},
						),
						"500": jsonErrorResponse(
							"Upload failed.",
							map[string]any{
								"uploadFailed": example("Upload failure", map[string]any{
									"code":    500,
									"message": "upload file failed: some upstream error",
								}),
							},
						),
						"504": jsonErrorResponse(
							"Upload timed out.",
							map[string]any{
								"timeout": example("Upload timeout", map[string]any{
									"code":    504,
									"message": "upload file timeout",
								}),
							},
						),
					},
				},
			},
			"/api/v1/devbox/{name}/files/download": map[string]any{
				"get": map[string]any{
					"tags":        []any{"Files"},
					"operationId": "downloadDevboxFile",
					"summary":     "Download a file from the Devbox",
					"description": "Returns a binary file stream on success. On failure, the endpoint returns the standard JSON error response envelope.",
					"parameters": []any{
						paramRef("DevboxName"),
						paramRef("FilePath"),
						paramRef("TransferTimeoutSeconds"),
						paramRef("ContainerName"),
						paramRef("DownloadFilename"),
					},
					"responses": map[string]any{
						"200": binaryResponse(
							"Binary file stream.",
							"application/octet-stream",
							map[string]any{
								"Content-Disposition": map[string]any{
									"description": "Download filename returned by the API.",
									"schema": map[string]any{
										"type":    "string",
										"example": `attachment; filename="app.txt"`,
									},
								},
								"Content-Length": map[string]any{
									"description": "Byte size when known.",
									"schema": map[string]any{
										"type":    "string",
										"example": "42",
									},
								},
								"X-Devbox-Path": map[string]any{
									"description": "Resolved file path inside the Devbox.",
									"schema": map[string]any{
										"type":    "string",
										"example": "/home/devbox/project/a.txt",
									},
								},
							},
						),
						"400": jsonErrorResponse(
							"Invalid download request.",
							map[string]any{
								"pathRequired": example("Missing path", map[string]any{
									"code":    400,
									"message": "path is required",
								}),
								"timeoutInvalid": example("Invalid timeout", map[string]any{
									"code":    400,
									"message": "timeoutSeconds must be in [1, 3600]",
								}),
							},
						),
						"401": unauthorizedResponse(),
						"404": jsonErrorResponse(
							"Pod or file not found.",
							map[string]any{
								"podNotFound": example("Pod missing", map[string]any{
									"code":    404,
									"message": "devbox pod not found",
								}),
								"fileNotFound": example("File missing", map[string]any{
									"code":    404,
									"message": "file not found",
								}),
							},
						),
						"409": jsonErrorResponse(
							"Devbox pod is not in a runnable state.",
							map[string]any{
								"podNotRunning": example("Pod not running", map[string]any{
									"code":    409,
									"message": "devbox pod is not running: Pending",
								}),
							},
						),
						"500": jsonErrorResponse(
							"Download failed.",
							map[string]any{
								"downloadFailed": example("Download failure", map[string]any{
									"code":    500,
									"message": "download file failed: some upstream error",
								}),
							},
						),
						"504": jsonErrorResponse(
							"Download timed out.",
							map[string]any{
								"timeout": example("Download timeout", map[string]any{
									"code":    504,
									"message": "download file timeout",
								}),
							},
						),
					},
				},
			},
		},
	}
}

func schemaRef(name string) map[string]any {
	return map[string]any{
		"$ref": "#/components/schemas/" + name,
	}
}

func paramRef(name string) map[string]any {
	return map[string]any{
		"$ref": "#/components/parameters/" + name,
	}
}

func successEnvelopeSchema(dataSchema any) map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"code": map[string]any{
				"type":    "integer",
				"example": 200,
			},
			"message": map[string]any{
				"type":    "string",
				"example": "ok",
			},
			"data": dataSchema,
		},
		"required": []any{"code", "message", "data"},
	}
}

func jsonRequestBody(description string, schema any, required bool, examples map[string]any) map[string]any {
	content := map[string]any{
		"application/json": map[string]any{
			"schema": schema,
		},
	}
	if len(examples) > 0 {
		content["application/json"].(map[string]any)["examples"] = examples
	}
	return map[string]any{
		"description": description,
		"required":    required,
		"content":     content,
	}
}

func jsonResponse(description string, schema any, examples map[string]any, headers map[string]any) map[string]any {
	response := map[string]any{
		"description": description,
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": schema,
			},
		},
	}
	if len(examples) > 0 {
		response["content"].(map[string]any)["application/json"].(map[string]any)["examples"] = examples
	}
	if len(headers) > 0 {
		response["headers"] = headers
	}
	return response
}

func jsonErrorResponse(description string, examples map[string]any) map[string]any {
	return jsonResponse(description, schemaRef("ErrorResponse"), examples, nil)
}

func binaryResponse(description string, contentType string, headers map[string]any) map[string]any {
	response := map[string]any{
		"description": description,
		"content": map[string]any{
			contentType: map[string]any{
				"schema": map[string]any{
					"type":   "string",
					"format": "binary",
				},
			},
		},
	}
	if len(headers) > 0 {
		response["headers"] = headers
	}
	return response
}

func unauthorizedResponse() map[string]any {
	return jsonErrorResponse(
		"Missing or invalid bearer token.",
		map[string]any{
			"missingHeader": example("Missing Authorization header", map[string]any{
				"code":    401,
				"message": "missing Authorization header",
			}),
			"invalidToken": example("Invalid JWT", map[string]any{
				"code":    401,
				"message": "invalid token",
			}),
		},
	)
}

func example(summary string, value any) map[string]any {
	return map[string]any{
		"summary": summary,
		"value":   value,
	}
}
