# Changelog

## [1.0.2](https://github.com/jryberg/gojinja/compare/v1.0.1...v1.0.2) (2026-05-18)


### Bug Fixes

* **api:** normalize nested maps to OrderedDict at the render boundary ([#11](https://github.com/jryberg/gojinja/issues/11)) ([ceff118](https://github.com/jryberg/gojinja/commit/ceff1186c33c131f2c15d356917d54154f9759cb))


### Documentation

* **readme:** document insertion-order parity in JSONVars ([f6748c9](https://github.com/jryberg/gojinja/commit/f6748c94a150f9053c347e7439ffc9eef086838b))

## [1.0.1](https://github.com/jryberg/gojinja/compare/v1.0.0...v1.0.1) (2026-05-18)


### Bug Fixes

* **api:** preserve dict insertion order end-to-end ([ea8127a](https://github.com/jryberg/gojinja/commit/ea8127af5e3eb9cfb3708b527388f86357b87ed3))

## [1.0.0](https://github.com/jryberg/gojinja/compare/v0.1.0...v1.0.0) (2026-05-11)


### Features

* **api:** expose NormalizeJSONNumbers and JSONVars for Python-aligned JSON ingestion ([270f94e](https://github.com/jryberg/gojinja/commit/270f94e0bcc790a84865d338c9278c85d5ce7bd7))
* **dict:** implement dict.copy() method dispatch ([93cc077](https://github.com/jryberg/gojinja/commit/93cc077061a0f44e2b405f7e0aacf3df8bffd41d))
* **dict:** implement dict.popitem() method dispatch ([cba9487](https://github.com/jryberg/gojinja/commit/cba948703def06cecac493b8fd1559402af469f9))


### Chores

* cut 1.0.0 release ([d1acfc8](https://github.com/jryberg/gojinja/commit/d1acfc81aca88ea1bc0aca59f45e9f7dcba010ed))

## [0.1.0](https://github.com/jryberg/gojinja/compare/v0.0.2...v0.1.0) (2026-05-07)


### Features

* **runtime:** mutating dict methods on in-template dicts ([bac842e](https://github.com/jryberg/gojinja/commit/bac842ec8268c2338bd670bba163f97d021ba74d))
* **runtime:** mutating list methods on in-template lists ([b6254f3](https://github.com/jryberg/gojinja/commit/b6254f3340feeb294cb44bd91c33eef649162521))


### Bug Fixes

* **environment:** name the missing identifier on Undefined-as-callee ([bfaf5c4](https://github.com/jryberg/gojinja/commit/bfaf5c44e8234b4afc9883cb056b10c801bad0fd))
* **parity:** surface Close error when writing fetched tar files ([c16d9b8](https://github.com/jryberg/gojinja/commit/c16d9b8912bb022ac81bd76b7eb5fe63475a3a9f))

## [0.0.2](https://github.com/jryberg/gojinja/compare/v0.0.1...v0.0.2) (2026-05-06)


### Bug Fixes

* **ci:** serialise docs deploys on gh-pages, not on ref ([e1e97be](https://github.com/jryberg/gojinja/commit/e1e97be43bb3fa809ffd6a22fcbad8390e2fc7fb))
* **docs:** wire pymdownx.emoji to material's icon index ([b8d40a1](https://github.com/jryberg/gojinja/commit/b8d40a196415cdd70ff7780f63b3d227e5ddb41e))

## 0.0.1 (2026-05-06)


### Documentation

* added doc site ([40ec2df](https://github.com/jryberg/gojinja/commit/40ec2dfd422c4c8aeb1b0bb5e087cc624149c874))


### Chores

* release 0.0.1 ([cb68fd3](https://github.com/jryberg/gojinja/commit/cb68fd3a3f6c1112185327a42b9d21b464b37501))
