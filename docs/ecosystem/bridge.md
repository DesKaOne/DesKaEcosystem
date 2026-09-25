# IndoChain Bridge

> Status: Draft / Ecosystem Design Baseline

## Purpose

A bridge may connect IndoChain assets or messages with external networks.

## Architecture Candidates

~~~text
Source Network
      ↓
Bridge Gateway
      ↓
Verification / Relayer Layer
      ↓
IndoChain Gateway
      ↓
Wrapped / Native Representation
~~~

## Security Boundary

A bridge introduces additional trust assumptions beyond the IndoChain base protocol. The bridge must therefore be treated as an independent security-critical subsystem.

## Candidate Models

Possible approaches include:

- multisignature custody;
- validator/relayer committees;
- light-client verification;
- proof-based verification;
- future zero-knowledge verification.

No bridge model is currently frozen.

## Operational Controls

Production bridge infrastructure should include limits, monitoring, pause/emergency procedures, key rotation, accounting reconciliation, and incident response.

## Deployment Policy

Bridge deployment should follow a stable core protocol, mature wallet/tooling, security review, and explicit governance approval.

## Open Decisions

Supported networks, verification model, custody model, wrapped-asset standard, relayer incentives, limits, and upgrade policy remain open.
