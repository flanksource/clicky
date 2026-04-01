const { useState, useCallback, useMemo } = React;

function sanitizeHTML(html) {
  const doc = new DOMParser().parseFromString(html, 'text/html');
  doc.querySelectorAll('script,iframe,object,embed,form').forEach(el => el.remove());
  doc.querySelectorAll('*').forEach(el => {
    for (const attr of [...el.attributes]) {
      if (attr.name.startsWith('on')) el.removeAttribute(attr.name);
    }
  });
  return doc.body.innerHTML;
}

function SafeHTML({ html, className }) {
  return <span className={className} dangerouslySetInnerHTML={{ __html: sanitizeHTML(html) }} />;
}

function prettifyName(name) {
  if (!name) return "";
  return name
    .replace(/([a-z])([A-Z])/g, "$1 $2")
    .replace(/_/g, " ")
    .replace(/\b\w/g, (c) => c.toUpperCase());
}

function Card({ title, children }) {
  return (
    <div className="bg-white rounded-lg shadow">
      {title && (
        <div className="px-6 py-4 border-b border-gray-200">
          <h2 className="text-xl font-semibold text-gray-900">{prettifyName(title)}</h2>
        </div>
      )}
      <div className="px-6 py-4">{children}</div>
    </div>
  );
}

function DataRenderer({ data }) {
  if (!data) return <div className="text-gray-400">No data</div>;

  switch (data.type) {
    case "table":
      return <TableView table={data.table} />;
    case "tree":
      return <TreeView node={data.tree} />;
    case "map":
      return <MapView fields={data.fields} schema={data.schema} />;
    case "list":
      return <ListView list={data.list} />;
    case "text":
      if (data.html) {
        return <SafeHTML html={data.html} />;
      }
      return <span>{data.text}</span>;
    default:
      return (
        <pre className="bg-gray-50 p-4 rounded text-sm overflow-auto">
          {JSON.stringify(data, null, 2)}
        </pre>
      );
  }
}

function Cell({ cell }) {
  if (!cell) return null;
  if (typeof cell === "string") return <span>{cell}</span>;
  if (cell.html) return <SafeHTML html={cell.html} />;
  return <span>{cell.text}</span>;
}

