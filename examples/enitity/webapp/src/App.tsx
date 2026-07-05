import { useMemo } from "react";
import {
  Link,
  Routes,
  Route,
  useLocation,
  useNavigate,
} from "react-router-dom";
import {
  DensitySwitcher,
  EntityExplorerApp,
  Icon,
  RouterProvider,
  ThemeSwitcher,
  type RouterAdapter,
} from "@flanksource/clicky-ui";
import {
  UiLayoutDashboard,
  UiLink,
  UiMarkdown,
} from "@flanksource/clicky-ui/icons";
import { apiClient } from "./api";
import { ChatWidget } from "./ChatWidget";
import { LinkExampleCommandPage, LinkExamplesPage } from "./LinkExamplesPage";
import { MarkdownEditorPage } from "./MarkdownEditorPage";

export function App() {
  const location = useLocation();
  const navigate = useNavigate();
  const activeSection = location.pathname.startsWith("/links")
    ? "links"
    : location.pathname.startsWith("/markdown")
      ? "markdown"
      : "explorer";

  // Bridge react-router into clicky-ui's RouterAdapter: <Link> for rendering,
  // useLocation for active state, useNavigate for imperative navigation.
  const adapter = useMemo<RouterAdapter>(
    () => ({
      pathname: location.pathname,
      renderLink: ({ to, className, children, title, key }) => (
        <Link key={key} to={to} className={className} title={title}>
          {children}
        </Link>
      ),
      navigate: (to, opts) => navigate(to, opts),
    }),
    [location.pathname, navigate],
  );

  return (
    <div className="flex h-full min-h-0 flex-col bg-background">
      <header className="border-b border-border/70 bg-background/90 backdrop-blur">
        <div className="mx-auto flex w-full max-w-7xl items-center justify-between gap-4 px-6 py-3">
          <div>
            <div className="text-sm font-semibold tracking-tight">
              Clicky Entity Example
            </div>
            <div className="text-xs text-muted-foreground">
              Metadata-driven explorer plus interactive Link and LinkCommand
              demos
            </div>
          </div>
          <div className="flex items-center gap-3">
            <nav
              aria-label="Top sections"
              className="flex items-center gap-1 rounded-full border border-border/70 bg-muted/40 p-1"
            >
              <Link
                to="/stacks"
                className={topNavLinkClass(activeSection === "explorer")}
                aria-current={activeSection === "explorer" ? "page" : undefined}
              >
                <Icon
                  icon={UiLayoutDashboard}
                  className="text-muted-foreground"
                />
                Explorer
              </Link>
              <Link
                to="/links"
                className={topNavLinkClass(activeSection === "links")}
                aria-current={activeSection === "links" ? "page" : undefined}
              >
                <Icon icon={UiLink} className="text-muted-foreground" />
                Link Examples
              </Link>
              <Link
                to="/markdown"
                className={topNavLinkClass(activeSection === "markdown")}
                aria-current={activeSection === "markdown" ? "page" : undefined}
              >
                <Icon icon={UiMarkdown} className="text-muted-foreground" />
                Markdown
              </Link>
            </nav>
            <div className="flex items-center gap-1 rounded-full border border-border/70 bg-muted/40 p-1">
              {activeSection !== "explorer" ? <ThemeSwitcher /> : null}
              <DensitySwitcher />
            </div>
          </div>
        </div>
      </header>

      <div className="min-h-0 flex-1">
        <Routes>
          <Route path="/links" element={<LinkExamplesPage />} />
          <Route
            path="/links/commands/:operationId"
            element={<LinkExampleCommandPage />}
          />
          <Route path="/markdown" element={<MarkdownEditorPage />} />
          <Route
            path="*"
            element={
              <RouterProvider adapter={adapter}>
                <EntityExplorerApp client={apiClient} />
              </RouterProvider>
            }
          />
        </Routes>
      </div>

      <ChatWidget client={apiClient} />
    </div>
  );
}

function topNavLinkClass(active: boolean) {
  return [
    "inline-flex items-center gap-1.5 rounded-full px-3 py-1.5 text-sm font-medium transition-colors",
    active
      ? "bg-foreground text-background"
      : "text-foreground hover:bg-accent hover:text-accent-foreground",
  ].join(" ");
}
