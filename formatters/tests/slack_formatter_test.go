package formatters

import (
	"testing"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/api"
	clickyformatters "github.com/flanksource/clicky/formatters"
)

func TestSlackFormatter_ScreenshotMessage(t *testing.T) {
	type Alert struct {
		Title   string          `pretty:"title"`
		Org     string          `pretty:"label=org"`
		Divider api.HtmlElement `pretty:"label=divider"`
		Summary api.Textable    `pretty:"title=Summary:"`
		Fixes   api.List        `pretty:"title=Recommended Fix:"`
		Also    api.List        `pretty:"title=Also Affected:"`
		Actions api.ButtonGroup `pretty:"label=Actions"`
	}

	message := Alert{
		Title:   "GitHub Action 'Create Release' Failing 🚨",
		Org:     "flanksource",
		Divider: api.HR,
		Summary: clicky.Text("The Create Release GitHub Actions workflow for `config-db` is failing. A process within the workflow is exiting with code 2, which is blocking the creation of new software releases."),
		Fixes: api.List{
			Bullet: clicky.Text("• "),
			Items: []api.Textable{
				clicky.Text("Check GitHub Actions logs for the exact failing step."),
				clicky.Text("Review the `.github/workflows/release.yml` script for errors."),
				clicky.Text("Verify repository secrets and `GITHUB_TOKEN` permissions."),
			},
		},
		Also: api.List{
			Bullet: clicky.Text("- "),
			Items: []api.Textable{
				clicky.Text("/GitHubAction::Workflow/config-db/Create Release"),
			},
		},
		Actions: api.ButtonGroup{
			Buttons: []api.Button{
				{Label: "View Config", Href: "https://example.com/config", Variant: "primary"},
				{Label: "Silence", Href: "https://example.com/silence", Variant: "secondary"},
			},
		},
	}

	formatter := clickyformatters.NewSlackFormatter()
	out, err := formatter.Format(message, clickyformatters.FormatOptions{})
	if err != nil {
		t.Fatalf("Format() failed: %v", err)
	}

	t.Logf("slack json (screenshot): %s", out)
}
