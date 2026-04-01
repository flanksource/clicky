import type { TaskSnapshot } from '../types';
import { statusColor, statusIcon, logLevelColor } from '../utils';

interface Props {
  task: TaskSnapshot;
  expanded: boolean;
  onToggle: () => void;
}

export function TaskRow({ task: t, expanded, onToggle }: Props) {
  const logs = t.logs || [];
  const hasLogs = logs.length > 0;

  return (
    <div
      class={`flex items-start gap-3 py-2 border-b border-gray-100 last:border-0${hasLogs ? ' cursor-pointer hover:bg-gray-50 rounded -mx-1 px-1' : ''}`}
      onClick={hasLogs ? onToggle : undefined}
    >
      <iconify-icon icon={statusIcon(t.status)} class={`${statusColor(t.status)} text-lg mt-0.5`} />
      <div class="flex-1 min-w-0">
        <div class="flex items-center justify-between gap-2">
          <span class="font-medium text-sm flex items-center gap-1.5 min-w-0">
            <span class="truncate">{t.name}</span>
            {t.error && <span class="text-xs text-red-500 font-normal truncate">{t.error}</span>}
            {hasLogs && (
              <span class="inline-flex items-center px-1.5 py-0 rounded-full text-[10px] font-medium bg-gray-100 text-gray-500 shrink-0">
                {logs.length}
              </span>
            )}
          </span>
          {t.duration && <span class="text-xs text-gray-400 shrink-0">{t.duration}</span>}
        </div>
        {expanded && hasLogs && (
          <div class="mt-1 ml-1 pl-2 border-l-2 border-gray-200 space-y-0.5 max-h-48 overflow-y-auto">
            {logs.map((l, i) => (
              <div key={i} class={`text-xs ${logLevelColor(l.level)}`}>
                <span class="font-mono text-gray-300 mr-1">{l.level.substring(0, 3)}</span>
                {l.message}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
