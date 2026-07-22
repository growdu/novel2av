import { Link, useParams } from 'react-router-dom';
import { useProjectVideos } from '../features/composition/api';
import { useChapters } from '../features/chapter/api';

export function ProjectVideosPage() {
  const { id } = useParams();
  const vids = useProjectVideos(id);
  const chs = useChapters(id);

  if (!id) return null;
  const byId = new Map((chs.data ?? []).map((c) => [c.id, c]));

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-semibold">成片</h2>
        <Link to={`/projects/${id}`} className="text-sm text-slate-500 hover:underline">← 项目详情</Link>
      </div>

      {vids.isLoading && <div className="text-slate-500">加载中…</div>}
      {vids.error && <div className="text-red-600">加载失败</div>}

      <ul className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
        {vids.data?.map((v) => {
          const ch = byId.get(v.chapter_id);
          return (
            <li key={v.chapter_id} className="border rounded bg-white p-3 space-y-2">
              <div className="text-sm text-slate-700">
                {ch ? `第 ${ch.index} 章 · ${ch.title}` : `章节 ${v.chapter_id.slice(0, 8)}`}
              </div>
              <div className="text-xs text-slate-500">
                {v.status} · {v.duration_sec.toFixed(1)}s
              </div>
              {v.video_key && (
                <span className="text-xs text-emerald-700">已生成（key: {v.video_key}）</span>
              )}
              <Link className="block text-sm text-slate-700 hover:underline"
                to={`/projects/${id}/chapters/${ch?.index ?? 1}`}>
                打开章节编辑器播放 →
              </Link>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
