package serve

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	srv, err := New(Config{
		DataDir:         dir,
		MaxManifestSize: 10 << 20, // 10 MB
		Version:         "test",
	})
	if err != nil {
		t.Fatalf("creating test server: %v", err)
	}
	return srv
}

func setupPassword(t *testing.T, srv *Server, password string) {
	t.Helper()
	body := `{"password":"` + password + `"}`
	req := httptest.NewRequest("POST", "/api/setup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("setup failed: %d %s", w.Code, w.Body.String())
	}
}

func uploadManifest(t *testing.T, srv *Server, manifest []byte, meta map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	fw, err := mw.CreateFormFile("manifest", "MANIFEST.age")
	if err != nil {
		t.Fatal(err)
	}
	fw.Write(manifest)

	if meta != nil {
		metaJSON, _ := json.Marshal(meta)
		mw.WriteField("meta", string(metaJSON))
	}
	mw.Close()

	req := httptest.NewRequest("POST", "/api/bundle", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

func TestSetupFlow(t *testing.T) {
	srv := newTestServer(t)

	// Before setup, status should show no password
	req := httptest.NewRequest("GET", "/api/status", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var status map[string]any
	json.NewDecoder(w.Body).Decode(&status)
	if status["hasPassword"] != false {
		t.Error("expected hasPassword=false before setup")
	}

	// Setup password
	setupPassword(t, srv, "testpassword123")

	// Status should now show password set
	req = httptest.NewRequest("GET", "/api/status", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	json.NewDecoder(w.Body).Decode(&status)
	if status["hasPassword"] != true {
		t.Error("expected hasPassword=true after setup")
	}

	// Second setup should return 409
	body := `{"password":"another"}`
	req = httptest.NewRequest("POST", "/api/setup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Errorf("expected 409 for second setup, got %d", w.Code)
	}
}

func TestBundleLifecycle(t *testing.T) {
	srv := newTestServer(t)
	setupPassword(t, srv, "adminpass")

	// Upload a manifest
	manifest := []byte("fake-encrypted-manifest-data")
	meta := map[string]any{
		"name":      "test-recovery",
		"threshold": 2,
		"total":     3,
	}
	w := uploadManifest(t, srv, manifest, meta)
	if w.Code != http.StatusOK {
		t.Fatalf("upload failed: %d %s", w.Code, w.Body.String())
	}

	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	if result["id"] == nil || result["id"] == "" {
		t.Error("expected non-empty bundle ID")
	}

	// Status should show manifest exists
	req := httptest.NewRequest("GET", "/api/status", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var status map[string]any
	json.NewDecoder(w.Body).Decode(&status)
	if status["hasManifest"] != true {
		t.Error("expected hasManifest=true after upload")
	}

	// Download manifest
	req = httptest.NewRequest("GET", "/api/bundle/manifest", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for manifest download, got %d", w.Code)
	}
	if !bytes.Equal(w.Body.Bytes(), manifest) {
		t.Error("downloaded manifest doesn't match uploaded")
	}

	// Delete with wrong password
	body := `{"password":"wrong"}`
	req = httptest.NewRequest("DELETE", "/api/bundle", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for wrong password, got %d", w.Code)
	}

	// Delete with correct password
	body = `{"password":"adminpass"}`
	req = httptest.NewRequest("DELETE", "/api/bundle", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for delete, got %d %s", w.Code, w.Body.String())
	}

	// Status should show no manifest
	req = httptest.NewRequest("GET", "/api/status", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	json.NewDecoder(w.Body).Decode(&status)
	if status["hasManifest"] != false {
		t.Error("expected hasManifest=false after delete")
	}
}

func TestSetupRequired(t *testing.T) {
	srv := newTestServer(t)

	// Upload should fail without setup
	manifest := []byte("test")
	w := uploadManifest(t, srv, manifest, nil)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 without setup, got %d", w.Code)
	}
}

func TestLargeManifest(t *testing.T) {
	dir := t.TempDir()
	srv, err := New(Config{
		DataDir:         dir,
		MaxManifestSize: 1 << 10, // 1 KB for testing
		Version:         "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	setupPassword(t, srv, "pass12345678")

	// Upload manifest larger than limit
	bigManifest := make([]byte, 2<<10) // 2 KB
	w := uploadManifest(t, srv, bigManifest, nil)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413 for large manifest, got %d", w.Code)
	}

	// Upload manifest within limit
	smallManifest := make([]byte, 512)
	w = uploadManifest(t, srv, smallManifest, nil)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for small manifest, got %d %s", w.Code, w.Body.String())
	}
}

func TestPathTraversalPrevention(t *testing.T) {
	s := &Store{dir: t.TempDir()}
	// Attempt to delete with path traversal ID
	err := s.Delete("../../etc/passwd")
	if err == nil {
		t.Error("expected error for path traversal ID")
	}
}

func TestConcurrentAccess(t *testing.T) {
	srv := newTestServer(t)
	setupPassword(t, srv, "concurrentpass")

	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			manifest := []byte(strings.Repeat("x", 100))
			w := uploadManifest(t, srv, manifest, map[string]any{
				"name": "concurrent-test",
			})
			if w.Code != http.StatusOK {
				errors <- io.EOF
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		if err != nil {
			t.Errorf("concurrent upload failed: %v", err)
		}
	}

	// Verify at least one bundle exists
	if !srv.store.HasManifest() {
		t.Error("expected at least one manifest after concurrent uploads")
	}
}

func TestRootRedirects(t *testing.T) {
	srv := newTestServer(t)

	// Before setup: should show setup page (200, not redirect)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for setup page, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Set up ReMemory") {
		t.Error("expected setup page content")
	}

	// After setup, no manifest: redirect to /create
	setupPassword(t, srv, "redirectpass1")
	req = httptest.NewRequest("GET", "/", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
	if w.Header().Get("Location") != "/create" {
		t.Errorf("expected redirect to /create, got %s", w.Header().Get("Location"))
	}

	// After upload: redirect to /recover
	manifest := []byte("test-manifest")
	uploadManifest(t, srv, manifest, nil)
	req = httptest.NewRequest("GET", "/", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
	if w.Header().Get("Location") != "/recover" {
		t.Errorf("expected redirect to /recover, got %s", w.Header().Get("Location"))
	}
}

func TestEmptyPasswordRejected(t *testing.T) {
	srv := newTestServer(t)

	body := `{"password":""}`
	req := httptest.NewRequest("POST", "/api/setup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty password, got %d", w.Code)
	}
}
