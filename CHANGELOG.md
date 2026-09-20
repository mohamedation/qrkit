# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and this project
adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Fixed
- `QRCode.Save` writes to a temporary file and replaces the destination only
  on success, so a failed write no longer leaves a partial file.

## [0.1.0] - 2026-09-18

### Added
- QR Code Model 2 encoder: versions 1-40, error-correction levels L/M/Q/H,
  numeric / alphanumeric / byte modes (auto-selected), automatic mask selection.
- PNG (`image.Image`) and SVG output.
- Styling: six module shapes, module scale (gaps), four finder shapes with
  independent colours, custom foreground/background colours, transparent
  backgrounds, configurable quiet zone and exact output size.
- Centre logo support with clip shapes, optional plate, per-block
  error-correction budget check and automatic version bump.
- `qrkit` command-line tool.

[Unreleased]: https://github.com/mohamedation/qrkit/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/mohamedation/qrkit/releases/tag/v0.1.0
