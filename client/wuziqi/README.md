# Client Project Layout

`client/wuziqi` is the Godot client project root.

## Directory conventions

- `assets/`: art, audio, fonts, shaders, and raw source assets
- `docs/`: client-side technical notes and integration docs
- `generated/`: generated content, never mix handwritten scripts here
- `generated/config/json/`: exported configuration json files
- `generated/proto/docpb/`: generated config protocol artifacts for client
- `generated/proto/pb/`: generated runtime protocol artifacts for client
- `proto/`: source `.proto` files mirrored from the repository root
- `scenes/`: scene tree entry points and feature scenes
- `scripts/autoload/`: global singletons
- `scripts/core/`: core infrastructure and shared helpers
- `scripts/data/`: config loading and local data access
- `scripts/features/`: feature-level gameplay and UI logic
- `scripts/net/`: network transport and protocol adapters
- `tests/`: automated test entry points and fixtures

## Recommended workflow

1. Run `tool/client_proto.ps1` or `tool/client_proto.sh` to sync protocol sources into the client project.
2. Run `tool/client_excel.ps1` or `tool/client_excel.sh` to sync generated config data into the client project.
3. Keep generated outputs under `generated/` and handwritten code under `scripts/`.
