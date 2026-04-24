import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Clicky,
  OperationCommandPage,
  isPositionalParam,
  type ClickyDocument,
  type ClickyResolvedCommand,
  type OpenAPIParameter,
  type RenderLink,
} from "@flanksource/clicky-ui";
import {
  Link,
  useNavigate,
  useParams,
  useSearchParams,
} from "react-router-dom";
import { apiClient } from "./api";

const renderLink: RenderLink = ({ to, className, children, title, key }) => (
  <Link key={key} to={to} className={className} title={title}>
    {children}
  </Link>
);

export function LinkExamplesPage() {
  const navigate = useNavigate();
  const examplesQuery = useQuery<ClickyDocument>({
    queryKey: ["link-examples"],
    queryFn: async () => {
      const response = await fetch("/api/examples/links", {
        headers: { Accept: "application/json+clicky" },
      });
      if (!response.ok) {
        throw new Error(
          `GET /api/examples/links failed with ${response.status}: ${await response.text() || response.statusText}`,
        );
      }
      return (await response.json()) as ClickyDocument;
    },
    staleTime: 30_000,
  });

  const commandRuntime = useMemo(
    () => ({
      client: apiClient,
      hrefForCommand: (resolved: ClickyResolvedCommand) =>
        buildExampleCommandHref(resolved),
      onNavigate: (resolved: ClickyResolvedCommand) => {
        const href = buildExampleCommandHref(resolved);
        if (href) {
          navigate(href);
        }
      },
    }),
    [navigate],
  );

  return (
    <div className="min-h-full overflow-auto bg-[radial-gradient(circle_at_top_left,rgba(14,165,233,0.12),transparent_28%),radial-gradient(circle_at_top_right,rgba(34,197,94,0.10),transparent_24%)]">
      <div className="mx-auto flex w-full max-w-7xl flex-col gap-6 px-6 py-8">
        <section className="rounded-[2rem] border border-border/70 bg-background/90 p-8 shadow-[0_20px_80px_-48px_rgba(15,23,42,0.55)] backdrop-blur">
          <p className="text-xs font-semibold uppercase tracking-[0.24em] text-sky-700">
            examples/enitity
          </p>
          <h1 className="mt-3 text-4xl font-semibold tracking-tight text-foreground">
            Link and LinkCommand examples
          </h1>
          <p className="mt-4 max-w-3xl text-sm leading-6 text-muted-foreground">
            This page fetches a real Clicky document from the Go demo server, so every
            example below is authored with <code>clicky.Link</code> or{" "}
            <code>clicky.LinkCommand</code> on the backend. Browser-target command
            links deep-link into a prefilled command page under{" "}
            <code>/links/commands/:operationId</code>, while dialog, hover, and expand
            execute in-place through <code>{"<Clicky />"}</code>.
          </p>
        </section>

        <div className="grid gap-6 xl:grid-cols-[minmax(0,1.35fr)_minmax(19rem,0.85fr)]">
          <section className="rounded-[2rem] border border-border/70 bg-background/92 p-6 shadow-[0_16px_64px_-44px_rgba(15,23,42,0.5)] backdrop-blur">
            <div className="mb-5 flex items-end justify-between gap-4">
              <div>
                <h2 className="text-lg font-semibold tracking-tight">
                  Go-emitted Clicky document
                </h2>
                <p className="mt-1 text-sm text-muted-foreground">
                  Plain links and command links render together, using the same runtime
                  as the entity explorer.
                </p>
              </div>
              <span className="rounded-full border border-border/70 bg-muted/50 px-3 py-1 text-xs text-muted-foreground">
                /api/examples/links
              </span>
            </div>

            {examplesQuery.isPending ? (
              <ExampleNotice
                title="Loading examples"
                body="Fetching the Clicky document and OpenAPI metadata."
              />
            ) : examplesQuery.isError ? (
              <ExampleNotice
                title="Failed to load examples"
                tone="danger"
                body={
                  examplesQuery.error instanceof Error
                    ? examplesQuery.error.message
                    : String(examplesQuery.error ?? "Unknown error")
                }
              />
            ) : (
              <Clicky data={examplesQuery.data} commandRuntime={commandRuntime} />
            )}
          </section>

          <aside className="space-y-5">
            <InfoCard
              eyebrow="What to click"
              title="Plain link targets"
              body="Default, _self, _window, and _tab are rendered as anchors. The demo routes those links back into the explorer so you can inspect how each target behaves."
            />
            <InfoCard
              eyebrow="What to click"
              title="Command link targets"
              body="Dialog, Hover, and Expand execute in-place. _clicky uses onNavigate, and the browser targets build a deep-link URL with prefilled params plus autoRun=true."
            />
            <InfoCard
              eyebrow="Deep-link shape"
              title="/links/commands/:operationId"
              body="The command page reads query params into initialValues and auto-runs when required params are already present. That mirrors the intended host-app integration without changing the shared explorer router."
            />
          </aside>
        </div>
      </div>
    </div>
  );
}

