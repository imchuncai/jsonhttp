package jsonhttp

import (
	"fmt"
	"net/http"
)

var _logger Logger

type Logger interface {
	Log(prefix Prefix, v ...interface{})
}

func Must(err error) {
	if err != nil {
		panic(err)
	}
}

// ErrorWithCode is an error with http response status code
type ErrorWithCode struct {
	HTTPResponseStatusCode int
	OriginError            error
}

func (e ErrorWithCode) Error() string {
	return fmt.Sprintf("HTTPResponseStatusCode:%d OriginError:%v", e.HTTPResponseStatusCode, e.OriginError)
}

func MustWithCode(err error, httpResponseStatusCode int) {
	if err != nil {
		panic(ErrorWithCode{httpResponseStatusCode, err})
	}
}

// Forbidden panic a ErrorWithCode error
func Forbidden(err error) {
	panic(ErrorWithCode{http.StatusForbidden, err})
}

func Log(prefix Prefix, v ...interface{}) {
	_logger.Log(prefix, v...)
}

type Prefix uint

const (
	Debug Prefix = iota
	Info
	Warn
	Error
)

var prefixes = []string{"DEBUG ", "INFO ", "WARN ", "ERROR "}

func (p Prefix) String() string {
	if p <= Error {
		return prefixes[p]
	}
	return "UNKNOWN "
}
