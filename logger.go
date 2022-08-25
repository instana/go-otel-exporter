package instana

import (
	"log"
	"os"
	"strings"
)

const (
	ERROR = iota + 1
	WARN
	INFO
	DEBUG
)

var logLevels = map[string]int{
	"error": ERROR,
	"warn":  WARN,
	"info":  INFO,
	"debug": DEBUG,
}

type Logger struct {
	level int
}

func newLogger() *Logger {
	l := &Logger{}

	if level, ok := os.LookupEnv("INSTANA_LOG_LEVEL"); ok {
		l.level = logLevels[strings.ToLower(level)]
	}

	if l.level == 0 {
		l.level = ERROR
	}

	return l
}

func (l Logger) prepend(prefix string, args []interface{}) []interface{} {
	return append([]interface{}{prefix}, args...)
}

func (l Logger) info(args ...interface{}) {
	if l.level >= INFO {
		log.Println(l.prepend("[INFO]:", args)...)
	}
}

func (l Logger) warn(args ...interface{}) {
	if l.level >= WARN {
		log.Println(l.prepend("[WARN]:", args)...)
	}
}

func (l Logger) debug(args ...interface{}) {
	if l.level >= DEBUG {
		log.Println(l.prepend("[DEBUG]:", args)...)
	}
}

func (l Logger) error(args ...interface{}) {
	if l.level >= ERROR {
		log.Println(l.prepend("[ERROR]:", args)...)
	}
}
