import { Link, Routes, Route, useLocation } from "react-router-dom";
import { EntityExplorerApp } from "@flanksource/clicky-ui/api-explorer";
import {
  DensitySwitcher,
  EntityExplorerApp,
  Icon,
  ThemeSwitcher,
  type RenderLink,
} from "@flanksource/clicky-ui";
import { apiClient } from "./api";
import { LinkExampleCommandPage, LinkExamplesPage } from "./LinkExamplesPage";

const renderLink: RenderLink = ({ to, className, children, title, key }) => (
  <Link key={key} to={to} className={className} title={title}>
    {children}
  </Link>
);

export function App() {
  const location = useLocation();
  const showingLinks = location.pathname.startsWith("/links");

  return (
    <div className="flex h-full min-h-0 flex-col bg-background">
      <header className="border-b border-border/70 bg-background/90 backdrop-blur">
        <div className="mx-auto flex w-full max-w-7xl items-center justify-between gap-4 px-6 py-3">
          <div>
            <div className="text-sm font-semibold tracking-tight">Clicky Entity Example</div>
            <div className="text-xs text-muted-foreground">
              Metadata-driven explorer plus interactive Link and LinkCommand demos
            </div>
          </div>
          <div className="flex items-center gap-3">
            <nav
              aria-label="Top sections"
              className="flex items-center gap-1 rounded-full border border-border/70 bg-muted/40 p-1"
            >
              <Link
                to="/stacks"
                className={topNavLinkClass(!showingLinks)}
                aria-current={!showingLinks ? "page" : undefined}
              >
                <Icon name="ph:squares-four" className="text-muted-foreground" />
                Explorer
              </Link>
              <Link
                to="/links"
                className={topNavLinkClass(showingLinks)}
                aria-current={showingLinks ? "page" : undefined}
              >
                <Icon name="ph:link" className="text-muted-foreground" />
                Link Examples
              </Link>
            </nav>
            <div className="flex items-center gap-1 rounded-full border border-border/70 bg-muted/40 p-1">
              {showingLinks ? <ThemeSwitcher /> : null}
              <DensitySwitcher />
            </div>
          </div>
        </div>
      </header>

      <div className="min-h-0 flex-1">
        <Routes>
          <Route path="/links" element={<LinkExamplesPage />} />
          <Route path="/links/commands/:operationId" element={<LinkExampleCommandPage />} />
          <Route
            path="*"
            element={
              <EntityExplorerApp
                client={apiClient}
                pathname={location.pathname}
                renderLink={renderLink}
              />
            }
          />
        </Routes>
      </div>
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
