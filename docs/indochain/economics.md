# IndoChain Economics

> Status: Draft / Economics Design Baseline

## 1. Purpose

This document defines the economic design surface of IndoChain without prematurely freezing tokenomics.

## 2. Native Asset

The proposed native asset is **dIDR**.

The name is a design choice inspired by Rupiah denomination. Supply, issuance, allocation, and monetary policy remain open decisions.

## 3. Economic Components

The final model may include:

- transaction fees;
- gas fees;
- validator rewards;
- staking;
- delegation;
- slashing;
- fee burn;
- treasury/community allocation;
- testnet/devnet distributions.

## 4. Production vs Test Networks

Devnet and testnet may use special allocations or faucet funding for development.

These distributions must not be treated as production monetary policy.

## 5. Staking

If PoS is enabled, the economics specification must define:

- minimum stake;
- delegation;
- validator commission;
- reward distribution;
- unbonding;
- redelegation;
- slashing;
- inactive validator behavior.

## 6. Fees

Fee economics must define:

- base fee;
- priority fee;
- fee recipient;
- burn portion;
- minimum fee;
- congestion policy.

## 7. Issuance

If the production network issues new dIDR, the protocol must define:

- initial supply;
- emission schedule;
- maximum supply if applicable;
- validator reward source;
- treasury allocation;
- genesis allocation.

No production values are frozen here.

## 8. PoW Profile

If a PoW network profile is enabled, block rewards and issuance rules must be explicitly tied to that profile.

PoW rewards must not silently alter a PoS/BFT production profile.

## 9. Faucet Economics

Faucet funds are test-network utility only.

The faucet must use network-specific funded accounts and must not be connected to Mainnet monetary issuance.

## 10. Economic Invariants

Potential invariants:

- no unauthorized balance creation;
- deterministic total-supply accounting;
- deterministic fee accounting;
- valid reward distribution;
- slash accounting correctness;
- conservation of balances across transfers.

## 11. Open Decisions

- total supply;
- emission;
- inflation/deflation;
- staking reward;
- burn ratio;
- treasury;
- genesis allocation;
- validator economics;
- delegation economics.
