import { useState } from 'preact/hooks';
import type { TaskSnapshot } from '../types';
import { statusColor, statusIcon, statusBg } from '../utils';
import { TaskRow } from './TaskRow';
import { ProgressBar, taskSegments } from './ProgressBar';

interface Props {
  group: TaskSnapshot;
  tasks: TaskSnapshot[];
  expanded: Record<string, boolean>;
  onToggle: (id: string) => void;
}

const MAX_COMPLETED = 5;
const MAX_PENDING = 3;

function isFailed(t: TaskSnapshot): boolean {
  return t.status === 'failed' || t.status === 'FAIL' || t.status === 'ERR' || t.status === 'warning';
}

export function TaskGroup({ group: g, tasks, expanded, onToggle }: Props) {
  const [showAll, setShowAll] = useState(false);
  const done = g.total! - (g.running || 0) - tasks.filter(t => t.status === 'pending').length;
  const progress = g.total! > 0 ? `${done}/${g.total}` : '';

  const running = tasks.filter(t => t.status === 'running');
  const pending = tasks.filter(t => t.status === 'pending');
  const completed = tasks.filter(t => t.status !== 'running' && t.status !== 'pending');
  const okCount = tasks.filter(t => t.status === 'success' || t.status === 'PASS').length;
  const warnCount = tasks.filter(t => t.status === 'warning').length;
  const failCount = tasks.filter(t => t.status === 'failed' || t.status === 'FAIL' || t.status === 'ERR').length;

  // Split completed into always-visible (failures/warnings) and collapsible (success)
  const alwaysShow = completed.filter(isFailed);
  const collapsible = completed.filter(t => !isFailed(t));

  const hiddenCompleted = collapsible.length - MAX_COMPLETED;
  const collapseCompleted = !showAll && hiddenCompleted > 0;
  const visibleSuccess = collapseCompleted ? collapsible.slice(-MAX_COMPLETED) : collapsible;

  const hiddenPending = pending.length - MAX_PENDING;
  const collapsePending = !showAll && hiddenPending > 0;
  const visiblePending = collapsePending ? pending.slice(0, MAX_PENDING) : pending;

  const totalHidden = (collapseCompleted ? hiddenCompleted : 0) + (collapsePending ? hiddenPending : 0);

  return (
    <div class="bg-white rounded-lg shadow p-4">
      <div class="flex items-center justify-between mb-3">
        <h2 class="font-semibold text-gray-900">
          <iconify-icon icon={statusIcon(g.status)} class={`${statusColor(g.status)} mr-1`} />
          {g.name} {progress && <span class="text-xs text-gray-400">{progress}</span>}
        </h2>
        {g.status !== 'running' && g.status !== 'pending' && (
          <span class={`text-xs px-2 py-0.5 rounded-full ${statusColor(g.status)} ${statusBg(g.status)}`}>
            {g.status}
          </span>
        )}
      </div>
      {g.total! > 0 && (
        <div class="mb-3">
          <ProgressBar
            segments={taskSegments({ ok: okCount, warn: warnCount, fail: failCount, run: running.length, pending: pending.length })}
            total={g.total!}
            height="h-1.5"
          />
        </div>
      )}

      {!showAll && totalHidden > 0 && (
        <div
          class="text-xs text-gray-400 py-1.5 cursor-pointer hover:text-gray-600 border-b border-gray-100"
          onClick={() => setShowAll(true)}
        >
          ... {totalHidden} more tasks
        </div>
      )}
      {showAll && totalHidden > 0 && (
        <div
          class="text-xs text-gray-400 py-1.5 cursor-pointer hover:text-gray-600 border-b border-gray-100"
          onClick={() => setShowAll(false)}
        >
          ▲ collapse
        </div>
      )}

      {visibleSuccess.map(t => (
        <TaskRow key={t.id} task={t} expanded={!!expanded[t.id]} onToggle={() => onToggle(t.id)} />
      ))}
      {alwaysShow.map(t => (
        <TaskRow key={t.id} task={t} expanded={!!expanded[t.id]} onToggle={() => onToggle(t.id)} />
      ))}
      {running.map(t => (
        <TaskRow key={t.id} task={t} expanded={!!expanded[t.id]} onToggle={() => onToggle(t.id)} />
      ))}
      {visiblePending.map(t => (
        <TaskRow key={t.id} task={t} expanded={!!expanded[t.id]} onToggle={() => onToggle(t.id)} />
      ))}
      {collapsePending && (
        <div
          class="text-xs text-gray-400 py-1.5 cursor-pointer hover:text-gray-600"
          onClick={() => setShowAll(true)}
        >
          ... {hiddenPending} more pending
        </div>
      )}
    </div>
  );
}
