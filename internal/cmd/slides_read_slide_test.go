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

func readSlidePresResponse() map[string]any {
	return map[string]any{
		"presentationId": "pres1",
		"slides": []any{
			map[string]any{
				"objectId": "slide_1",
				"slideProperties": map[string]any{
					"notesPage": map[string]any{
						"notesProperties": map[string]any{
							"speakerNotesObjectId": "notes_1",
						},
						"pageElements": []any{
							map[string]any{
								"objectId": "notes_1",
								"shape": map[string]any{
									"placeholder": map[string]any{"type": "BODY"},
									"text": map[string]any{
										"textElements": []any{
											map[string]any{
												"textRun": map[string]any{
													"content": "These are speaker notes",
												},
											},
										},
									},
								},
							},
						},
					},
				},
				"pageElements": []any{
					map[string]any{
						"objectId": "text_el_1",
						"shape": map[string]any{
							"text": map[string]any{
								"textElements": []any{
									map[string]any{
										"textRun": map[string]any{
											"content": "Slide Title",
										},
									},
								},
							},
						},
					},
					map[string]any{
						"objectId": "img_el_1",
						"image": map[string]any{
							"contentUrl": "https://example.com/image.png",
						},
					},
				},
			},
		},
	}
}

func readSlideRecursivePresResponse() map[string]any {
	return map[string]any{
		"presentationId": "pres1",
		"slides": []any{
			map[string]any{
				"objectId": "slide_recursive",
				"slideProperties": map[string]any{
					"notesPage": map[string]any{},
				},
				"pageElements": []any{
					map[string]any{
						"objectId": "group_1",
						"elementGroup": map[string]any{
							"children": []any{
								map[string]any{
									"objectId": "group_child_shape",
									"shape": map[string]any{
										"text": map[string]any{
											"textElements": []any{
												map[string]any{
													"textRun": map[string]any{
														"content": "Faster provisioning",
													},
												},
											},
										},
									},
								},
								map[string]any{
									"objectId": "group_child_wordart",
									"wordArt": map[string]any{
										"renderedText": "Grow our ecosystem",
									},
								},
								map[string]any{
									"objectId": "nested_group",
									"elementGroup": map[string]any{
										"children": []any{
											map[string]any{
												"objectId": "nested_group_child_shape",
												"shape": map[string]any{
													"text": map[string]any{
														"textElements": []any{
															map[string]any{
																"textRun": map[string]any{
																	"content": "Operational consistency",
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
					map[string]any{
						"objectId": "table_1",
						"table": map[string]any{
							"tableRows": []any{
								map[string]any{
									"tableCells": []any{
										map[string]any{
											"text": map[string]any{
												"textElements": []any{
													map[string]any{
														"textRun": map[string]any{
															"content": "Cell A",
														},
													},
												},
											},
										},
										map[string]any{
											"text": map[string]any{
												"textElements": []any{
													map[string]any{
														"textRun": map[string]any{
															"content": "Cell B",
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func TestSlidesReadSlide(t *testing.T) {
	origSlides := newSlidesService
	t.Cleanup(func() { newSlidesService = origSlides })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/presentations/pres1") && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(readSlidePresResponse())
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	svc, err := slides.NewService(context.Background(),
		option.WithoutAuthentication(),
		option.WithHTTPClient(srv.Client()),
		option.WithEndpoint(srv.URL+"/"),
	)
	if err != nil {
		t.Fatalf("slides.NewService: %v", err)
	}
	newSlidesService = func(context.Context, string) (*slides.Service, error) { return svc, nil }

	flags := &RootFlags{Account: "a@b.com"}

	out := captureStdout(t, func() {
		u, uiErr := ui.New(ui.Options{Stdout: os.Stdout, Stderr: io.Discard, Color: "never"})
		if uiErr != nil {
			t.Fatalf("ui.New: %v", uiErr)
		}
		ctx := ui.WithUI(context.Background(), u)

		cmd := &SlidesReadSlideCmd{
			PresentationID: "pres1",
			SlideID:        "slide_1",
		}
		if err := cmd.Run(ctx, flags); err != nil {
			t.Fatalf("Run: %v", err)
		}
	})

	if !strings.Contains(out, "Slide 1") {
		t.Errorf("expected slide number, got: %q", out)
	}
	if !strings.Contains(out, "These are speaker notes") {
		t.Errorf("expected speaker notes, got: %q", out)
	}
	if !strings.Contains(out, "Slide Title") {
		t.Errorf("expected text element, got: %q", out)
	}
	if !strings.Contains(out, "img_el_1") {
		t.Errorf("expected image element, got: %q", out)
	}
}

func TestSlidesReadSlide_JSON(t *testing.T) {
	origSlides := newSlidesService
	t.Cleanup(func() { newSlidesService = origSlides })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/presentations/pres1") && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(readSlidePresResponse())
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	svc, err := slides.NewService(context.Background(),
		option.WithoutAuthentication(),
		option.WithHTTPClient(srv.Client()),
		option.WithEndpoint(srv.URL+"/"),
	)
	if err != nil {
		t.Fatalf("slides.NewService: %v", err)
	}
	newSlidesService = func(context.Context, string) (*slides.Service, error) { return svc, nil }

	flags := &RootFlags{Account: "a@b.com"}

	out := captureStdout(t, func() {
		u, uiErr := ui.New(ui.Options{Stdout: io.Discard, Stderr: io.Discard, Color: "never"})
		if uiErr != nil {
			t.Fatalf("ui.New: %v", uiErr)
		}
		ctx := ui.WithUI(context.Background(), u)
		ctx = outfmt.WithMode(ctx, outfmt.Mode{JSON: true})

		cmd := &SlidesReadSlideCmd{
			PresentationID: "pres1",
			SlideID:        "slide_1",
		}
		if err := cmd.Run(ctx, flags); err != nil {
			t.Fatalf("Run: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("JSON parse: %v\noutput: %q", err, out)
	}
	if result["slideNumber"] != float64(1) {
		t.Errorf("expected slideNumber=1, got %v", result["slideNumber"])
	}
	if result["notes"] != "These are speaker notes" {
		t.Errorf("expected notes text, got %v", result["notes"])
	}
	if result["slideObjectId"] != "slide_1" {
		t.Errorf("expected slideObjectId=slide_1, got %v", result["slideObjectId"])
	}

	textEls, ok := result["textElements"].([]any)
	if !ok || len(textEls) != 1 {
		t.Errorf("expected 1 text element, got %v", result["textElements"])
	}

	imgs, ok := result["images"].([]any)
	if !ok || len(imgs) != 1 {
		t.Errorf("expected 1 image, got %v", result["images"])
	}
}

func TestSlidesReadSlide_RecursiveJSON(t *testing.T) {
	origSlides := newSlidesService
	t.Cleanup(func() { newSlidesService = origSlides })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/presentations/pres1") && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(readSlideRecursivePresResponse())
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	svc, err := slides.NewService(context.Background(),
		option.WithoutAuthentication(),
		option.WithHTTPClient(srv.Client()),
		option.WithEndpoint(srv.URL+"/"),
	)
	if err != nil {
		t.Fatalf("slides.NewService: %v", err)
	}
	newSlidesService = func(context.Context, string) (*slides.Service, error) { return svc, nil }

	flags := &RootFlags{Account: "a@b.com"}

	out := captureStdout(t, func() {
		u, uiErr := ui.New(ui.Options{Stdout: io.Discard, Stderr: io.Discard, Color: "never"})
		if uiErr != nil {
			t.Fatalf("ui.New: %v", uiErr)
		}
		ctx := ui.WithUI(context.Background(), u)
		ctx = outfmt.WithMode(ctx, outfmt.Mode{JSON: true})

		cmd := &SlidesReadSlideCmd{
			PresentationID: "pres1",
			SlideID:        "slide_recursive",
			Recursive:      true,
		}
		if err := cmd.Run(ctx, flags); err != nil {
			t.Fatalf("Run: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("JSON parse: %v\noutput: %q", err, out)
	}

	textEls, ok := result["textElements"].([]any)
	if !ok {
		t.Fatalf("expected textElements array, got %T", result["textElements"])
	}

	var gotTexts []string
	for _, raw := range textEls {
		m, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("expected text element object, got %T", raw)
		}
		text, _ := m["text"].(string)
		gotTexts = append(gotTexts, text)
	}

	wantTexts := []string{
		"Faster provisioning",
		"Grow our ecosystem",
		"Operational consistency",
		"Cell A\nCell B",
	}
	for _, want := range wantTexts {
		found := false
		for _, got := range gotTexts {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected recursive text extraction to include %q, got %v", want, gotTexts)
		}
	}
}

func TestSlidesReadSlide_NonRecursiveSkipsGroupedText(t *testing.T) {
	origSlides := newSlidesService
	t.Cleanup(func() { newSlidesService = origSlides })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/presentations/pres1") && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(readSlideRecursivePresResponse())
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	svc, err := slides.NewService(context.Background(),
		option.WithoutAuthentication(),
		option.WithHTTPClient(srv.Client()),
		option.WithEndpoint(srv.URL+"/"),
	)
	if err != nil {
		t.Fatalf("slides.NewService: %v", err)
	}
	newSlidesService = func(context.Context, string) (*slides.Service, error) { return svc, nil }

	flags := &RootFlags{Account: "a@b.com"}

	out := captureStdout(t, func() {
		u, uiErr := ui.New(ui.Options{Stdout: io.Discard, Stderr: io.Discard, Color: "never"})
		if uiErr != nil {
			t.Fatalf("ui.New: %v", uiErr)
		}
		ctx := ui.WithUI(context.Background(), u)
		ctx = outfmt.WithMode(ctx, outfmt.Mode{JSON: true})

		cmd := &SlidesReadSlideCmd{
			PresentationID: "pres1",
			SlideID:        "slide_recursive",
		}
		if err := cmd.Run(ctx, flags); err != nil {
			t.Fatalf("Run: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("JSON parse: %v\noutput: %q", err, out)
	}

	textEls, ok := result["textElements"].([]any)
	if !ok {
		t.Fatalf("expected textElements array, got %T", result["textElements"])
	}
	if len(textEls) != 1 {
		t.Fatalf("expected only top-level table text without recursive extraction, got %v", textEls)
	}

	m, ok := textEls[0].(map[string]any)
	if !ok {
		t.Fatalf("expected text element object, got %T", textEls[0])
	}
	if got := m["text"]; got != "Cell A\nCell B" {
		t.Fatalf("expected only top-level table text, got %v", got)
	}
}

func TestSlidesReadSlide_NotFound(t *testing.T) {
	origSlides := newSlidesService
	t.Cleanup(func() { newSlidesService = origSlides })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/presentations/pres1") && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(readSlidePresResponse())
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	svc, err := slides.NewService(context.Background(),
		option.WithoutAuthentication(),
		option.WithHTTPClient(srv.Client()),
		option.WithEndpoint(srv.URL+"/"),
	)
	if err != nil {
		t.Fatalf("slides.NewService: %v", err)
	}
	newSlidesService = func(context.Context, string) (*slides.Service, error) { return svc, nil }

	flags := &RootFlags{Account: "a@b.com"}
	u, uiErr := ui.New(ui.Options{Stdout: io.Discard, Stderr: io.Discard, Color: "never"})
	if uiErr != nil {
		t.Fatalf("ui.New: %v", uiErr)
	}
	ctx := ui.WithUI(context.Background(), u)

	cmd := &SlidesReadSlideCmd{
		PresentationID: "pres1",
		SlideID:        "nonexistent",
	}
	err = cmd.Run(ctx, flags)
	if err == nil || !strings.Contains(err.Error(), `slide "nonexistent" not found`) {
		t.Fatalf("expected slide-not-found error, got: %v", err)
	}
}

func TestSlidesReadSlide_NoNotes(t *testing.T) {
	origSlides := newSlidesService
	t.Cleanup(func() { newSlidesService = origSlides })

	presResp := map[string]any{
		"presentationId": "pres1",
		"slides": []any{
			map[string]any{
				"objectId": "slide_1",
				"slideProperties": map[string]any{
					"notesPage": map[string]any{},
				},
				"pageElements": []any{},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/presentations/pres1") && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(presResp)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	svc, err := slides.NewService(context.Background(),
		option.WithoutAuthentication(),
		option.WithHTTPClient(srv.Client()),
		option.WithEndpoint(srv.URL+"/"),
	)
	if err != nil {
		t.Fatalf("slides.NewService: %v", err)
	}
	newSlidesService = func(context.Context, string) (*slides.Service, error) { return svc, nil }

	flags := &RootFlags{Account: "a@b.com"}

	out := captureStdout(t, func() {
		u, uiErr := ui.New(ui.Options{Stdout: os.Stdout, Stderr: io.Discard, Color: "never"})
		if uiErr != nil {
			t.Fatalf("ui.New: %v", uiErr)
		}
		ctx := ui.WithUI(context.Background(), u)

		cmd := &SlidesReadSlideCmd{
			PresentationID: "pres1",
			SlideID:        "slide_1",
		}
		if err := cmd.Run(ctx, flags); err != nil {
			t.Fatalf("Run: %v", err)
		}
	})

	if !strings.Contains(out, "Speaker Notes: (none)") {
		t.Errorf("expected '(none)' for empty notes, got: %q", out)
	}
}
