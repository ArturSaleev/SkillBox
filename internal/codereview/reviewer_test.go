package codereview_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aibox/skillbox/internal/codereview"
)

func TestLocalOpenAICompatibleReviewDoesNotRequireExternalConsent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path=%s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "main.py") {
			t.Error("code was not sent to local provider")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"summary\":\"reviewed\",\"findings\":[],\"recommendation\":\"manual review\"}"}}]}`))
	}))
	defer server.Close()
	reviewer, err := codereview.NewOpenAICompatible(server.URL, "local-model", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if reviewer.Destination().External {
		t.Fatal("loopback provider marked external")
	}
	result, err := codereview.Review(context.Background(), reviewer, codereview.Request{PackageName: "test", Files: []codereview.File{{Path: "main.py", Content: "print('x')"}}})
	if err != nil || result.Summary != "reviewed" || result.Model != "local-model" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestExternalProviderIsBlockedBeforeCodeTransmissionWithoutApproval(t *testing.T) {
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("external request must not be sent")
		return nil, nil
	})
	reviewer, err := codereview.NewOpenAICompatible("https://review.example.com", "remote-model", "secret", &http.Client{Transport: transport})
	if err != nil {
		t.Fatal(err)
	}
	if !reviewer.Destination().External {
		t.Fatal("remote provider not marked external")
	}
	_, err = codereview.Review(context.Background(), reviewer, codereview.Request{Files: []codereview.File{{Path: "secret.py", Content: "private"}}})
	if !errors.Is(err, codereview.ErrExternalConsentRequired) {
		t.Fatalf("err=%v", err)
	}
}

func TestExternalProviderSendsOnlyAfterExplicitApproval(t *testing.T) {
	called := false
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		called = true
		if request.Header.Get("Authorization") != "Bearer api-key" {
			t.Error("missing API authorization")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"manual findings"}}]}`))}, nil
	})
	reviewer, err := codereview.NewOpenAICompatible("https://review.example.com/v1", "remote-model", "api-key", &http.Client{Transport: transport})
	if err != nil {
		t.Fatal(err)
	}
	result, err := codereview.Review(context.Background(), reviewer, codereview.Request{ExternalTransmissionApproved: true, Files: []codereview.File{{Path: "main.go", Content: "package main"}}})
	if err != nil || !called || result.Summary != "manual findings" {
		t.Fatalf("called=%v result=%#v err=%v", called, result, err)
	}
}

func TestRegistryIsExtensible(t *testing.T) {
	registry := codereview.DefaultRegistry()
	if err := registry.Register("future-anthropic", func(c codereview.Config) (codereview.CodeReviewer, error) { return stubReviewer{}, nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Create(codereview.Config{Provider: "future-anthropic"}); err != nil {
		t.Fatal(err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

type stubReviewer struct{}

func (stubReviewer) Name() string                        { return "stub" }
func (stubReviewer) Destination() codereview.Destination { return codereview.Destination{} }
func (stubReviewer) Review(context.Context, codereview.Request) (codereview.Result, error) {
	return codereview.Result{}, nil
}
