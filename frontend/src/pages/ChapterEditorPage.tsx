import { useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { ChapterListPanel } from '../features/chapter/ChapterListPanel';
import { useChapters, useMergeChaptersLocally, usePatchChapter } from '../features/chapter/api';
import { ChapterVideoPlayer } from './VideoPreviewPage';

export function ChapterEditorPage() {
  const { id, n } = useParams();
  if (!id || !n) return null;
  const idx = parseInt(n, 10);
  const q = useChapters(id);
  const ch = q.data?.find((c) => c.index === idx);
  const patch = usePatchChapter(id);
  const merge = useMergeChaptersLocally(id);

  const [title, setTitle] = useState(ch?.title ?? '');
  const [mergeFrom, setMergeFrom] = useState(idx);
  const [mergeTo, setMergeTo] = useState(idx + 1);

  if (!ch) return <div className="text-slate-500">正在加载该章节…</div>;

  return (
    <div className="grid grid-cols-12 gap-4">
      <aside className="col-span-4">
        <ChapterListPanel projectId={id} />
      </aside>

      <section className="col-span-8 space-y-4">
        <div className="border rounded bg-white p-4 space-y-3">
          <div className="text-sm text-slate-500">
            第 {ch.index} 章 · {ch.word_count} 字 · <Link className="hover:underline" to={`/projects/${id}/chapters`}>返回列表</Link>
          </div>
          <input
            className="w-full text-lg font-semibold border-b focus:outline-none"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            onBlur={() => title !== ch.title && patch.mutate({ id: ch.id, title })}
          />
        </div>

        <div className="border rounded bg-white p-4 space-y-2">
          <h3 className="font-medium">合并到相邻章节</h3>
          <p className="text-sm text-slate-500">
            M2 阶段：把 [from, to] 范围内的章节标记为合并态，并保留首章标题。
          </p>
          <div className="flex items-center gap-2 text-sm">
            <input type="number" min={1} value={mergeFrom}
              onChange={(e) => setMergeFrom(parseInt(e.target.value, 10))}
              className="w-20 border rounded px-2 py-1" />
            <span>到</span>
            <input type="number" min={1} value={mergeTo}
              onChange={(e) => setMergeTo(parseInt(e.target.value, 10))}
              className="w-20 border rounded px-2 py-1" />
            <button
              onClick={() => merge.mutate({ projectId: id, from: mergeFrom, to: mergeTo, keepTitle: title })}
              disabled={merge.isPending || mergeFrom >= mergeTo}
              className="px-3 py-1.5 rounded bg-slate-900 text-white disabled:opacity-50"
            >合并</button>
          </div>
          {merge.error && <p className="text-sm text-red-600">{(merge.error as Error).message}</p>}
        </div>

        <div className="border rounded bg-white p-4">
          <h3 className="font-medium mb-2">成片预览</h3>
          <ChapterVideoPlayer chapterId={ch.id} projectId={id} aspect="9:16" />
        </div>
      </section>
    </div>
  );
}
