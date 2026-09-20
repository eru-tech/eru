package functions

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
)

func jsonRequest(t *testing.T, payload string, withGetBody bool) *http.Request {
	t.Helper()
	r := &http.Request{
		Method:        "POST",
		Header:        http.Header{},
		Body:          io.NopCloser(bytes.NewBufferString(payload)),
		ContentLength: int64(len(payload)),
	}
	r.Header.Set("Content-Type", "application/json")
	if withGetBody {
		r.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader([]byte(payload))), nil
		}
	}
	return r
}

// Resuming a plan skips the steps that already ran, so the request is not
// re-buffered on the way down and the same one gets cloned again further along.
// The second clone used to come back empty and the step died decoding a body its
// Content-Length promised was there.
func TestCloneRequestReplaysASpentBody(t *testing.T) {
	const payload = `{"content":"add an entity"}`
	src := jsonRequest(t, payload, true)

	first, err := CloneRequest(context.Background(), src)
	if err != nil {
		t.Fatalf("first clone: %v", err)
	}
	if got, _ := io.ReadAll(first.Body); string(got) != payload {
		t.Fatalf("first clone body = %q", got)
	}
	// Drain the source the way an executed step does.
	_, _ = io.ReadAll(src.Body)

	second, err := CloneRequest(context.Background(), src)
	if err != nil {
		t.Fatalf("second clone: %v", err)
	}
	got, _ := io.ReadAll(second.Body)
	if string(got) != payload {
		t.Errorf("a spent source must be replayed from GetBody, got %q", got)
	}
}

// Without GetBody there is nothing to replay from; the clone must still not error.
func TestCloneRequestWithoutGetBodyStillClones(t *testing.T) {
	const payload = `{"content":"x"}`
	src := jsonRequest(t, payload, false)

	if _, err := CloneRequest(context.Background(), src); err != nil {
		t.Fatalf("clone without GetBody: %v", err)
	}
}
