import { useMemo, useState } from 'react';
import {
  useShots, useTriggerBreakdown, useRegenShotImage, useRegenShotTTS,
} from './api';

export function ShotList({ projectId, aspect }: { projectId: string; aspect?: string }) {
  const q = useShots(projectId);
  const trigger = useTriggerBreakdown(projectId);
  const regenImg = useRegenShotImage('');
  const regenTTS = useRegenShotTTS('');

  const [pending, setPending] = useState<Record<string, boolean>>({});

  const groups = useMemo(() => {
    const map = new Map<string, typeof q.data>();
    (q.data ?? []).forEach((s) => {
      const key = `${s.chapter_id}#${s.scene_idx}`;
      if (!map.has(key)) map.set(key, [] as any);
      map.get(key)!.push(s);
    });
    return Array.from(map.entries());
  }, [q.data]);

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <button onClick={() => trigger.mutate()} disabled={trigger.isPending}
          className="px-3 py-1.5 rounded bg-slate-900 text-white disabled:opacity-50">
          {trigger.isPending ? '排队中…' : '触发场景拆分'}
        </button>
        <button onClick={() => q.refetch()} className="px-3 py-1.5 rounded border text-sm">
          刷新
        </button>
        {q.data && <span className="text-sm text-slate-500">共 {q.data.length} 个镜头</span>}
      </div>

      {q.data && q.data.length === 0 && (
        <div className="border rounded p-4 text-sm text-slate-500">
          还没有分镜。先确保章节与角色都已就绪，然后点击「触发场景拆分」。
        </div>
      )}

      {groups.map(([key, shots]) => (
        <section key={key} className="border rounded bg-white p-3 space-y-2">
          <h3 className="text-sm font-medium text-slate-700">场景 {shots[0].scene_idx}</h3>
          <ul className="grid grid-cols-1 md:grid-cols-2 gap-3">
            {shots.map((s) => (
              <li key={s.id} className="border rounded p-2 space-y-2">
                <div className="flex gap-3">
                  {s.meta?.image_url ? (
                    <img src={s.meta.image_url} alt={s.description}
                      className="w-24 h-24 object-cover rounded bg-slate-100" />
                  ) : (
                    <div className="w-24 h-24 bg-slate-100 rounded flex items-center justify-center text-xs text-slate-400">
                      无图
                    </div>
                  )}
                  <div className="flex-1 space-y-1">
                    <div className="text-xs text-slate-400">
                      镜头 {s.shot_idx} · {s.type} · {s.duration_sec.toFixed(1)}s
                    </div>
                    <p className="text-sm text-slate-700 line-clamp-2">{s.description}</p>
                    <p className="text-xs text-slate-500 line-clamp-2">「{s.narration}」</p>
                  </div>
                </div>
                {s.meta?.tts_url && (
                  <audio src={s.meta.tts_url} controls preload="none" className="w-full h-8" />
                )}
                <div className="flex gap-2 text-xs">
                  <button
                    disabled={pending[s.id] || regenImg.isPending}
                    onClick={async () => {
                      setPending((p) => ({ ...p, [s.id]: true }));
                      try { await regenImg.mutateAsync(aspect ?? ''); } finally {
                        setPending((p) => ({ ...p, [s.id]: false }));
                      }
                    }}
                    className="px-2 py-1 rounded border disabled:opacity-50"
                  >重新生图</button>
                  <button
                    disabled={pending[s.id] || regenTTS.isPending}
                    onClick={async () => {
                      setPending((p) => ({ ...p, [s.id]: true }));
                      try { await regenTTS.mutateAsync(); } finally {
                        setPending((p) => ({ ...p, [s.id]: false }));
                      }
                    }}
                    className="px-2 py-1 rounded border disabled:opacity-50"
                  >重新配音</button>
                  <span className="ml-auto px-2 py-0.5 rounded bg-slate-100 text-slate-700">{s.status}</span>
                </div>
              </li>
            ))}
          </ul>
        </section>
      ))}
    </div>
  );
}