export function LinkExampleCommandPage() {
  const { operationId } = useParams();
  const [searchParams] = useSearchParams();
  const initialValues = useMemo(() => {
    const values: Record<string, string> = {};
    for (const [key, value] of searchParams.entries()) {
      if (key === "autoRun") {
        continue;
      }
      values[key] = value;
    }
    return values;
  }, [searchParams]);
  const autoRun = useMemo(() => {
    const value = searchParams.get("autoRun");
    return value === "1" || value === "true";
  }, [searchParams]);

  return (
    <div className="min-h-full overflow-auto bg-[linear-gradient(180deg,rgba(148,163,184,0.10),transparent_18rem)]">
      <div className="mx-auto flex w-full max-w-6xl flex-col gap-6 px-6 py-8">
        <section className="rounded-[2rem] border border-border/70 bg-background/92 p-8 shadow-[0_20px_80px_-52px_rgba(15,23,42,0.6)] backdrop-blur">
          <p className="text-xs font-semibold uppercase tracking-[0.24em] text-emerald-700">
            Deep-linked command page
          </p>
          <h1 className="mt-3 text-3xl font-semibold tracking-tight">
            Prefilled operation execution
          </h1>
          <p className="mt-3 max-w-3xl text-sm leading-6 text-muted-foreground">
            This route is where the example runtime sends <code>_clicky</code>,{" "}
            <code>_self</code>, <code>_window</code>, and <code>_tab</code> command
            links. Query params become <code>initialValues</code>, and{" "}
            <code>autoRun</code> triggers immediate execution when the request is fully
            specified.
          </p>
        </section>

        <section className="rounded-[2rem] border border-border/70 bg-background/96 p-6 shadow-[0_16px_64px_-44px_rgba(15,23,42,0.5)] backdrop-blur">
          <OperationCommandPage
            client={apiClient}
            operationId={operationId}
            initialValues={initialValues}
            autoRun={autoRun}
            backHref="/links"
            backLabel="Back to Link Examples"
            renderLink={renderLink}
          />
        </section>
      </div>
    </div>
  );
}

function buildExampleCommandHref(resolved: ClickyResolvedCommand): string | undefined {
  const operationId = resolved.operation?.operation.operationId;
  if (!operationId) {
    return undefined;
  }

  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(commandInitialValues(resolved))) {
    if (value !== "") {
      params.set(key, value);
    }
  }
  if (resolved.request.autoRun) {
    params.set("autoRun", "1");
  }

  const query = params.toString();
  return query
    ? `/links/commands/${encodeURIComponent(operationId)}?${query}`
    : `/links/commands/${encodeURIComponent(operationId)}`;
}

function commandInitialValues(
  resolved: ClickyResolvedCommand,
): Record<string, string> {
  const values: Record<string, string> = { ...(resolved.request.flags ?? {}) };
  const args = [...(resolved.request.args ?? [])];
  const positionalParams = (resolved.operation?.operation.parameters ?? []).filter((param) =>
    isDemoPositionalParam(param),
  );
  let argIndex = 0;

  for (const param of positionalParams) {
    if (param.name === "args") {
      const remaining = args.slice(argIndex);
      if (remaining.length > 0) {
        values.args = remaining.join(",");
      }
      argIndex = args.length;
      continue;
    }

    if (argIndex < args.length) {
      values[param.name] = args[argIndex] ?? "";
      argIndex += 1;
    }
  }

  return values;
}

function isDemoPositionalParam(param: OpenAPIParameter) {
  return isPositionalParam(param);
}

function InfoCard({
  eyebrow,
  title,
  body,
}: {
  eyebrow: string;
  title: string;
  body: string;
}) {
  return (
    <section className="rounded-[1.75rem] border border-border/70 bg-background/88 p-5 shadow-[0_16px_56px_-44px_rgba(15,23,42,0.55)] backdrop-blur">
      <p className="text-[11px] font-semibold uppercase tracking-[0.22em] text-muted-foreground">
        {eyebrow}
      </p>
      <h2 className="mt-3 text-lg font-semibold tracking-tight">{title}</h2>
      <p className="mt-2 text-sm leading-6 text-muted-foreground">{body}</p>
    </section>
  );
}

function ExampleNotice({
  title,
  body,
  tone = "default",
}: {
  title: string;
  body: string;
  tone?: "default" | "danger";
}) {
  return (
    <div
      className={[
        "rounded-[1.5rem] border p-5",
        tone === "danger"
          ? "border-red-500/30 bg-red-500/5 text-red-700"
          : "border-border/70 bg-muted/35 text-foreground",
      ].join(" ")}
    >
      <div className="text-sm font-semibold">{title}</div>
      <div className="mt-2 text-sm text-muted-foreground">{body}</div>
    </div>
  );
}
