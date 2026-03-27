# kit
## Purpose
Expose lightweight helpers for numeric parsing, JSON bytes, and panic-safe execution.

## Use When
You need `atoi`/`atoi64`, protobuf serialization helpers, or defensive wrappers like `Try/Exception`.

## Avoid When
A richer utility library already covers these conversions or you require detailed error handling per call.

## Key Entry Points
- `Atoi`, `Atoi32`, `Atoi64`, `UAtoi64`, `Atof`, `Itoa`
- `Try`, `Exception`, `PbData`
- `Type`, `IsNilPointer`, `IsPointer`

## Notes
`Try`/`Exception` swallow panics; wrap only the smallest possible scope.

## Business Usage
- Business usage is narrow: `persist/model_account.go` and several item/mail/login handlers mainly use `Itoa` and small numeric parsers when building Mongo field paths or converting request values.
- In this codebase, `kit.Itoa` is often part of persisted key paths like `Roles.<id>` or `it.<itemId>`. Agents should not casually replace it with ad-hoc formatting because these strings feed `AddUpdateOp`/`AddUnsetOp` style writes.
- `Try` / `Exception` exist, but business code does not treat them as a normal control-flow tool. If a path already assumes valid config/state, prefer the project convention of direct execution over adding extra panic wrapping.
