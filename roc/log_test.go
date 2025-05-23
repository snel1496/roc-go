package roc

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const defaultLogLevel LogLevel = LogError

type testWriter struct {
	ch chan string
}

func makeTestWriter() testWriter {
	return testWriter{
		ch: make(chan string, 1000),
	}
}

func (tw testWriter) Write(buf []byte) (int, error) {
	select {
	case tw.ch <- string(buf):
	default:
		// drop message instead of blocking if the channel is full
	}
	return len(buf), nil
}

func (tw testWriter) waitMatching(predicate func(msg string) bool) string {
	const waitTimeout = time.Minute

	for {
		select {
		case s := <-tw.ch:
			if predicate(s) {
				return s
			}
		case <-time.After(waitTimeout):
			return ""
		}
	}
}

func (tw testWriter) waitAny() string {
	return tw.waitMatching(func(msg string) bool {
		return true
	})
}

func TestLog_Default(t *testing.T) {
	tests := []struct {
		name    string
		setupFn func()
	}{
		{name: "default", setupFn: func() {}},
		{name: "set_logger_func", setupFn: func() { SetLoggerFunc(nil) }},
		{name: "set_logger", setupFn: func() { SetLogger(nil) }},
	}

	logDrain()

	SetLogLevel(LogDebug)
	defer SetLogLevel(defaultLogLevel)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tw := makeTestWriter()

			log.SetOutput(&tw)
			defer log.SetOutput(ioutil.Discard)

			tt.setupFn()

			ctx, err := OpenContext(ContextConfig{})
			require.NoError(t, err)
			ctx.Close()

			if tw.waitAny() == "" {
				t.Fatalf("expected logs, didn't get them before timeout")
			}
		})
	}
}

func TestLog_Interface(t *testing.T) {
	logDrain()

	SetLogLevel(LogDebug)
	defer SetLogLevel(defaultLogLevel)

	tw := makeTestWriter()
	logger := log.New(&tw, "", log.Lshortfile)

	SetLogger(logger)
	defer SetLogger(nil)

	ctx, err := OpenContext(ContextConfig{})
	require.NoError(t, err)
	ctx.Close()

	if tw.waitAny() == "" {
		t.Fatal("expected logs, didn't get them before timeout")
	}
}

func TestLog_Func(t *testing.T) {
	logDrain()

	SetLogLevel(LogTrace)
	defer SetLogLevel(defaultLogLevel)

	ch := make(chan LogMessage, 1)

	SetLoggerFunc(func(msg LogMessage) {
		if msg.Level == LogTrace {
			select {
			case ch <- msg:
			default:
			}
		}
	})
	defer SetLoggerFunc(nil)

	ctx, err := OpenContext(ContextConfig{})
	require.NoError(t, err)
	ctx.Close()

	select {
	case msg := <-ch:
		assert.Equal(t, LogTrace, msg.Level, "Expected log level to be trace")
		assert.NotEmpty(t, msg.Module)
		assert.NotEmpty(t, msg.File)
		assert.NotEmpty(t, msg.Line)
		assert.False(t, msg.Time.IsZero())
		assert.NotEmpty(t, msg.Pid)
		assert.NotEmpty(t, msg.Tid)
		assert.NotEmpty(t, msg.Text)
	case <-time.After(time.Minute):
		t.Fatal("expected logs, didn't get them before timeout")
	}
}

func TestLog_Levels(t *testing.T) {
	logDrain()

	tests := []struct {
		level LogLevel
		str   string
	}{
		{LogError, "[err]"},
		{LogInfo, "[inf]"},
		{LogDebug, "[dbg]"},
		{LogTrace, "[trc]"},
	}

	for _, tt := range tests {
		t.Run(tt.str, func(t *testing.T) {
			tw := makeTestWriter()
			logger := log.New(&tw, "", log.Lshortfile)

			SetLogger(logger)
			defer SetLogger(nil)

			SetLogLevel(tt.level)
			defer SetLogLevel(defaultLogLevel)

			ctx, err := OpenContext(ContextConfig{})
			require.NoError(t, err)

			_, err = OpenReceiver(ctx, ReceiverConfig{})
			require.Error(t, err)

			ctx.Close()

			msg := tw.waitMatching(func(msg string) bool {
				return strings.Contains(msg, tt.str)
			})

			if msg == "" {
				t.Fatal("expected logs, didn't get them before timeout")
			}
		})
	}
}

func TestLog_Write(t *testing.T) {
	logDrain()

	SetLogLevel(LogDebug)
	defer SetLogLevel(defaultLogLevel)

	ch := make(chan LogMessage, 10)

	SetLoggerFunc(func(msg LogMessage) {
		if msg.Module == "roc_go" && msg.File == "roc/context.go" {
			select {
			case ch <- msg:
			default:
			}
		}
	})
	defer SetLoggerFunc(nil)

	testStartTime := time.Now()
	ctx, err := OpenContext(ContextConfig{})
	require.NoError(t, err)
	defer ctx.Close()

	var entered, leaved bool

	for !(entered && leaved) {
		select {
		case msg := <-ch:
			assert.Equal(t, LogDebug, msg.Level, "Expected log level to be debug")
			assert.Equal(t, "roc_go", msg.Module)
			assert.Equal(t, "roc/context.go", msg.File)
			assert.Greater(t, msg.Line, 0, "Line number must be positive")
			assert.Equal(t, uint64(os.Getpid()), msg.Pid)
			assert.NotEmpty(t, msg.Tid)
			assert.WithinRange(t, msg.Time, testStartTime.Add(-time.Millisecond),
				time.Now().Add(time.Millisecond), "Time must have meaningful value")
			if strings.Contains(msg.Text, "entering OpenContext()") {
				entered = true
			}
			if entered && strings.Contains(msg.Text, "leaving OpenContext()") {
				leaved = true
				assert.Contains(t, msg.Text, fmt.Sprintf("context=%p", ctx))
			}
		case <-time.After(time.Minute):
			t.Fatal("expected logs, didn't get them before timeout")
		}
	}
}
