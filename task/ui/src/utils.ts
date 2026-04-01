const STATUS_COLORS: Record<string, string> = {
  success: 'text-green-600', PASS: 'text-green-600', completed: 'text-green-600',
  failed: 'text-red-600', FAIL: 'text-red-600', ERR: 'text-red-600',
  warning: 'text-yellow-600',
  running: 'text-blue-600',
  pending: 'text-gray-400', canceled: 'text-gray-400', SKIP: 'text-gray-400',
};

const STATUS_ICONS: Record<string, string> = {
  pending: 'codicon:clock',
  running: 'svg-spinners:ring-resize',
  success: 'codicon:pass-filled', PASS: 'codicon:pass-filled', completed: 'codicon:pass-filled',
  failed: 'codicon:error', FAIL: 'codicon:error', ERR: 'codicon:error',
  warning: 'codicon:warning',
  canceled: 'codicon:circle-slash', SKIP: 'codicon:circle-slash',
};

const STATUS_BG: Record<string, string> = {
  success: 'bg-green-100', completed: 'bg-green-100', PASS: 'bg-green-100',
  failed: 'bg-red-100', FAIL: 'bg-red-100', ERR: 'bg-red-100',
  warning: 'bg-yellow-100',
  running: 'bg-blue-100',
  pending: 'bg-gray-100',
};

export function statusColor(s: string): string {
  return STATUS_COLORS[s] || 'text-gray-500';
}

export function statusIcon(s: string): string {
  return STATUS_ICONS[s] || 'codicon:circle-outline';
}

export function statusBg(s: string): string {
  return STATUS_BG[s] || 'bg-gray-100';
}

export function logLevelColor(level: string): string {
  switch (level) {
    case 'error': return 'text-red-500';
    case 'warn': return 'text-yellow-600';
    case 'debug': return 'text-gray-400';
    default: return 'text-gray-500';
  }
}
