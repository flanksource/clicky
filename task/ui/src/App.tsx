import { useState, useEffect, useCallback, useRef } from 'preact/hooks';
import type { TaskSnapshot } from './types';
import { Summary } from './components/Summary';
import { TaskGroup } from './components/TaskGroup';

export function App() {
  const [groups, setGroups] = useState<Record<string, TaskSnapshot>>({});
  const [tasks, setTasks] = useState<Record<string, TaskSnapshot>>({});
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const [status, setStatus] = useState('Connecting...');
  const startTime = useRef<number | null>(null);
  const [, forceUpdate] = useState(0);

  useEffect(() => {
    const es = new EventSource('/api/tasks/stream');
    setStatus('Connected — streaming updates');

    es.addEventListener('task', (e: MessageEvent) => {
      if (!startTime.current) startTime.current = Date.now();
      const snap: TaskSnapshot = JSON.parse(e.data);
      if (snap.type === 'group') {
        setGroups(prev => ({ ...prev, [snap.id]: snap }));
      } else if (snap.type === 'task') {
        setTasks(prev => ({ ...prev, [snap.id]: snap }));
      }
    });

    es.addEventListener('done', () => {
      setStatus('All tasks completed');
      es.close();
    });

    es.onerror = () => setStatus('Connection lost — retrying...');

    // Update elapsed time every second
    const timer = setInterval(() => {
      if (startTime.current) forceUpdate(n => n + 1);
    }, 1000);

    return () => { es.close(); clearInterval(timer); };
  }, []);

  const toggleLogs = useCallback((id: string) => {
    setExpanded(prev => ({ ...prev, [id]: !prev[id] }));
  }, []);

  const groupOrder = Object.keys(groups);

  return (
    <div class="bg-gray-100 min-h-screen p-6">
      <div class="max-w-4xl mx-auto space-y-4">
        <div class="flex items-center justify-between">
          <h1 class="text-2xl font-bold text-gray-900">Task Progress</h1>
          <Summary tasks={tasks} startTime={startTime.current} />
        </div>
        <div class="text-sm text-gray-500">{status}</div>
        <div class="space-y-3">
          {groupOrder.map(gid => (
            <TaskGroup
              key={gid}
              group={groups[gid]}
              tasks={Object.values(tasks).filter(t => t.group === gid)}
              expanded={expanded}
              onToggle={toggleLogs}
            />
          ))}
        </div>
      </div>
    </div>
  );
}
