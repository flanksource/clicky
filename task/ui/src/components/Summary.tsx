import type { TaskSnapshot } from '../types';
import { ProgressBar, taskSegments } from './ProgressBar';

interface Props {
  tasks: Record<string, TaskSnapshot>;
  startTime: number | null;
}

export function Summary({ tasks, startTime }: Props) {
  const all = Object.values(tasks);
  const total = all.length;
  const ok = all.filter(t => t.status === 'success' || t.status === 'PASS').length;
  const warn = all.filter(t => t.status === 'warning').length;
  const fail = all.filter(t => t.status === 'failed' || t.status === 'FAIL' || t.status === 'ERR').length;
  const run = all.filter(t => t.status === 'running').length;
  const pending = total - ok - warn - fail - run;
  const elapsed = startTime ? ((Date.now() - startTime) / 1000).toFixed(1) + 's' : '—';

  return (
    <div class="flex flex-col items-end gap-1">
      <div class="flex gap-3 text-sm text-gray-500 items-center">
        <span class="font-medium text-gray-700">{total} tasks</span>
        {ok > 0 && <><Sep /><span class="text-green-600">{ok} passed</span></>}
        {warn > 0 && <><Sep /><span class="text-yellow-600">{warn} warnings</span></>}
        {fail > 0 && <><Sep /><span class="text-red-600">{fail} failed</span></>}
        {run > 0 && <><Sep /><span class="text-blue-600">{run} running</span></>}
        <Sep />
        <span class="text-gray-400">
          <iconify-icon icon="codicon:clock" class="mr-0.5" />
          {elapsed}
        </span>
      </div>
      {total > 0 && (
        <div class="w-64">
          <ProgressBar segments={taskSegments({ ok, warn, fail, run, pending })} total={total} />
        </div>
      )}
    </div>
  );
}

function Sep() {
  return <span class="text-gray-300">|</span>;
}
