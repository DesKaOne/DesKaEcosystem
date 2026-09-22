# IndoChain Mining

> Status: Draft / Experimental Mining Module

## 1. Purpose

Mining is documented as an optional PoW module for experimentation, development, benchmarking, and network profiles where PoW is explicitly enabled.

Mining is **not currently frozen as the primary IndoChain production consensus mechanism**.

## 2. Architecture

~~~text
consensus/
├── pos/
├── bft/
└── pow/
~~~

The mining module should implement a consensus-facing interface rather than coupling PoW logic to the rest of the node.

## 3. Candidate PoW Flow

~~~text
Candidate Block
      ↓
Prepare Mining Header
      ↓
Iterate Nonce / Search Space
      ↓
Hash
      ↓
Check Target
      ↓
Valid Solution
      ↓
Broadcast Block
~~~

## 4. Difficulty

A PoW profile requires deterministic difficulty/target rules.

The protocol must define:

- target representation;
- initial difficulty;
- adjustment interval;
- adjustment formula;
- minimum/maximum bounds;
- timestamp rules.

No specific values are frozen here.

## 5. Mining Rewards

If a PoW network profile is enabled, rewards must be defined by the corresponding economics specification.

Reward issuance must not silently apply to a network profile that does not enable PoW.

## 6. Devnet / Testnet Use

Mining can be useful for:

- developer experiments;
- consensus testing;
- block-production simulations;
- educational tooling;
- performance benchmarks.

A local CPU miner may be provided for development networks.

## 7. Security

Mining implementation must prevent:

- invalid target acceptance;
- nonce overflow bugs;
- malformed headers;
- timestamp abuse;
- duplicate block submission;
- resource exhaustion through oversized work requests.

## 8. Open Decisions

- PoW algorithm;
- target format;
- difficulty adjustment;
- block interval;
- reward schedule;
- whether a PoW network profile is maintained long-term.
