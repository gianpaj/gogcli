package cmd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"google.golang.org/api/option"
	"google.golang.org/api/slides/v1"

	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/ui"
)

func newSlidesReplaceTextTestService(t *testing.T, h http.HandlerFunc) *slides.Service {
	t.Helper()

	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	svc, err := slides.NewService(context.Background(),
		option.WithoutAuthentication(),
		option.WithHTTPClient(srv.Client()),
		option.WithEndpoint(srv.URL+"/"),
	)
	if err != nil {
		t.Fatalf("slides.NewService: %v", err)
	}
	return svc
}

func TestSlidesReplaceText(t *testing.T) {
	origSlides := newSlidesService
	t.Cleanup(func() { newSlidesService = origSlides })

	var got slides.BatchUpdatePresentationRequest
	svc := newSlidesReplaceTextTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/presentations/pres1:batchUpdate"):
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Fatalf("decode batchUpdate: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"presentationId": "pres1",
				"replies": []any{
					map[string]any{
						"replaceAllText": map[string]any{
							"occurrencesChanged": 2,
						},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})
	newSlidesService = func(context.Context, string) (*slides.Service, error) { return svc, nil }

	flags := &RootFlags{Account: "a@b.com"}

	out := captureStdout(t, func() {
		u, uiErr := ui.New(ui.Options{Stdout: os.Stdout, Stderr: io.Discard, Color: "never"})
		if uiErr != nil {
			t.Fatalf("ui.New: %v", uiErr)
		}
		ctx := ui.WithUI(context.Background(), u)

		cmd := &SlidesReplaceTextCmd{}
		if err := runKong(t, cmd, []string{"pres1", "Draft", "Final"}, ctx, flags); err != nil {
			t.Fatalf("slides replace-text: %v", err)
		}
	})

	if len(got.Requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(got.Requests))
	}
	req := got.Requests[0].ReplaceAllText
	if req == nil {
		t.Fatal("expected ReplaceAllText request")
	}
	if req.ContainsText == nil {
		t.Fatal("expected ContainsText criteria")
	}
	if req.ContainsText.Text != "Draft" {
		t.Fatalf("expected find text %q, got %q", "Draft", req.ContainsText.Text)
	}
	if req.ContainsText.MatchCase {
		t.Fatal("expected matchCase=false by default")
	}
	if req.ReplaceText != "Final" {
		t.Fatalf("expected replace text %q, got %q", "Final", req.ReplaceText)
	}
	if len(req.PageObjectIds) != 0 {
		t.Fatalf("expected no page scoping by default, got %v", req.PageObjectIds)
	}

	if !strings.Contains(out, "presentationId\tpres1") {
		t.Errorf("expected presentationId in output, got: %q", out)
	}
	if !strings.Contains(out, "find\tDraft") {
		t.Errorf("expected find in output, got: %q", out)
	}
	if !strings.Contains(out, "replace\tFinal") {
		t.Errorf("expected replace in output, got: %q", out)
	}
	if !strings.Contains(out, "replacements\t2") {
		t.Errorf("expected replacements count in output, got: %q", out)
	}
}

func TestSlidesReplaceText_PageIDAndMatchCase_JSON(t *testing.T) {
	origSlides := newSlidesService
	t.Cleanup(func() { newSlidesService = origSlides })

	var got slides.BatchUpdatePresentationRequest
	svc := newSlidesReplaceTextTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/presentations/pres1:batchUpdate"):
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Fatalf("decode batchUpdate: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"presentationId": "pres1",
				"replies": []any{
					map[string]any{
						"replaceAllText": map[string]any{
							"occurrencesChanged": 1,
						},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})
	newSlidesService = func(context.Context, string) (*slides.Service, error) { return svc, nil }

	flags := &RootFlags{Account: "a@b.com"}

	out := captureStdout(t, func() {
		u, uiErr := ui.New(ui.Options{Stdout: io.Discard, Stderr: io.Discard, Color: "never"})
		if uiErr != nil {
			t.Fatalf("ui.New: %v", uiErr)
		}
		ctx := ui.WithUI(context.Background(), u)
		ctx = outfmt.WithMode(ctx, outfmt.Mode{JSON: true})

		cmd := &SlidesReplaceTextCmd{}
		if err := runKong(t, cmd, []string{
			"pres1",
			"twin.org",
			"{{title}}",
			"--page-id", "slide_7",
			"--match-case",
		}, ctx, flags); err != nil {
			t.Fatalf("slides replace-text --json: %v", err)
		}
	})

	if len(got.Requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(got.Requests))
	}
	req := got.Requests[0].ReplaceAllText
	if req == nil {
		t.Fatal("expected ReplaceAllText request")
	}
	if req.ContainsText == nil {
		t.Fatal("expected ContainsText criteria")
	}
	if req.ContainsText.Text != "twin.org" {
		t.Fatalf("expected find text %q, got %q", "twin.org", req.ContainsText.Text)
	}
	if !req.ContainsText.MatchCase {
		t.Fatal("expected matchCase=true")
	}
	if req.ReplaceText != "{{title}}" {
		t.Fatalf("expected replace text %q, got %q", "{{title}}", req.ReplaceText)
	}
	if len(req.PageObjectIds) != 1 || req.PageObjectIds[0] != "slide_7" {
		t.Fatalf("expected page scoping to slide_7, got %v", req.PageObjectIds)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("JSON parse: %v\noutput: %q", err, out)
	}
	if got := result["presentationId"]; got != "pres1" {
		t.Errorf("expected presentationId=pres1, got %v", got)
	}
	if got := result["find"]; got != "twin.org" {
		t.Errorf("expected find=twin.org, got %v", got)
	}
	if got := result["replace"]; got != "{{title}}" {
		t.Errorf("expected replace={{title}}, got %v", got)
	}
	if got := result["replacements"]; got != float64(1) {
		t.Errorf("expected replacements=1, got %v", got)
	}
	if got := result["pageId"]; got != "slide_7" {
		t.Errorf("expected pageId=slide_7, got %v", got)
	}
	if got := result["matchCase"]; got != true {
		t.Errorf("expected matchCase=true, got %v", got)
	}
}

