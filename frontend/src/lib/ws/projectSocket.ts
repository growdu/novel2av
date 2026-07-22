export type WsEvent = {
  type: 'job.progress' | 'job.log' | 'chapter.ready' | 'job.failed' | string;
  job_id?: string;
  chapter_id?: string;
  shot_id?: string;
  step?: string;
  current?: number;
  total?: number;
  status?: string;
  message?: string;
  level?: 'info' | 'warn' | 'error';
};

// Single WebSocket per project with exponential reconnect.
export function openProjectSocket(
  projectId: string,
  onEvent: (e: WsEvent) => void,
): () => void {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  const url = `${proto}://${location.host}/api/v1/ws/projects/${projectId}`;
  let ws: WebSocket | null = null;
  let closed = false;
  let delay = 1000;

  const connect = () => {
    ws = new WebSocket(url);
    ws.onmessage = (ev) => {
      try { onEvent(JSON.parse(ev.data) as WsEvent); } catch { /* ignore */ }
      delay = 1000;
    };
    ws.onclose = () => {
      if (closed) return;
      setTimeout(connect, delay);
      delay = Math.min(delay * 2, 15000);
    };
    ws.onerror = () => ws?.close();
  };
  connect();

  return () => { closed = true; ws?.close(); };
}
