package daemon_test

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mattdurham/grv/daemon"
)

func startTestDaemon(t *testing.T) (sockPath string) {
	t.Helper()
	dir := t.TempDir()
	sockPath = filepath.Join(dir, "test.sock")
	pidPath := filepath.Join(dir, "test.pid")
	s := daemon.NewServer(dir, sockPath, pidPath, "")
	go s.Run()
	// Wait for socket to appear
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(sockPath); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("daemon did not start within 2 seconds")
	return
}

func sendRequest(t *testing.T, sockPath, tool string, args json.RawMessage) daemon.Response {
	t.Helper()
	conn, err := net.DialTimeout("unix", sockPath, 5*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	req := daemon.Request{Tool: tool, Args: args}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		t.Fatalf("encode request: %v", err)
	}
	var resp daemon.Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func TestDaemonHappyPath(t *testing.T) {
	sockPath := startTestDaemon(t)

	absFile, err := filepath.Abs("../testdata/simple.go")
	if err != nil {
		t.Fatal(err)
	}
	args, _ := json.Marshal(map[string]string{"file": absFile})
	resp := sendRequest(t, sockPath, "ast_list", args)

	if resp.Error != nil {
		t.Fatalf("expected no error, got: %s", *resp.Error)
	}
	if len(resp.Result) == 0 {
		t.Fatal("expected non-empty result")
	}

	var items []interface{}
	if err := json.Unmarshal(resp.Result, &items); err != nil {
		t.Fatalf("result is not a JSON array: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected non-empty AST list")
	}
}

func TestDaemonKeepAlive(t *testing.T) {
	sockPath := startTestDaemon(t)

	absFile, err := filepath.Abs("../testdata/simple.go")
	if err != nil {
		t.Fatal(err)
	}
	args, _ := json.Marshal(map[string]string{"file": absFile})

	// Send two requests on the same connection
	conn, err := net.DialTimeout("unix", sockPath, 5*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)

	for i := 0; i < 2; i++ {
		req := daemon.Request{Tool: "ast_list", Args: args}
		if err := enc.Encode(req); err != nil {
			t.Fatalf("encode request %d: %v", i, err)
		}
		var resp daemon.Response
		if err := dec.Decode(&resp); err != nil {
			t.Fatalf("decode response %d: %v", i, err)
		}
		if resp.Error != nil {
			t.Errorf("request %d: expected no error, got: %s", i, *resp.Error)
		}
	}
}

func TestDaemonUnknownTool(t *testing.T) {
	sockPath := startTestDaemon(t)

	resp := sendRequest(t, sockPath, "nonexistent_tool", json.RawMessage(`{}`))
	if resp.Error == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestDaemonConcurrent(t *testing.T) {
	sockPath := startTestDaemon(t)

	absFile, err := filepath.Abs("../testdata/simple.go")
	if err != nil {
		t.Fatal(err)
	}
	args, _ := json.Marshal(map[string]string{"file": absFile})

	var wg sync.WaitGroup
	errs := make(chan error, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp := sendRequest(t, sockPath, "ast_list", args)
			if resp.Error != nil {
				errs <- nil
			}
			var items []interface{}
			if err := json.Unmarshal(resp.Result, &items); err != nil {
				errs <- err
			}
		}()
	}

	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Errorf("concurrent request error: %v", e)
		}
	}
}
