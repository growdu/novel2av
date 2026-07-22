import { useState } from 'react';
import { useChapters, useIngestChapters, useSplitChapters } from './api';
import { Link } from 'react-router-dom';

export function ChapterListPanel({ projectId }: { projectId: string }) {
  const q = useChapters(projectId);
  const split = useSplitChapters(projectId);
  const ingest = useIngestChapters(projectId);

  const [poll, setPoll] = useState(false);

  // Optionally poll after triggering split, since the WS handler is still TBD.
  useChaptersWithPoll(projectId, poll && split.isSuccess);

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <button
          onClick={() => split.mutate()}
          disabled={split.isPending}
          className="px-3 py-1.5 rounded bg-slate-900 text-white disabled:opacity-50"
        >
          {split.isPending ? '排队中…' : '触发章节切分'}
        </button>
        <button
          onClick={() => ingest.mutate()}
          disabled={ingest.isPending}
          className="px-3 py-1.5 rounded border disabled:opacity-50"
        >
          {ingest.isPending ? '拉取中…' : '拉取结果'}
        </button>
        <button
          onClick={() => setPoll((p) => !p)}
          className="px-3 py-1.5 rounded border text-sm"
        >
          {poll ? '停止轮询' : '开始轮询'}
        </button>
        {q.data && <span className="text-sm text-slate-500">共 {q.data.length} 章</span>}
      </div>

      {split.error && <p className="text-sm text-red-600">{(split.error as Error).message}</p>}
      {ingest.error && <p className="text-sm text-red-600">{(ingest.error as Error).message}</p>}

      {q.data && q.data.length === 0 && (
        <div className="border rounded p-4 text-sm text-slate-500">
          还没有章节。点击「触发章节切分」开始（LLM/规则在 ai-engine 内运行）。
        </div>
      )}

      <ol className="space-y-1">
        {q.data?.map((c) => (
          <li key={c.id} className="flex items-center justify-between border rounded bg-white px-3 py-2">
            <div className="flex items-center gap-3">
              <span className="text-slate-400 w-8 text-right">{c.index}</span>
              <Link to={`/projects/${projectId}/chapters/${c.index}`} className="hover:underline">
                {c.title}
              </Link>
              <span className="text-xs text-slate-500">{c.word_count} 字</span>
            </div>
            <span className="text-xs px-2 py-0.5 rounded bg-slate-100 text-slate-700">{c.status}</span>
          </li>
        ))}
      </ol>
    </div>
  );
}

// Tiny in-file polling helper; replaced by WS push in M4.
import { useQueryClient } from '@tanstack/react-query';
function useChaptersWithPoll(projectId: string, on: boolean) {
  const qc = useQueryClient();
  if (typeof window === 'undefined') return;
  // run in effect
  useEffectPoll(() => {
    if (!on) return;
    const t = setInterval(() => qc.invalidateQueries({ queryKey: ['chapters', projectId] }), 4000);
    return () => clearInterval(t);
  });
}
import { useEffect as useEffectPoll } from 'react';
