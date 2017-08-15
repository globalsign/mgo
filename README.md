[![Build Status](https://travis-ci.org/globalsign/mgo.svg?branch=master)](https://travis-ci.org/globalsign/mgo) [![GoDoc](https://godoc.org/github.com/globalsign/mgo?status.svg)](https://godoc.org/github.com/globalsign/mgo)

The MongoDB driver for Go
-------------------------

This fork has had a few improvements by ourselves as well as several PR's merged from the original mgo repo that are currently awaiting review. Changes are mostly geared towards performance improvements and bug fixes, though a few new features have been added.

Further PR's (with tests) are welcome, but please maintain backwards compatibility.

## Changes
* Fixes attempting to authenticate before every query ([details](https://github.com/go-mgo/mgo/issues/254))
* Removes bulk update / delete batch size limitations ([details](https://github.com/go-mgo/mgo/issues/288))
* Adds native support for `time.Duration` marshalling ([details](https://github.com/go-mgo/mgo/pull/373))
* Reduce memory footprint / garbage collection pressure by reusing buffers ([details](https://github.com/go-mgo/mgo/pull/229))
* Support majority read concerns ([details](https://github.com/globalsign/mgo/pull/2))
* Improved connection handling ([details](https://github.com/globalsign/mgo/pull/5))
* Hides SASL warnings ([details](https://github.com/globalsign/mgo/pull/7))
* Support for partial indexes ([detials](https://github.com/domodwyer/mgo/commit/5efe8eccb028238d93c222828cae4806aeae9f51))
* Fixes timezone handling ([details](https://github.com/go-mgo/mgo/pull/464)) 
* Integration tests run against newest MongoDB 3.2 releases ([details](https://github.com/globalsign/mgo/pull/4), [more](https://github.com/globalsign/mgo/pull/24))
* Improved multi-document transaction performance ([details](https://github.com/globalsign/mgo/pull/10), [more](https://github.com/globalsign/mgo/pull/11), [more](https://github.com/globalsign/mgo/pull/16))
* Fixes cursor timeouts ([detials](https://jira.mongodb.org/browse/SERVER-24899))
* Support index hints and timeouts for count queries ([details](https://github.com/globalsign/mgo/pull/17))

---

### Thanks to
* @BenLubar
* @carter2000
* @cezarsa
* @drichelson
* @eaglerayp
* @fmpwizard
* @jameinel
* @Reenjii
* @smoya
* @wgallagher