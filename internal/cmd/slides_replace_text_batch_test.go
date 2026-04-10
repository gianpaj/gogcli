package cmd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/api/option"
	"google.golang.org/api/slides/v1"

	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/ui"
)

func newSlidesReplaceTextBatchTestService(t *testing.T, h http.HandlerFunc) *slides.Service {
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

func TestSlidesReplaceTextBatch(t *testing.T) {
	origSlides := newSlidesService
	t.Cleanup(func() { newSlidesService = origSlides })

	var got slides.BatchUpdatePresentationRequest
	svc := newSlidesReplaceTextBatchTestService(t, func(w http.ResponseWriter, r *http.Request) {
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
		u, uiErr := ui.New(ui.Options{Stdout: os.Stdout, Stderr: io.Discard, Color: "never"})
		if uiErr != nil {
			t.Fatalf("ui.New: %v", uiErr)
		}
		ctx := ui.WithUI(context.Background(), u)

		cmd := &SlidesReplaceTextBatchCmd{}
		if err := runKong(t, cmd, []string{
			"pres1",
			"--replace", "Draft=Final",
			"--replace", "TODO=DONE",
		}, ctx, flags); err != nil {
			t.Fatalf("slides replace-text-batch: %v", err)
		}
	})

	if len(got.Requests) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(got.Requests))
	}

	req1 := got.Requests[0].ReplaceAllText
	if req1 == nil {
		t.Fatal("expected first ReplaceAllText request")
	}
	if req1.ContainsText == nil || req1.ContainsText.Text != "Draft" {
		t.Fatalf("expected first find text %q, got %+v", "Draft", req1.ContainsText)
	}
	if req1.ContainsText.MatchCase {
		t.Fatal("expected matchCase=false by default")
	}
	if req1.ReplaceText != "Final" {
		t.Fatalf("expected first replace text %q, got %q", "Final", req1.ReplaceText)
	}
	if len(req1.PageObjectIds) != 0 {
		t.Fatalf("expected no page scoping by default, got %v", req1.PageObjectIds)
	}

	req2 := got.Requests[1].ReplaceAllText
	if req2 == nil {
		t.Fatal("expected second ReplaceAllText request")
	}
	if req2.ContainsText == nil || req2.ContainsText.Text != "TODO" {
		t.Fatalf("expected second find text %q, got %+v", "TODO", req2.ContainsText)
	}
	if req2.ReplaceText != "DONE" {
		t.Fatalf("expected second replace text %q, got %q", "DONE", req2.ReplaceText)
	}

	if !strings.Contains(out, "presentationId\tpres1") {
		t.Errorf("expected presentationId in output, got: %q", out)
	}
	if !strings.Contains(out, "replacementCount\t2") {
		t.Errorf("expected replacementCount in output, got: %q", out)
	}
	if !strings.Contains(out, "totalChanged\t3") {
		t.Errorf("expected totalChanged in output, got: %q", out)
	}
	if !strings.Contains(out, "Replacements:") {
		t.Errorf("expected replacements section in output, got: %q", out)
	}
	if !strings.Contains(out, "Draft\tFinal\t2") {
		t.Errorf("expected Draft replacement line in output, got: %q", out)
	}
	if !strings.Contains(out, "TODO\tDONE\t1") {
		t.Errorf("expected TODO replacement line in output, got: %q", out)
	}
}

