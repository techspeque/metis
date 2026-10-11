# phase-1-ws-1.2 — Sign the payload

## Goal

Sign every webhook payload with HMAC-SHA256.

## Review

- The signature covers the raw body, not the parsed JSON
- Key rotation is out of scope (ADR-0004)
