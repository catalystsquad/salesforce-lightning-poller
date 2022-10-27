## [1.1.5](https://github.com/catalystsquad/salesforce-lightning-poller/compare/v1.1.4...v1.1.5) (2022-10-27)


### Bug Fixes

* automatically detect expired sessions and attempt reauthentication ([#15](https://github.com/catalystsquad/salesforce-lightning-poller/issues/15)) ([a013e1e](https://github.com/catalystsquad/salesforce-lightning-poller/commit/a013e1e164689dde5991cd0f4472e19717fca8d8))

## [1.1.4](https://github.com/catalystsquad/salesforce-lightning-poller/compare/v1.1.3...v1.1.4) (2022-10-13)


### Bug Fixes

* stringToTimeMap func, return empty map if string is empty ([#14](https://github.com/catalystsquad/salesforce-lightning-poller/issues/14)) ([f36df3f](https://github.com/catalystsquad/salesforce-lightning-poller/commit/f36df3fc26d7a5b5e531fd3de4645ebf3c7393bd))

## [1.1.3](https://github.com/catalystsquad/salesforce-lightning-poller/compare/v1.1.2...v1.1.3) (2022-10-13)


### Bug Fixes

* implement a starting position override configuration ([#13](https://github.com/catalystsquad/salesforce-lightning-poller/issues/13)) ([2fb8261](https://github.com/catalystsquad/salesforce-lightning-poller/commit/2fb8261192a53f30bbe9d9a637b2a5e133cc483e))

## [1.1.2](https://github.com/catalystsquad/salesforce-lightning-poller/compare/v1.1.1...v1.1.2) (2022-09-14)


### Bug Fixes

* dedupe record logic ([#11](https://github.com/catalystsquad/salesforce-lightning-poller/issues/11)) ([5893bbe](https://github.com/catalystsquad/salesforce-lightning-poller/commit/5893bbe238679923a3dc8649b8e79ed6f72e34ca))

## [1.1.1](https://github.com/catalystsquad/salesforce-lightning-poller/compare/v1.1.0...v1.1.1) (2022-09-13)


### Bug Fixes

* save previeous record ids in position, remove dupe records ([#10](https://github.com/catalystsquad/salesforce-lightning-poller/issues/10)) ([7f73904](https://github.com/catalystsquad/salesforce-lightning-poller/commit/7f73904ecd2692134ab02b200c69726273833bdb))

# [1.1.0](https://github.com/catalystsquad/salesforce-lightning-poller/compare/v1.0.3...v1.1.0) (2022-09-13)


### Features

* implement usage of NextRecordsUrl in SOQL response ([#8](https://github.com/catalystsquad/salesforce-lightning-poller/issues/8)) ([be0364f](https://github.com/catalystsquad/salesforce-lightning-poller/commit/be0364f24e244a6db4a55446a800c12e993f1806))

## [1.0.3](https://github.com/catalystsquad/salesforce-lightning-poller/compare/v1.0.2...v1.0.3) (2022-08-29)


### Bug Fixes

* date comparison ([#7](https://github.com/catalystsquad/salesforce-lightning-poller/issues/7)) ([b8ec412](https://github.com/catalystsquad/salesforce-lightning-poller/commit/b8ec41297288c4fee5f14d70b16f9ccf36b81016))

## [1.0.2](https://github.com/catalystsquad/repo-name/compare/v1.0.1...v1.0.2) (2022-06-24)


### Bug Fixes

* Refactor to handle queries in parallel and on independent poll statuses ([#6](https://github.com/catalystsquad/repo-name/issues/6)) ([b9aa75f](https://github.com/catalystsquad/repo-name/commit/b9aa75f27fb29b4a212d32f47d60fd00145c6435))

## [1.0.1](https://github.com/catalystsquad/repo-name/compare/v1.0.0...v1.0.1) (2022-06-07)


### Bug Fixes

* Fix position tracking edge case ([#5](https://github.com/catalystsquad/repo-name/issues/5)) ([bf4b671](https://github.com/catalystsquad/repo-name/commit/bf4b671ffc2d36ade768892eb3fba1a2263b165a))

# 1.0.0 (2022-05-11)


### Bug Fixes

* Add persistence and better handling of offsets and dates to ensure we don't miss or duplicate data ([#4](https://github.com/catalystsquad/repo-name/issues/4)) ([43a5aa1](https://github.com/catalystsquad/repo-name/commit/43a5aa1bf236ca8e9e2c708c4724ac163149360f))
* Initial release ([#1](https://github.com/catalystsquad/repo-name/issues/1)) ([865bdd1](https://github.com/catalystsquad/repo-name/commit/865bdd198042d1988bb88c393fc3f3afbac14890))
* Refactor query to a function, instead of a string. ([#3](https://github.com/catalystsquad/repo-name/issues/3)) ([e433e9f](https://github.com/catalystsquad/repo-name/commit/e433e9ff81dfceb7175e6af6e9f8450d04c0eede))
