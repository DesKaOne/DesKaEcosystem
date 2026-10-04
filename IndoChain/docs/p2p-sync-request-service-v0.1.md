# P2P Sync Request Service v0.1

## Purpose

Provide a small orchestration boundary around SyncMessageHandler for future network integration.

## Responsibilities

- Hold the configured request handler.
- Forward block-request messages.
- Preserve handler validation and service errors.
- Compose a successful BlockResponse into a development response Message when requested.

## Boundary

The service does not perform network I/O or define canonical wire serialization.

The underlying SyncMessageHandler remains responsible for message validation, request decoding, and bounded block-range serving.

## Response-message composition

HandleMessage(msg, encoder, maxPayload) executes the request service and delegates response encoding to the injected SyncResponseEncoder.

The resulting payload is wrapped as MessageTypeBlockResponse and checked against the configured payload limit through the existing response-message boundary.

The encoder remains injected because canonical block-response serialization is not frozen in v0.1.

## Error behavior

A nil response encoder is rejected before request execution. Request-service errors and encoder errors are preserved.

## v0.1 limitation

This remains an orchestration boundary only. Transport, peer management, authentication, and canonical P2P serialization remain separate concerns.

The block importer/authentication boundary is not bypassed by this service.
