# P2P Sync Message v0.1

## Purpose

This document defines the development-only helper that connects a planned BlockRequest to the existing P2P Message envelope.

The helper is transport-agnostic and does not define canonical protocol serialization.

## Request flow

1. Validate the BlockRequest against the configured maximum range.
2. Encode the request with the development sync codec.
3. Wrap the encoded payload as MessageTypeBlockRequest.
4. Enforce the configured maximum message payload size.

The resulting message can be passed to the existing message encoder or transport layer.

## Boundary

BuildBlockRequestMessage composes:

- BlockRequest
- EncodeBlockRequest
- Message

It does not:

- open a network connection;
- select peers;
- execute or import blocks;
- define canonical wire serialization;
- bypass transaction authentication during block import.

## Validation

Invalid requests are rejected before a message is produced. Payloads larger than the configured message limit are rejected as invalid messages.

## v0.1 status

This is a development integration boundary only. The exact canonical P2P message schema remains subject to the protocol serialization freeze.
