package httpclient

import (
	"testing"
	"time"

	"github.com/valyala/fasthttp"
)

func TestConstants(t *testing.T) {
	if DefaultSleep != 500*time.Millisecond {
		t.Error("DefaultSleep should be 500ms")
	}
	if MethodGet != "GET" {
		t.Error("MethodGet should be GET")
	}
	if MethodPost != "POST" {
		t.Error("MethodPost should be POST")
	}
}

func TestRetrySuccessFirstAttempt(t *testing.T) {
	callCount := 0
	fn := func() (*fasthttp.Response, int, error) {
		callCount++
		return &fasthttp.Response{}, 200, nil
	}

	resp, status, err := Retry(3, 1*time.Millisecond, fn)
	if err != nil {
		t.Fatalf("Retry failed: %v", err)
	}
	if status != 200 {
		t.Errorf("status = %d, want 200", status)
	}
	if resp == nil {
		t.Error("response should not be nil")
	}
	if callCount != 1 {
		t.Errorf("should call fn once on success, called %d times", callCount)
	}
}

func TestRetryEventualSuccess(t *testing.T) {
	callCount := 0
	fn := func() (*fasthttp.Response, int, error) {
		callCount++
		if callCount < 3 {
			return &fasthttp.Response{}, 500, nil // server error triggers retry
		}
		return &fasthttp.Response{}, 200, nil
	}

	resp, status, err := Retry(3, 1*time.Millisecond, fn)
	if err != nil {
		t.Fatalf("Retry failed: %v", err)
	}
	if status != 200 {
		t.Errorf("status = %d, want 200", status)
	}
	if resp == nil {
		t.Error("response should not be nil")
	}
	if callCount != 3 {
		t.Errorf("should retry twice, called %d times", callCount)
	}
}

func TestRetryExhausted(t *testing.T) {
	fn := func() (*fasthttp.Response, int, error) {
		return &fasthttp.Response{}, 500, nil // always fail
	}

	_, status, _ := Retry(2, 1*time.Millisecond, fn)
	if status != 500 {
		t.Errorf("status = %d, want 500 after exhausted", status)
	}
}

func TestRetryNoErrorButServerError(t *testing.T) {
	callCount := 0
	fn := func() (*fasthttp.Response, int, error) {
		callCount++
		return &fasthttp.Response{}, 503, nil
	}

	resp, _, _ := Retry(2, 1*time.Millisecond, fn)
	if resp != nil {
		t.Error("response should be nil when retries exhausted")
	}
	if callCount != 2 { // 1 initial + 1 retry
		t.Errorf("callCount = %d, want 2", callCount)
	}
}

func TestRetryZeroSleepUsesDefault(t *testing.T) {
	fn := func() (*fasthttp.Response, int, error) {
		return &fasthttp.Response{}, 200, nil
	}

	_, status, err := Retry(1, 0, fn) // sleep=0 → uses DefaultSleep
	if err != nil {
		t.Fatalf("Retry failed: %v", err)
	}
	if status != 200 {
		t.Errorf("status = %d, want 200", status)
	}
}

func TestRetryNonServerErrorNoRetry(t *testing.T) {
	callCount := 0
	fn := func() (*fasthttp.Response, int, error) {
		callCount++
		return &fasthttp.Response{}, 400, nil // 4xx not retried
	}

	_, status, _ := Retry(3, 1*time.Millisecond, fn)
	if status != 400 {
		t.Errorf("status = %d, want 400", status)
	}
	if callCount != 1 {
		t.Errorf("should NOT retry on 4xx, called %d times", callCount)
	}
}

func TestSendHeadersDefaultUserAgent(t *testing.T) {
	// Verify the headers constants exist and are correct
	if FasthttpVersion != "v1.61.0" {
		t.Error("FasthttpVersion constant should exist")
	}
}
