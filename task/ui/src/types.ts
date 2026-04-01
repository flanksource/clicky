export interface LogEntry {
  level: string;
  message: string;
}

export interface TaskSnapshot {
  id: string;
  name: string;
  type: 'task' | 'group';
  group?: string;
  status: string;
  duration?: string;
  error?: string;
  message?: string;
  logs?: LogEntry[];
  total?: number;
  completed?: number;
  failed?: number;
  running?: number;
}
