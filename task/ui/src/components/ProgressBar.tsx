interface Segment {
  count: number;
  color: string;
  label: string;
}

interface Props {
  segments: Segment[];
  total: number;
  height?: string;
}

export function ProgressBar({ segments, total, height = 'h-2' }: Props) {
  if (total === 0) return null;

  const tooltip = segments
    .filter(s => s.count > 0)
    .map(s => `${s.count} ${s.label}`)
    .join(', ');

  return (
    <div class={`w-full bg-gray-200 rounded-full ${height} flex overflow-hidden`} title={tooltip}>
      {segments.map((seg, i) => {
        if (seg.count === 0) return null;
        const pct = (seg.count / total) * 100;
        return (
          <div
            key={i}
            class={`${seg.color} ${height} transition-all duration-300`}
            style={{ width: `${pct}%` }}
            title={`${seg.count} ${seg.label}`}
          />
        );
      })}
    </div>
  );
}

export function taskSegments(counts: { ok: number; warn: number; fail: number; run: number; pending: number }): Segment[] {
  return [
    { count: counts.ok, color: 'bg-green-500', label: 'passed' },
    { count: counts.warn, color: 'bg-yellow-400', label: 'warnings' },
    { count: counts.fail, color: 'bg-red-500', label: 'failed' },
    { count: counts.run, color: 'bg-blue-500', label: 'running' },
    { count: counts.pending, color: 'bg-gray-300', label: 'pending' },
  ];
}
