import { useMemo, useState } from "react";
import {
  MarkdownEditor,
  type MarkdownEditorPreviewFormat,
  type MarkdownEditorPreviewRequest,
  type MarkdownEditorPreviewResult,
} from "@flanksource/clicky-ui";

const SAMPLE_MARKDOWN = `# Stack rollout notes

The **api** stack is ready to promote after the canary check completes.

| Stack | Cluster | Status |
| ----- | ------- | ------ |
| api   | prod-eu | healthy |
| worker | prod-us | degraded |

> Reconcile the worker queue before approving the final restart.

:::warning Manual review
Check the deployment evidence before publishing this note.
:::`;

export function MarkdownEditorPage() {
  const [markdown, setMarkdown] = useState(SAMPLE_MARKDOWN);
  const loadPreview = useMemo(
    () => (request: MarkdownEditorPreviewRequest) => loadMarkdownPreview(request),
    [],
  );

  return (
    <main className="min-h-0 overflow-auto bg-muted/20">
      <div className="mx-auto flex w-full max-w-7xl flex-col gap-4 px-6 py-6">
        <div className="space-y-1">
          <h1 className="text-xl font-semibold tracking-tight">Markdown Editor</h1>
          <p className="max-w-3xl text-sm text-muted-foreground">
            Edit markdown and preview it through the same Clicky formatting stack used by
            the CLI and executor API.
          </p>
        </div>
        <MarkdownEditor
          value={markdown}
          onChange={setMarkdown}
          label="Rollout note"
          minHeight={520}
          previewEndpoint="/api/examples/markdown-preview"
          loadPreview={loadPreview}
        />
      </div>
    </main>
  );
}

async function loadMarkdownPreview({
  markdown,
  format,
  signal,
}: MarkdownEditorPreviewRequest): Promise<MarkdownEditorPreviewResult> {
  const response = await fetch(
    `/api/examples/markdown-preview?format=${encodeURIComponent(clickyFormat(format))}`,
    {
      method: "POST",
      headers: {
        Accept: acceptHeader(format),
        "Content-Type": "text/markdown; charset=utf-8",
      },
      body: markdown,
      signal,
    },
  );
  if (!response.ok) {
    throw new Error(
      `Preview failed with ${response.status}: ${await response.text() || response.statusText}`,
    );
  }

  const contentType = response.headers.get("Content-Type") ?? "";
  if (format === "pdf" || format === "excel") {
    return {
      kind: "blob",
      blob: await response.blob(),
      contentType,
      filename: format === "excel" ? "markdown-preview.xlsx" : "markdown-preview.pdf",
    };
  }

  const text = await response.text();
  if (format === "react") {
    return { kind: "clicky", data: text };
  }
  if (format === "html") {
    return { kind: "html", html: text };
  }
  if (format === "json") {
    return { kind: "json", data: JSON.parse(text) as unknown };
  }
  return { kind: "text", text, contentType };
}

function clickyFormat(format: MarkdownEditorPreviewFormat) {
  return format === "react" ? "clicky-json" : format;
}

function acceptHeader(format: MarkdownEditorPreviewFormat) {
  switch (format) {
    case "react":
      return "application/json+clicky";
    case "html":
      return "text/html";
    case "markdown":
      return "text/markdown";
    case "pdf":
      return "application/pdf";
    case "json":
      return "application/json";
    case "csv":
      return "text/csv";
    case "excel":
      return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet";
  }
}
