# Structure Notes

This client follows a separation between source assets, handwritten logic, and generated artifacts.

- Scene assets stay under `scenes/`.
- Shared singleton services stay under `scripts/autoload/`.
- Cross-feature utilities stay under `scripts/core/`.
- Feature scripts should depend on `core`, `data`, and `net`, but avoid coupling directly to other features.
- Generated files belong under `generated/` so cleanup and regeneration are safe.
