# [1.9.0](https://github.com/martynvdijke/redchef/compare/v1.8.0...v1.9.0) (2026-07-20)


### Bug Fixes

* restore unlock flow, add mock iDEAL + invoice emails + share route + admin fixes ([fab596c](https://github.com/martynvdijke/redchef/commit/fab596c7171860f8115c0f5ecbde0f0088ae1890))


### Features

* email notification to all users when new post is uploaded ([ffab43d](https://github.com/martynvdijke/redchef/commit/ffab43daa2118c7635cdaebc9b807ede813517ae))

# [1.8.0](https://github.com/martynvdijke/redchef/compare/v1.7.0...v1.8.0) (2026-07-20)


### Features

* Phase 2 social features + auth fix + email settings ([f86fe12](https://github.com/martynvdijke/redchef/commit/f86fe12b7f482b04835b65e4127f868abc052de2))

# [1.7.0](https://github.com/martynvdijke/redchef/compare/v1.6.1...v1.7.0) (2026-07-20)


### Features

* Phase 1 — auth, paywall, media pipeline, admin users, feed filters ([ff75578](https://github.com/martynvdijke/redchef/commit/ff75578b163ccbaf969a0bd4a3a42e481ca997a9))

## [1.6.1](https://github.com/martynvdijke/redchef/compare/v1.6.0...v1.6.1) (2026-07-20)


### Bug Fixes

* restore updateMemberUI function that was accidentally removed ([269b424](https://github.com/martynvdijke/redchef/commit/269b4242f81ff0636b2de0592ad3070128df24a8))

# [1.6.0](https://github.com/martynvdijke/redchef/compare/v1.5.1...v1.6.0) (2026-07-20)


### Bug Fixes

* align default uploadDir with main.go (/app/media) ([bd5cb5c](https://github.com/martynvdijke/redchef/commit/bd5cb5ce0802147f792892794759ae872f245c98))


### Features

* iDeal payment flow with per-item and subscription pricing ([1600caf](https://github.com/martynvdijke/redchef/commit/1600caf28bc32d2709f26c85a174c4cc4191f36f))

## [1.5.1](https://github.com/martynvdijke/redchef/compare/v1.5.0...v1.5.1) (2026-07-20)


### Bug Fixes

* auto-append /script.js to Umami script URL if missing ([e536e18](https://github.com/martynvdijke/redchef/commit/e536e18def6f6b89253994c7a228624e2f8bf873))

# [1.5.0](https://github.com/martynvdijke/redchef/compare/v1.4.0...v1.5.0) (2026-07-20)


### Features

* upload progress bar and lock toggle UI ([c9d181e](https://github.com/martynvdijke/redchef/commit/c9d181eaba2b775911c0b607b48a9fa9a07265ba))

# [1.4.0](https://github.com/martynvdijke/redchef/compare/v1.3.1...v1.4.0) (2026-07-20)


### Features

* lock toggle, upload progress bar, admin nav link, fix upload path default ([580431d](https://github.com/martynvdijke/redchef/commit/580431d38cab6dc6b150b55ffbd0cad11ac0f805))

## [1.3.1](https://github.com/martynvdijke/redchef/compare/v1.3.0...v1.3.1) (2026-07-20)


### Bug Fixes

* upload and date parsing bugs, add comprehensive tests ([eaf6cfc](https://github.com/martynvdijke/redchef/commit/eaf6cfcb8cac65a82bed001d2fc15cc6427d529b))

# [1.3.0](https://github.com/martynvdijke/redchef/compare/v1.2.0...v1.3.0) (2026-07-20)


### Features

* first-run admin setup flow with web UI ([66de4ad](https://github.com/martynvdijke/redchef/commit/66de4ad3cb58962510d80eb001b555101978f807))

# [1.2.0](https://github.com/martynvdijke/redchef/compare/v1.1.3...v1.2.0) (2026-07-20)


### Features

* royal member subscription gate with fan-site UI ([e12e044](https://github.com/martynvdijke/redchef/commit/e12e04489b636e47b44f5d29a7b2ad9e762d561b))

## [1.1.3](https://github.com/martynvdijke/redchef/compare/v1.1.2...v1.1.3) (2026-07-20)


### Bug Fixes

* switch to alpine base image for healthcheck support and embed persistent paths in code ([d226d87](https://github.com/martynvdijke/redchef/commit/d226d87bac0034de72170b4bee93d8a565666f44))

## [1.1.2](https://github.com/martynvdijke/redchef/compare/v1.1.1...v1.1.2) (2026-07-20)


### Bug Fixes

* set persistent paths /db for database and /app/media for uploads ([79cd99f](https://github.com/martynvdijke/redchef/commit/79cd99ff1ea6719899d261a520c0de75e50b0759))

## [1.1.1](https://github.com/martynvdijke/redchef/compare/v1.1.0...v1.1.1) (2026-07-20)


### Bug Fixes

* change docker container port from 8080 to 6270 ([51599e9](https://github.com/martynvdijke/redchef/commit/51599e9ecd757a0b83af04c11c65580188bce3f4))

# [1.1.0](https://github.com/martynvdijke/redchef/compare/v1.0.0...v1.1.0) (2026-07-20)


### Features

* add admin-configurable Umami analytics settings ([9b3180c](https://github.com/martynvdijke/redchef/commit/9b3180cfa2fc626474ef5ddeb89f03acf662b551))

# 1.0.0 (2026-07-19)


### Bug Fixes

* bump Go version to 1.23 in CI/release workflows and Dockerfile ([5b729f7](https://github.com/martynvdijke/redchef/commit/5b729f7cb193706f47ea098a804b7ee06bd98531))


### Features

* initial RedChef implementation ([6738a2e](https://github.com/martynvdijke/redchef/commit/6738a2e5def29c22630f6b1905d084a818b3982c))
