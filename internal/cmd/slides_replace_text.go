package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"google.golang.org/api/slides/v1"

	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/ui"
)

type SlidesReplaceTextCmd struct {
	PresentationID string `arg:"" name:"presentationId" help:"Presentation ID"`
	Find           string `arg:"" name:"find" help:"Text to find"`
	Replace        string `arg:"" name:"replace" help:"Replacement text"`
	PageID         string `name:"page-id" help:"Limit replacement to a specific slide/page object ID"`
	MatchCase      bool   `name:"match-case" help:"Use case-sensitive matching"`
}

func (c *SlidesReplaceTextCmd) Run(ctx context.Context, flags *RootFlags) error {
	u := ui.FromContext(ctx)

	account, err := requireAccount(flags)
	if err != nil {
		return err
	}

	presentationID := strings.TrimSpace(c.PresentationID)
	if presentationID == "" {
		return usage("empty presentationId")
	}

	find := c.Find
	if strings.TrimSpace(find) == "" {
		return usage("empty find")
	}

	pageID := strings.TrimSpace(c.PageID)

	slidesSvc, err := newSlidesService(ctx, account)
	if err != nil {
		return err
	}

	req := &slides.ReplaceAllTextRequest{
		ContainsText: &slides.SubstringMatchCriteria{
			Text:      find,
			MatchCase: c.MatchCase,
		},
		ReplaceText: c.Replace,
	}
	if pageID != "" {
		req.PageObjectIds = []string{pageID}
	}

	result, err := slidesSvc.Presentations.BatchUpdate(presentationID, &slides.BatchUpdatePresentationRequest{
		Requests: []*slides.Request{{
			ReplaceAllText: req,
		}},
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("replace text: %w", err)
	}

	var occurrences int64
	if len(result.Replies) > 0 && result.Replies[0] != nil && result.Replies[0].ReplaceAllText != nil {
		occurrences = result.Replies[0].ReplaceAllText.OccurrencesChanged
	}

	payload := map[string]any{
		"presentationId": presentationID,
		"find":           find,
		"replace":        c.Replace,
		"replacements":   occurrences,
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
	u.Out().Printf("find\t%s", find)
	u.Out().Printf("replace\t%s", c.Replace)
	u.Out().Printf("replacements\t%d", occurrences)
	if pageID != "" {
		u.Out().Printf("pageId\t%s", pageID)
	}
	if c.MatchCase {
		u.Out().Printf("matchCase\ttrue")
	}

	return nil
}
