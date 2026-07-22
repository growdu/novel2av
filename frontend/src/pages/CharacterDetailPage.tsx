import { useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { useCharacter, usePatchCharacter, useRegenImage, roleColor, roleLabel } from '../features/character/api';

export function CharacterDetailPage() {
  const { id } = useParams();
  const q = useCharacter(id);
  const patch = usePatchCharacter(id ?? '');
  const regen = useRegenImage(id ?? '');

  const [name, setName] = useState('');
  const [appearance, setAppearance] = useState('');
  const [voice, setVoice] = useState('');

  useEffect(() => {
    if (!q.data) return;
    setName(q.data.name);
    setAppearance(q.data.appearance);
    setVoice(q.data.voice);
  }, [q.data]);

  if (!q.data) return <div className="text-slate-500">加载中…</div>;
  const c = q.data;
  const url = c.meta?.ref_image_url;

  return (
    <div className="grid grid-cols-12 gap-4">
      <aside className="col-span-12 md:col-span-5">
        <div className="border rounded bg-white overflow-hidden">
          {url ? (
            <img src={url} alt={c.name} className="w-full aspect-square object-cover bg-slate-100" />
          ) : (
            <div className="w-full aspect-square bg-slate-100 flex items-center justify-center text-slate-400">
              暂无形象图
            </div>
          )}
          <div className="p-3 flex items-center justify-between">
            <span className={`text-xs px-2 py-0.5 rounded ${roleColor(c.role)}`}>{roleLabel(c.role)}</span>
            <button
              onClick={() => regen.mutate(4)}
              disabled={regen.isPending}
              className="px-3 py-1.5 rounded bg-slate-900 text-white disabled:opacity-50 text-sm"
            >
              {regen.isPending ? '排队中…' : '重新生成形象图 (4 张)'}
            </button>
          </div>
          {regen.error && <p className="text-sm text-red-600 px-3 pb-3">{(regen.error as Error).message}</p>}
        </div>
      </aside>

      <section className="col-span-12 md:col-span-7 space-y-3">
        <div className="border rounded bg-white p-4 space-y-3">
          <Link className="text-xs text-slate-500 hover:underline" to={`/projects/${c.project_id}/characters`}>← 返回画廊</Link>
          <label className="block">
            <span className="text-sm text-slate-700">名字</span>
            <input className="mt-1 w-full border rounded px-3 py-2 text-lg font-semibold"
              value={name} onChange={(e) => setName(e.target.value)}
              onBlur={() => name && name !== c.name && patch.mutate({ name })} />
          </label>
          <label className="block">
            <span className="text-sm text-slate-700">外貌描述</span>
            <textarea className="mt-1 w-full border rounded px-3 py-2 min-h-24"
              value={appearance} onChange={(e) => setAppearance(e.target.value)}
              onBlur={() => appearance !== c.appearance && patch.mutate({ appearance })} />
          </label>
          <label className="block">
            <span className="text-sm text-slate-700">音色描述</span>
            <input className="mt-1 w-full border rounded px-3 py-2"
              value={voice} onChange={(e) => setVoice(e.target.value)}
              onBlur={() => voice !== c.voice && patch.mutate({ voice })} />
          </label>
        </div>

        <div className="border rounded bg-slate-50 p-4 text-sm text-slate-600 space-y-2">
          <div><span className="text-slate-500">性格：</span>{c.personality || '—'}</div>
          {c.aliases.length > 0 && (
            <div><span className="text-slate-500">别名：</span>{c.aliases.join('、')}</div>
          )}
        </div>
      </section>
    </div>
  );
}
