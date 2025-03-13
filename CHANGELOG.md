# Changelog

## [0.4.4](https://github.com/slawo/gollamas/compare/v0.4.3...v0.4.4) (2025-03-13)


### Bug Fixes

* fix broken ollama api compatibility ([ad32a2d](https://github.com/slawo/gollamas/commit/ad32a2df37b009937108ef353afd19e2b5f53744))

## [0.4.3](https://github.com/slawo/gollamas/compare/v0.4.2...v0.4.3) (2025-03-12)


### Bug Fixes

* validate connection urls properly ([6fcc8e4](https://github.com/slawo/gollamas/commit/6fcc8e4e3bd98c4b9ff295f9a4a73848387fb6af))

## [0.4.2](https://github.com/slawo/gollamas/compare/v0.4.1...v0.4.2) (2025-03-11)


### Bug Fixes

* add missing environment variable for --list-aliases ([32a5534](https://github.com/slawo/gollamas/commit/32a55344de4c1ff4b299a73d5e6899b2dcf03939))

## [0.4.1](https://github.com/slawo/gollamas/compare/v0.4.0...v0.4.1) (2025-03-11)


### Bug Fixes

* fixes [#14](https://github.com/slawo/gollamas/issues/14) remove duplicate calls to aggregate endpoints ([6161f1c](https://github.com/slawo/gollamas/commit/6161f1c6e72221157d5d199bb0763a8f9a61aa4c))

## [0.4.0](https://github.com/slawo/gollamas/compare/v0.3.1...v0.4.0) (2025-03-10)


### Features

* returns aliases in the list when listing models ([5aa03c2](https://github.com/slawo/gollamas/commit/5aa03c2ef41f04b29a0e22c47036204e6fdc1d23))


### Bug Fixes

* some models returned by list cannot be reached in other requests ([8f5bd27](https://github.com/slawo/gollamas/commit/8f5bd27d9bd086bbfb56399c8451fd4f87a619da))

## [0.3.1](https://github.com/slawo/gollamas/compare/v0.3.0...v0.3.1) (2025-03-08)


### Bug Fixes

* fixes long delay when a proxied server is unavailable ([81aca2f](https://github.com/slawo/gollamas/commit/81aca2f85e2db47313a67e70ee685c31b74f1e3d))
* resolve an issue where some models are not reachable ([806af79](https://github.com/slawo/gollamas/commit/806af795bdaabc5a06e18312a5fe084a887eb930))

## [0.3.0](https://github.com/slawo/gollamas/compare/v0.2.4...v0.3.0) (2025-03-08)


### Features

* add support for model aliases ([99778b9](https://github.com/slawo/gollamas/commit/99778b9a983e206c8931b00252c10e07b6ffb4e8)), closes [#9](https://github.com/slawo/gollamas/issues/9)
* models not configured on the proxy are filtered out ([f71a9e7](https://github.com/slawo/gollamas/commit/f71a9e71848c2658d909d0090de173bda9a4df69))


### Bug Fixes

* invalid proxy string when proxies is empty ([188d0ae](https://github.com/slawo/gollamas/commit/188d0ae0512caf8f8bb83acb86042fde3c6880f6))

## [0.2.4](https://github.com/slawo/gollamas/compare/v0.2.3...v0.2.4) (2025-03-06)


### Bug Fixes

* change the release name of the packages used for docker ([f3616ae](https://github.com/slawo/gollamas/commit/f3616aef74024d29f36f19a7f3ec7d6bd761ff8a))

## [0.2.3](https://github.com/slawo/gollamas/compare/v0.2.2...v0.2.3) (2025-03-06)


### Bug Fixes

* package naming template ([e25f8fb](https://github.com/slawo/gollamas/commit/e25f8fb171dd2db4af27022227ee12ff92f8f37e))

## [0.2.2](https://github.com/slawo/gollamas/compare/v0.2.1...v0.2.2) (2025-03-06)


### Bug Fixes

* tests failing on list running models ([2a651b4](https://github.com/slawo/gollamas/commit/2a651b415b84e0c5266bc6e2ce93e2a96a96362e))

## [0.2.1](https://github.com/slawo/gollamas/compare/v0.2.0...v0.2.1) (2025-03-05)


### Bug Fixes

* fixes listen/address flag not being applied ([8de41bf](https://github.com/slawo/gollamas/commit/8de41bf8c6f4c7dca82f4f8c64c20d2d4c77bf43))
* resolves a nil pointer crash ([ad95541](https://github.com/slawo/gollamas/commit/ad955414579d0982bad215b3ca50c08ce801e6c3))

## [0.2.0](https://github.com/slawo/gollamas/compare/v0.1.0...v0.2.0) (2025-03-04)


### Features

* add support for environment variables ([ce001a7](https://github.com/slawo/gollamas/commit/ce001a76910c6a0e4022cac7d569eda46f3e1e02))

## [0.1.0](https://github.com/slawo/gollamas/compare/v0.0.1...v0.1.0) (2025-03-04)


### Features

* fixes release process and version command ([29184ae](https://github.com/slawo/gollamas/commit/29184aea11b50eb0de4bbd76a488861ccc6e1b30))
