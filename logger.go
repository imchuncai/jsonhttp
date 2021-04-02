package jsonhttp

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"time"
)

// LogError logs error and not panic
func LogError(err error) {
	if err == nil {
		return
	}
	err = getDetailError(err)
	Log(Error, err.Error()+"\n"+string(debug.Stack()))
}

// LogErrorAndPanic logs error and panic
func LogErrorAndPanic(err error) {
	if err == nil {
		return
	}
	err = getDetailError(err)
	Log(Error, err.Error()+"\n"+string(debug.Stack()))
	panic(nil)
}

func getDetailError(err error) error {
	if err == nil {
		return nil
	}
	pc, f, l, ok := runtime.Caller(1)
	if ok {
		var _fun = runtime.FuncForPC(pc)
		var funName = "未知"
		if _fun != nil {
			funName = _fun.Name()
		}
		err = fmt.Errorf("%s\t%s:%d\n%s", funName, f, l, err.Error())
	}
	return err
}

// ErrorWithCode is an error struct with http response status code
type ErrorWithCode struct {
	HTTPResponseStatusCode int
	OriginError            error
}

func (e ErrorWithCode) Error() string {
	return fmt.Sprintf("HTTPResponseStatusCode:%d OriginError:%v", e.HTTPResponseStatusCode, e.OriginError)
}

// CheckError will panic if err is not nil
func CheckError(err error) {
	if err != nil {
		panic(err)
	}
}

// CheckErrorWithCode will panic if err is not nil
func CheckErrorWithCode(err error, httpResponseStatusCode int) {
	if err != nil {
		panic(ErrorWithCode{httpResponseStatusCode, err})
	}
}

// Forbidden panic a ErrorWithCode error
func Forbidden(err error) {
	panic(ErrorWithCode{http.StatusForbidden, err})
}

// Level log level
type Level int

// all log levels
const (
	Debug Level = iota
	Info
	Important
	Error
	Panic
)

var levels = []string{"debug", "info", "important", "error", "panic"}

func init() {
	makeDirs()
}

func makeDirs() {
	var logsFilePath = filepath.Join("logs")
	CheckError(os.MkdirAll(logsFilePath, os.ModeDir))
}

// Log logs message of target level
func Log(level Level, details string) {
	if level < Debug || level > Panic {
		return
	}
	logFilePath := filepath.Join("logs", time.Now().Format("2006-01-02")+".log")
	if _, err := os.Stat(logFilePath); err != nil && os.IsNotExist(err) {
		logFile, err := os.Create(logFilePath)
		if err != nil {
			fmt.Println(err.Error())
		}
		defer logFile.Close()
	}
	logFile, err := os.OpenFile(logFilePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	defer logFile.Close()
	var infoLog = log.New(logFile, levels[level], log.LstdFlags)
	infoLog.Println(details)
}
