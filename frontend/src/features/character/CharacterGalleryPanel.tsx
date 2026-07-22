import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { useCharacters, useExtractCharacters, useIngestCharacters, roleColor, roleLabel } from './api';

export function CharacterGalleryPanel({ projectId }: { projectId: string }) {
  const q = useCharacters(projectId);
  const extract = useExtractCharacters(projectId);
  const ingest = useIngestCharacters(projectId);
  const [poll, setPoll] = useState(false);

  useEffect(() => {
    if (!poll) return;
    const t = setInterval(() => q.refetch(), 4000);
    return () => clearInterval(t);
  }, [poll, q]);

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <button onClick={() => extract.mutate()}
          disabled={extract.isPending}
          className="px-3 py-1.5 rounded bg-slate-900 text-white disabled:opacity-50">
          {extract.isPending ? '排队中…' : '触发角色提取'}
        </button>
        <button onClick={() => ingest.mutate()}
          disabled={ingest.isPending}
          className="px-3 py-1.5 rounded border disabled:opacity-50">
          {ingest.isPending ? '拉取中…' : '拉取结果'}
        </button>
        <button onClick={() => setPoll((p) => !p)} className="px-3 py-1.5 rounded border text-sm">
          {poll ? '停止轮询' : '开始轮询'}
        </button>
        {q.data && <span className="text-sm text-slate-500">共 {q.data.length} 人</span>}
      </div>

      {(extract.error || ingest.error) && (
        <p className="text-sm text-red-600">{String((extract.error || ingest.error) as Error).message}</p>
      )}

      {q.data && q.data.length === 0 && (
        <div className="border rounded p-4 text-sm text-slate-500">
          还没有角色。先确保章节已切分，然后点击「触发角色提取」。
        </div>
      )}

      <ul className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-3">
        {q.data?.map((c) => {
          const url = c.meta?.ref_image_url;
          return (
            <li key={c.id} className="border rounded bg-white overflow-hidden">
              <Link to={`/projects/${projectId}/characters/${c.id}`} className="block">
                {url ? (
                  <img src={url} alt={c.name} className="aspect-square w-full object-cover bg-slate-100" />
                ) : (
                  <div className="aspect-square w-full bg-slate-100 flex items-center justify-center text-slate-400 text-sm">
                    无图
                  </div>
                )}
                <div className="p-2 space-y-1">
                  <div className="flex items-center justify-between">
                    <span className="font-medium truncate">{c.name}</span>
                    <span className={`text-[10px] px-1.5 py-0.5 rounded ${roleColor(c.role)}`}>
                      {roleLabel(c.role)}
                    </span>
                  </div>
                  <p className="text-xs text-slate-500 line-clamp-2">{c.appearance || c.personality || '—'}</p>
                </div>
              </Link>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