func TestSlidesReplaceTextBatch_PageIDAndMatchCase_JSON(t *testing.T) {
	origSlides := newSlidesService
	t.Cleanup(func() { newSlidesService = origSlides })

	var got slides.BatchUpdatePresentationRequest
	svc := newSlidesReplaceTextBatchTestService(t, func(w http.ResponseWriter, r *http.Request) {
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
					map[string]any{
						"replaceAllText": map[string]any{
							"occurrencesChanged": 4,
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

		cmd := &SlidesReplaceTextBatchCmd{}
		if err := runKong(t, cmd, []string{
			"pres1",
			"--replace", "twin.org={{title}}",
			"--replace", "Importance={{item_1_title}}",
			"--page-id", "slide_7",
			"--match-case",
		}, ctx, flags); err != nil {
			t.Fatalf("slides replace-text-batch --json: %v", err)
		}
	})

	if len(got.Requests) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(got.Requests))
	}

	for i, want := range []struct {
		find    string
		replace string
	}{
		{find: "twin.org", replace: "{{title}}"},
		{find: "Importance", replace: "{{item_1_title}}"},
	} {
		req := got.Requests[i].ReplaceAllText
		if req == nil {
			t.Fatalf("expected ReplaceAllText request %d", i)
		}
		if req.ContainsText == nil || req.ContainsText.Text != want.find {
			t.Fatalf("expected find text %q, got %+v", want.find, req.ContainsText)
		}
		if !req.ContainsText.MatchCase {
			t.Fatalf("expected matchCase=true for request %d", i)
		}
		if req.ReplaceText != want.replace {
			t.Fatalf("expected replace text %q, got %q", want.replace, req.ReplaceText)
		}
		if len(req.PageObjectIds) != 1 || req.PageObjectIds[0] != "slide_7" {
			t.Fatalf("expected page scoping to slide_7, got %v", req.PageObjectIds)
		}
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("JSON parse: %v\noutput: %q", err, out)
	}

	if got := result["presentationId"]; got != "pres1" {
		t.Errorf("expected presentationId=pres1, got %v", got)
	}
	if got := result["replacementCount"]; got != float64(2) {
		t.Errorf("expected replacementCount=2, got %v", got)
	}
	if got := result["totalChanged"]; got != float64(5) {
		t.Errorf("expected totalChanged=5, got %v", got)
	}
	if got := result["pageId"]; got != "slide_7" {
		t.Errorf("expected pageId=slide_7, got %v", got)
	}
	if got := result["matchCase"]; got != true {
		t.Errorf("expected matchCase=true, got %v", got)
	}

	repls, ok := result["replacements"].(map[string]any)
	if !ok {
		t.Fatalf("expected replacements object, got %T", result["replacements"])
	}
	entry, ok := repls["twin.org"].(map[string]any)
	if !ok {
		t.Fatalf("expected replacements[twin.org] object, got %T", repls["twin.org"])
	}
	if got := entry["replace"]; got != "{{title}}" {
		t.Errorf("expected twin.org replace={{title}}, got %v", got)
	}
	if got := entry["replacements"]; got != float64(1) {
		t.Errorf("expected twin.org replacements=1, got %v", got)
	}
}

func TestSlidesReplaceTextBatch_ReplacementsFile(t *testing.T) {
	origSlides := newSlidesService
	t.Cleanup(func() { newSlidesService = origSlides })

	tmp := t.TempDir()
	replacementsPath := filepath.Join(tmp, "replacements.json")
	if err := os.WriteFile(replacementsPath, []byte(`{
  "Draft": "Final",
  "count": 42,
  "enabled": true,
  "empty": null
}`), 0o644); err != nil {
		t.Fatalf("write replacements file: %v", err)
	}

	var got slides.BatchUpdatePresentationRequest
	svc := newSlidesReplaceTextBatchTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/presentations/pres1:batchUpdate"):
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Fatalf("decode batchUpdate: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"presentationId": "pres1",
				"replies": []any{
					map[string]any{"replaceAllText": map[string]any{"occurrencesChanged": 1}},
					map[string]any{"replaceAllText": map[string]any{"occurrencesChanged": 2}},
					map[string]any{"replaceAllText": map[string]any{"occurrencesChanged": 3}},
					map[string]any{"replaceAllText": map[string]any{"occurrencesChanged": 0}},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})
	newSlidesService = func(context.Context, string) (*slides.Service, error) { return svc, nil }

	flags := &RootFlags{Account: "a@b.com"}
	u, uiErr := ui.New(ui.Options{Stdout: io.Discard, Stderr: io.Discard, Color: "never"})
	if uiErr != nil {
		t.Fatalf("ui.New: %v", uiErr)
	}
	ctx := ui.WithUI(context.Background(), u)

	cmd := &SlidesReplaceTextBatchCmd{}
	if err := runKong(t, cmd, []string{
		"pres1",
		"--replacements", replacementsPath,
	}, ctx, flags); err != nil {
		t.Fatalf("slides replace-text-batch --replacements: %v", err)
	}

	if len(got.Requests) != 4 {
		t.Fatalf("expected 4 requests, got %d", len(got.Requests))
	}

	gotMap := map[string]string{}
	for _, req := range got.Requests {
		if req.ReplaceAllText == nil || req.ReplaceAllText.ContainsText == nil {
			t.Fatalf("expected ReplaceAllText request with ContainsText, got %+v", req)
		}
		gotMap[req.ReplaceAllText.ContainsText.Text] = req.ReplaceAllText.ReplaceText
	}

	if gotMap["Draft"] != "Final" {
		t.Fatalf("expected Draft -> Final, got %q", gotMap["Draft"])
	}
	if gotMap["count"] != "42" {
		t.Fatalf("expected count -> 42, got %q", gotMap["count"])
	}
	if gotMap["enabled"] != "true" {
		t.Fatalf("expected enabled -> true, got %q", gotMap["enabled"])
	}
	if gotMap["empty"] != "" {
		t.Fatalf("expected empty -> empty string, got %q", gotMap["empty"])
	}
}