func TestSlidesReplaceText_ZeroOccurrences(t *testing.T) {
	origSlides := newSlidesService
	t.Cleanup(func() { newSlidesService = origSlides })

	svc := newSlidesReplaceTextTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/presentations/pres1:batchUpdate"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"presentationId": "pres1",
				"replies": []any{
					map[string]any{
						"replaceAllText": map[string]any{
							"occurrencesChanged": 0,
						},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})
	newSlidesService = func(context.Context, string) (*slides.Service, error) { return svc, nil }

	flags := &RootFlags{Account: "a@b.com"}

	out := captureStdout(t, func() {
		u, uiErr := ui.New(ui.Options{Stdout: os.Stdout, Stderr: io.Discard, Color: "never"})
		if uiErr != nil {
			t.Fatalf("ui.New: %v", uiErr)
		}
		ctx := ui.WithUI(context.Background(), u)

		cmd := &SlidesReplaceTextCmd{}
		if err := runKong(t, cmd, []string{"pres1", "missing", "whatever"}, ctx, flags); err != nil {
			t.Fatalf("slides replace-text zero occurrences: %v", err)
		}
	})

	if !strings.Contains(out, "replacements\t0") {
		t.Errorf("expected zero replacements in output, got: %q", out)
	}
}

func TestSlidesReplaceText_EmptyPresentationID(t *testing.T) {
	flags := &RootFlags{Account: "a@b.com"}
	u, uiErr := ui.New(ui.Options{Stdout: io.Discard, Stderr: io.Discard, Color: "never"})
	if uiErr != nil {
		t.Fatalf("ui.New: %v", uiErr)
	}
	ctx := ui.WithUI(context.Background(), u)

	cmd := &SlidesReplaceTextCmd{
		Find:    "foo",
		Replace: "bar",
	}
	err := cmd.Run(ctx, flags)
	if err == nil || !strings.Contains(err.Error(), "empty presentationId") {
		t.Fatalf("expected empty presentationId error, got: %v", err)
	}
}

func TestSlidesReplaceText_EmptyFind(t *testing.T) {
	flags := &RootFlags{Account: "a@b.com"}
	u, uiErr := ui.New(ui.Options{Stdout: io.Discard, Stderr: io.Discard, Color: "never"})
	if uiErr != nil {
		t.Fatalf("ui.New: %v", uiErr)
	}
	ctx := ui.WithUI(context.Background(), u)

	cmd := &SlidesReplaceTextCmd{
		PresentationID: "pres1",
		Find:           "   ",
		Replace:        "bar",
	}
	err := cmd.Run(ctx, flags)
	if err == nil || !strings.Contains(err.Error(), "empty find") {
		t.Fatalf("expected empty find error, got: %v", err)
	}
}

func TestSlidesReplaceText_APIFailure(t *testing.T) {
	origSlides := newSlidesService
	t.Cleanup(func() { newSlidesService = origSlides })

	svc := newSlidesReplaceTextTestService(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"boom"}}`, http.StatusInternalServerError)
	})
	newSlidesService = func(context.Context, string) (*slides.Service, error) { return svc, nil }

	flags := &RootFlags{Account: "a@b.com"}
	u, uiErr := ui.New(ui.Options{Stdout: io.Discard, Stderr: io.Discard, Color: "never"})
	if uiErr != nil {
		t.Fatalf("ui.New: %v", uiErr)
	}
	ctx := ui.WithUI(context.Background(), u)

	cmd := &SlidesReplaceTextCmd{
		PresentationID: "pres1",
		Find:           "foo",
		Replace:        "bar",
	}
	err := cmd.Run(ctx, flags)
	if err == nil || !strings.Contains(err.Error(), "replace text:") {
		t.Fatalf("expected replace text error, got: %v", err)
	}
}