function TableView({ table, title }) {
  if (!table || !table.rows || table.rows.length === 0) {
    return <div className="text-gray-400 text-sm">No data</div>;
  }

  const columns = table.columns || [];
  const colLabels = columns.map((c) => prettifyName(c.label || c.name));
  const colNames = columns.map((c) => c.name);

  const [sortCol, setSortCol] = useState(null);
  const [sortAsc, setSortAsc] = useState(true);

  const sortedRows = useMemo(() => {
    if (sortCol === null) return table.rows;
    const name = colNames[sortCol];
    return [...table.rows].sort((a, b) => {
      const av = (a[name]?.text || a[name]?.html || "").toLowerCase();
      const bv = (b[name]?.text || b[name]?.html || "").toLowerCase();
      const cmp = av < bv ? -1 : av > bv ? 1 : 0;
      return sortAsc ? cmp : -cmp;
    });
  }, [table.rows, sortCol, sortAsc]);

  const handleSort = (i) => {
    if (sortCol === i) setSortAsc(!sortAsc);
    else { setSortCol(i); setSortAsc(true); }
  };

  const inner = (
    <table className="w-full border-collapse text-sm">
      <thead>
        <tr>
          {colLabels.map((label, i) => (
            <th key={i}
                onClick={() => handleSort(i)}
                className="font-medium text-left text-slate-900 border-b border-gray-200 py-2 px-2 cursor-pointer hover:bg-gray-50 select-none">
              {label}
              {sortCol === i && <span className="ml-1 text-gray-400">{sortAsc ? "↑" : "↓"}</span>}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {sortedRows.map((row, ri) => (
          <tr key={ri} className="border-b border-gray-200 last:border-b-0 hover:bg-gray-50">
            {colNames.map((name, ci) => (
              <td key={ci} className="py-1.5 px-2 text-gray-700 align-top">
                <Cell cell={row[name]} />
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  );

  return title ? <Card title={title}>{inner}</Card> : <div className="my-4">{inner}</div>;
}

function TreeNode({ node, depth = 0, overrideExpand }) {
  const [localExpanded, setLocalExpanded] = useState(depth < 2);
  const expanded = overrideExpand !== undefined ? overrideExpand : localExpanded;
  const hasChildren = node.children && node.children.length > 0;

  return (
    <div style={{ marginLeft: depth > 0 ? 16 : 0 }}>
      <div
        className="flex items-center gap-1 py-0.5 cursor-pointer hover:bg-gray-50 rounded px-1"
        onClick={() => hasChildren && setLocalExpanded(!expanded)}
      >
        {hasChildren && (
          <span className="text-gray-400 text-xs w-4">
            {expanded ? "▼" : "▶"}
          </span>
        )}
        {!hasChildren && <span className="w-4" />}
        {node.html ? (
          <SafeHTML html={node.html} />
        ) : (
          <span className="text-sm">{node.label}</span>
        )}
      </div>
      {expanded &&
        hasChildren &&
        node.children.map((child, i) => (
          <TreeNode key={i} node={child} depth={depth + 1} overrideExpand={overrideExpand} />
        ))}
    </div>
  );
}

function countNodes(node) {
  if (!node) return 0;
  let n = 1;
  if (node.children) {
    for (const c of node.children) n += countNodes(c);
  }
  return n;
}

function TreeView({ node }) {
  if (!node) return null;
  const [overrideExpand, setOverrideExpand] = useState(undefined);
  const hasChildren = countNodes(node) > 1;

  const expandAll = useCallback(() => setOverrideExpand(true), []);
  const collapseAll = useCallback(() => setOverrideExpand(false), []);
  const reset = useCallback(() => setOverrideExpand(undefined), []);

  return (
    <div className="font-mono text-sm">
      {hasChildren && (
        <div className="flex gap-2 mb-2 font-sans text-xs">
          <button
            onClick={expandAll}
            className="px-2 py-0.5 rounded border border-gray-300 text-gray-600 hover:bg-gray-100"
          >
            Expand All
          </button>
          <button
            onClick={collapseAll}
            className="px-2 py-0.5 rounded border border-gray-300 text-gray-600 hover:bg-gray-100"
          >
            Collapse All
          </button>
          {overrideExpand !== undefined && (
            <button
              onClick={reset}
              className="px-2 py-0.5 rounded border border-gray-300 text-gray-500 hover:bg-gray-100"
            >
              Reset
            </button>
          )}
        </div>
      )}
      <TreeNode node={node} overrideExpand={overrideExpand} />
    </div>
  );
}

function MapView({ fields, schema }) {
  if (!fields) return null;

  const kvPairs = [];
  const tableSections = [];
  const treeSections = [];
  const listSections = [];

  const fieldOrder = schema
    ? schema.map((s) => s.name)
    : Object.keys(fields);

  for (const key of fieldOrder) {
    const value = fields[key];
    if (!value) continue;
    if (value.type === "text") {
      kvPairs.push({ label: key, value: value.html || value.text || "" });
    } else if (value.type === "table") {
      tableSections.push({ key, value });
    } else if (value.type === "tree") {
      treeSections.push({ key, value });
    } else {
      listSections.push({ key, value });
    }
  }

  return (
    <div className="space-y-6">
      {kvPairs.length > 0 && (
        <Card title="Summary">
          <dl className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {kvPairs.map((kv, i) => (
              <div key={i}>
                <dt className="text-sm font-medium text-gray-500">{prettifyName(kv.label)}</dt>
                <SafeHTML html={kv.value} className="mt-1 text-sm" />
              </div>
            ))}
          </dl>
        </Card>
      )}
      {tableSections.map(({ key, value }) => (
        <TableView key={key} table={value.table} title={key} />
      ))}
      {treeSections.map(({ key, value }) => (
        <Card key={key} title={key}>
          <TreeView node={value.tree} />
        </Card>
      ))}
      {listSections.map(({ key, value }) => (
        <Card key={key} title={key}>
          <DataRenderer data={value} />
        </Card>
      ))}
    </div>
  );
}

function ListView({ list }) {
  if (!list || list.length === 0) return null;
  return (
    <div className="space-y-4">
      {list.map((item, i) => (
        <DataRenderer key={i} data={item} />
      ))}
    </div>
  );
}

function App({ data }) {
  return (
    <div className="bg-gray-100 min-h-screen p-6">
      <div className="max-w-6xl mx-auto space-y-6">
        <DataRenderer data={data} />
      </div>
    </div>
  );
}
