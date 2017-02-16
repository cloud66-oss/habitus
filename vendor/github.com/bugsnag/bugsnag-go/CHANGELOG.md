## 1.1.1 (2016-12-16)

### Bug fixes

* Replace empty error class property in reports with "error"

## 1.1.0 (2016-11-07)

### Enhancements

* Add middleware for Gin
  [Mike Bull](https://github.com/bullmo)
  [#40](https://github.com/bugsnag/bugsnag-go/pull/40)

* Add middleware for Negroni
  [am-manideep](https://github.com/am-manideep)
  [#28](https://github.com/bugsnag/bugsnag-go/pull/28)

* Support stripping subpackage names
  [Facundo Ferrer](https://github.com/fjferrer)
  [#25](https://github.com/bugsnag/bugsnag-go/pull/25)

* Support using `ErrorWithCallers` to create a stacktrace for errors
  [Conrad Irwin](https://github.com/ConradIrwin)
  [#35](https://github.com/bugsnag/bugsnag-go/pull/35)

## 1.0.5

### Bug fixes

* Avoid swallowing errors which occur upon delivery

1.0.4
-----

- Fix appengine integration broken by 1.0.3

1.0.3
-----

- Allow any Logger with a Printf method.

1.0.2
-----

- Use bugsnag copies of dependencies to avoid potential link rot

1.0.1
-----

- gofmt/golint/govet docs improvements.

1.0.0
-----
