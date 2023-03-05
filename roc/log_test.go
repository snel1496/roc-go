package roc

import (
	"io/ioutil"
	"log"
	"testing"
	"time"
)

const defaultLogLevel LogLevel = LogError

type testWriter struct {
	ch chan string
}

func makeTestWriter() testWriter {
	return testWriter{
		// capacity 1 is needed to ensure that at least one
		// message can be written without blocking
		ch: make(chan string, 1),
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

func (tw testWriter) wait() string {
	const waitTimeout = time.Minute

	select {
	case s := <-tw.ch:
		return s
	case <-time.After(waitTimeout):
		return ""
	}
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

	SetLogLevel(LogDebug)
	defer SetLogLevel(defaultLogLevel)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tw := makeTestWriter()

			log.SetOutput(&tw)
			defer log.SetOutput(ioutil.Discard)

			tt.setupFn()

			ctx, _ := OpenContext(ContextConfig{})
			ctx.Close()

			if tw.wait() == "" {
				t.Fatalf("expected logs, didn't get them before timeout")
			}
		})
	}
}

func TestLog_Func(t *testing.T) {
	SetLogLevel(LogDebug)
	defer SetLogLevel(defaultLogLevel)

	tw := makeTestWriter()
	SetLoggerFunc(func(msg LogMessage) {
		_, _ = tw.Write([]byte(msg.Module + ":" + msg.Text))
	})
	defer SetLoggerFunc(nil)

	ctx, _ := OpenContext(ContextConfig{})
	ctx.Close()

	if tw.wait() == "" {
		t.Fatal("expected logs, didn't get them before timeout")
	}
}

func TestLog_Interface(t *testing.T) {
	SetLogLevel(LogDebug)
	defer SetLogLevel(defaultLogLevel)

	tw := makeTestWriter()
	logger := log.New(&tw, "", log.Lshortfile)

	SetLogger(logger)
	defer SetLogger(nil)

	ctx, _ := OpenContext(ContextConfig{})
	ctx.Close()

	if tw.wait() == "" {
		t.Fatal("expected logs, didn't get them before timeout")
	}
}
