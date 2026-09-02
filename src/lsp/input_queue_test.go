package lsp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

type lockedBuffer struct {
	mu sync.Mutex
	bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Buffer.Write(p)
}
func (b *lockedBuffer) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.Buffer.String() }

func writeTestMessage(w io.Writer, msg message) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(payload), payload)
	return err
}

func TestServeDiscardsAnalysisSupersededWhileRunning(t *testing.T) {
	reader, writer := io.Pipe()
	var output lockedBuffer
	started := make(chan int, 2)
	release := make(chan struct{}, 2)
	analyzer := func(job analysisJob, _ string) analysisResult {
		started <- job.documents[0].Version
		<-release
		return analysisResult{generation: job.generation, documents: job.documents, results: map[string]*analysis{job.documents[0].URI: {}}}
	}
	done := make(chan error, 1)
	go func() { done <- serveWithAnalyzer(reader, &output, "", false, analyzer) }()

	uri := "file:///main.mg"
	openParams, _ := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": uri, "text": "version one", "version": 1}})
	if err := writeTestMessage(writer, message{Method: "textDocument/didOpen", Params: openParams}); err != nil {
		t.Fatal(err)
	}
	select {
	case version := <-started:
		if version != 1 {
			t.Fatalf("first analysis version = %d", version)
		}
	case <-time.After(time.Second):
		t.Fatal("first analysis did not start")
	}

	changeParams, _ := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": uri, "version": 2}, "contentChanges": []map[string]any{{"text": "version two"}}})
	if err := writeTestMessage(writer, message{Method: "textDocument/didChange", Params: changeParams}); err != nil {
		t.Fatal(err)
	}
	release <- struct{}{}
	select {
	case version := <-started:
		if version != 2 {
			t.Fatalf("replacement analysis version = %d", version)
		}
	case <-time.After(time.Second):
		t.Fatal("replacement analysis did not start")
	}
	if strings.Contains(output.String(), `"version":1`) {
		t.Fatalf("superseded diagnostics were published: %s", output.String())
	}
	release <- struct{}{}

	deadline := time.Now().Add(time.Second)
	for !strings.Contains(output.String(), `"version":2`) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := output.String(); !strings.Contains(got, `"version":2`) {
		t.Fatalf("latest diagnostics were not published: %s", got)
	}
	_ = writer.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not stop")
	}
}