func TestSlidesReplaceTextBatch_NoReplacements(t *testing.T) {
	flags := &RootFlags{Account: "a@b.com"}
	u, uiErr := ui.New(ui.Options{Stdout: io.Discard, Stderr: io.Discard, Color: "never"})
	if uiErr != nil {
		t.Fatalf("ui.New: %v", uiErr)
	}
	ctx := ui.WithUI(context.Background(), u)

	cmd := &SlidesReplaceTextBatchCmd{}
	err := runKong(t, cmd, []string{"pres1"}, ctx, flags)
	if err == nil || !strings.Contains(err.Error(), "no replacements specified") {
		t.Fatalf("expected no replacements specified error, got: %v", err)
	}
}

func TestSlidesReplaceTextBatch_InvalidReplaceFormat(t *testing.T) {
	flags := &RootFlags{Account: "a@b.com"}
	u, uiErr := ui.New(ui.Options{Stdout: io.Discard, Stderr: io.Discard, Color: "never"})
	if uiErr != nil {
		t.Fatalf("ui.New: %v", uiErr)
	}
	ctx := ui.WithUI(context.Background(), u)

	cmd := &SlidesReplaceTextBatchCmd{}
	err := runKong(t, cmd, []string{
		"pres1",
		"--replace", "missing-separator",
	}, ctx, flags)
	if err == nil || !strings.Contains(err.Error(), `invalid replacement format "missing-separator"`) {
		t.Fatalf("expected invalid replacement format error, got: %v", err)
	}
}

func TestSlidesReplaceTextBatch_APIFailure(t *testing.T) {
	origSlides := newSlidesService
	t.Cleanup(func() { newSlidesService = origSlides })

	svc := newSlidesReplaceTextBatchTestService(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"boom"}}`, http.StatusInternalServerError)
	})
	newSlidesService = func(context.Context, string) (*slides.Service, error) { return svc, nil }

	flags := &RootFlags{Account: "a@b.com"}
	u, uiErr := ui.New(ui.Options{Stdout: io.Discard, Stderr: io.Discard, Color: "never"})
	if uiErr != nil {
		t.Fatalf("ui.New: %v", uiErr)
	}
	ctx := ui.WithUI(context.Background(), u)

	cmd := &SlidesReplaceTextBatchCmd{}
	err := runKong(t, cmd, []string{
		"pres1",
		"--replace", "foo=bar",
	}, ctx, flags)
	if err == nil || !strings.Contains(err.Error(), "replace text batch:") {
		t.Fatalf("expected replace text batch error, got: %v", err)
	}
}
