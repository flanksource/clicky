import { Link, Navigate, Route, Routes, useParams } from "react-router-dom";
import {
  OperationCatalog,
  OperationCommandPage,
  ThemeSwitcher,
  type RenderLink,
} from "@flanksource/clicky-ui";
import { apiClient } from "./api";
import { domainOrder, domains } from "./domains";

const renderLink: RenderLink = ({ to, className, children, title, key }) => (
  <Link key={key} to={to} className={className} title={title}>
    {children}
  </Link>
);

function DomainPage() {
  const { domainKey } = useParams<{ domainKey: string }>();
  const spec = domainKey ? domains[domainKey] : undefined;
  if (!spec) {
    return (
      <div className="p-6 text-sm text-muted-foreground">
        Unknown domain: <code>{domainKey}</code>
      </div>
    );
  }
  return (
    <OperationCatalog
      definition={spec.definition}
      entities={spec.entities}
      allOperations={spec.allOperations}
      operationIdPrefix={spec.operationIdPrefix}
      listOperationId={spec.listOperationId}
      detailOperationId={spec.detailOperationId}
      client={apiClient}
      renderLink={renderLink}
    />
  );
}

function CommandRoute() {
  const { operationId } = useParams<{ operationId: string }>();
  return (
    <OperationCommandPage
      client={apiClient}
      operationId={operationId}
      backHref="/explorer"
      backLabel="Back to API Explorer"
      renderLink={renderLink}
    />
  );
}

function Sidebar() {
  return (
    <aside className="flex w-56 shrink-0 flex-col border-r border-border bg-muted/30 p-4">
      <div className="mb-6">
        <div className="text-sm font-semibold">Entity Demo</div>
        <div className="text-xs text-muted-foreground">Clicky RPC + UI</div>
      </div>
      <nav className="flex flex-col gap-1">
        {domainOrder.map((key) => {
          const spec = domains[key];
          return (
            <Link
              key={key}
              to={`/${key}`}
              className="rounded-md px-2 py-1.5 text-sm text-foreground hover:bg-accent"
            >
              {spec.definition.title}
            </Link>
          );
        })}
      </nav>
      <div className="mt-auto pt-4">
        <ThemeSwitcher />
      </div>
    </aside>
  );
}

export function App() {
  return (
    <div className="flex h-full">
      <Sidebar />
      <main className="flex-1 overflow-auto p-6">
        <Routes>
          <Route path="/" element={<Navigate to="/stacks" replace />} />
          <Route path="/commands/:operationId" element={<CommandRoute />} />
          <Route path="/:domainKey" element={<DomainPage />} />
        </Routes>
      </main>
    </div>
  );
}
