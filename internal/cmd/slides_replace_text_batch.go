package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"google.golang.org/api/slides/v1"

	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/ui"
)

type SlidesReplaceTextBatchCmd struct {
	PresentationID string   `arg:"" name:"presentationId" help:"Presentation ID"`
	Replace        []string `name:"replace" help:"Replacement in format 'find=replace' (repeatable)"`
	Replacements   string   `name:"replacements" help:"JSON file containing replacements" type:"existingfile"`
	PageID         string   `name:"page-id" help:"Limit replacements to a specific slide/page object ID"`
	MatchCase      bool     `name:"match-case" help:"Use case-sensitive matching"`
}

type slidesBatchReplacement struct {
	Find    string
	Replace string
}

func (c *SlidesReplaceTextBatchCmd) Run(ctx context.Context, flags *RootFlags) error {
	u := ui.FromContext(ctx)

	account, err := requireAccount(flags)
	if err != nil {
		return err
	}

	presentationID := strings.TrimSpace(c.PresentationID)
	if presentationID == "" {
		return usage("empty presentationId")
	}

	replacements, err := c.parseReplacements()
	if err != nil {
		return err
	}
	if len(replacements) == 0 {
		return usage("no replacements specified (use --replace or --replacements)")
	}

	pageID := strings.TrimSpace(c.PageID)

	slidesSvc, err := newSlidesService(ctx, account)
	if err != nil {
		return err
	}

	requests := make([]*slides.Request, 0, len(replacements))
	keys := make([]string, 0, len(replacements))
	for _, replacement := range replacements {
		req := &slides.ReplaceAllTextRequest{
			ContainsText: &slides.SubstringMatchCriteria{
				Text:      replacement.Find,
				MatchCase: c.MatchCase,
			},
			ReplaceText: replacement.Replace,
		}
		if pageID != "" {
			req.PageObjectIds = []string{pageID}
		}
		requests = append(requests, &slides.Request{
			ReplaceAllText: req,
		})
		keys = append(keys, replacement.Find)
	}

	result, err := slidesSvc.Presentations.BatchUpdate(presentationID, &slides.BatchUpdatePresentationRequest{
		Requests: requests,
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("replace text batch: %w", err)
	}

	stats := make(map[string]map[string]any, len(keys))
	var total int64
	for i, replacement := range replacements {
		occurrences := int64(0)
		if i < len(result.Replies) && result.Replies[i] != nil && result.Replies[i].ReplaceAllText != nil {
			occurrences = result.Replies[i].ReplaceAllText.OccurrencesChanged
		}
		total += occurrences
		stats[replacement.Find] = map[string]any{
			"replace":      replacement.Replace,
			"replacements": occurrences,
		}
	}

	payload := map[string]any{
		"presentationId":   presentationID,
		"replacementCount": len(replacements),
		"replacements":     stats,
		"totalChanged":     total,
	}
	if pageID != "" {
		payload["pageId"] = pageID
	}
	if c.MatchCase {
		payload["matchCase"] = true
	}

	if outfmt.IsJSON(ctx) {
		return outfmt.WriteJSON(ctx, os.Stdout, payload)
	}

	u.Out().Printf("presentationId\t%s", presentationID)
	u.Out().Printf("replacementCount\t%d", len(replacements))
	u.Out().Printf("totalChanged\t%d", total)
	if pageID != "" {
		u.Out().Printf("pageId\t%s", pageID)
	}
	if c.MatchCase {
		u.Out().Printf("matchCase\ttrue")
	}
	u.Out().Println("")
	u.Out().Println("Replacements:")
	for _, key := range keys {
		entry := stats[key]
		u.Out().Printf("  %s\t%s\t%d", key, entry["replace"], entry["replacements"])
	}

	return nil
}

func (c *SlidesReplaceTextBatchCmd) parseReplacements() ([]slidesBatchReplacement, error) {
	var result []slidesBatchReplacement
	indexByFind := make(map[string]int)

	if c.Replacements != "" {
		data, err := os.ReadFile(c.Replacements)
		if err != nil {
			return nil, fmt.Errorf("failed to read replacements file: %w", err)
		}

		var fileReplacements map[string]any
		if err := json.Unmarshal(data, &fileReplacements); err != nil {
			return nil, fmt.Errorf("invalid JSON in replacements file: %w", err)
		}

		for rawKey, rawValue := range fileReplacements {
			key := strings.TrimSpace(rawKey)
			if key == "" {
				continue
			}
			value, err := stringifySlidesBatchReplacementValue(rawValue)
			if err != nil {
				return nil, fmt.Errorf("cannot convert value for key %q to string: %w", key, err)
			}
			indexByFind[key] = len(result)
			result = append(result, slidesBatchReplacement{
				Find:    key,
				Replace: value,
			})
		}
	}

	for _, replacement := range c.Replace {
		parts := strings.SplitN(replacement, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid replacement format %q (expected find=replace)", replacement)
		}

		find := strings.TrimSpace(parts[0])
		replaceText := parts[1]

		if find == "" {
			return nil, fmt.Errorf("empty find text in replacement %q", replacement)
		}

		if idx, ok := indexByFind[find]; ok {
			result[idx].Replace = replaceText
			continue
		}

		indexByFind[find] = len(result)
		result = append(result, slidesBatchReplacement{
			Find:    find,
			Replace: replaceText,
		})
	}

	return result, nil
}

func stringifySlidesBatchReplacementValue(v any) (string, error) {
	switch val := v.(type) {
	case string:
		return val, nil
	case float64:
		return fmt.Sprintf("%g", val), nil
	case bool:
		return fmt.Sprintf("%t", val), nil
	case nil:
		return "", nil
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
}
