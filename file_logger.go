package jsonhttp

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

type fileLogger struct {
	logger         *log.Logger
	fileChangeTime time.Time
	path           string
}

func FileLogger(path string) Logger {
	makeDir(path)
	var now = time.Now()
	var f, err = newLogFile(path, now)
	Must(err)
	return &fileLogger{log.New(f, "", log.LstdFlags|log.Lmsgprefix), nextDayZero(now), path}
}

func (l *fileLogger) Log(prefix Prefix, v ...interface{}) {
	l.changeLogFile()
	l.logger.SetPrefix(prefix.String())
	l.logger.Println(v...)
}

func (l *fileLogger) changeLogFile() {
	var now = time.Now()
	if now.Before(l.fileChangeTime) {
		return
	}
	l.fileChangeTime = nextDayZero(now)
	var f, err = newLogFile(l.path, now)
	if err == nil {
		l.logger.SetOutput(f)
	}
}

func nextDayZero(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local).Add(time.Hour * 24)
}

func makeDir(path string) {
	Must(os.MkdirAll(path, os.ModeDir))
}

func newLogFile(path string, t time.Time) (*os.File, error) {
	var name = filepath.Join(path, t.Format("2006-01-02")+".log")
	return os.OpenFile(name, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
}
