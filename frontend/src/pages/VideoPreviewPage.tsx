import { Link, useParams } from 'react-router-dom';
import { useChapterVideo, useComposeChapter } from '../features/composition/api';

export function VideoPreviewPage() {
  const { id, chapter } = useParams();
  if (!id || !chapter) return null;
  return <PreviewFor projectId={id} chapterIndex={Number(chapter)} />;
}

function PreviewFor({ projectId, chapterIndex }: { projectId: string; chapterIndex: number }) {
  // We don't have chapter id from URL; fetch list and pick by index.
  // For M5 we expose a separate route that accepts chapter id. This is a stub.
  return (
    <div className="space-y-3">
      <h2 className="text-xl font-semibold">成片预览（章节 #{chapterIndex}）</h2>
      <p className="text-sm text-slate-500">
        M5 阶段。后续接 chapter-id 路由即可播放。
      </p>
      <Link className="text-slate-700 hover:underline text-sm" to={`/projects/${projectId}/chapters`}>← 返回章节列表</Link>
    </div>
  );
}

// Reusable player that takes an explicit chapter id (used by /chapters/:n linking in M6).
export function ChapterVideoPlayer({ chapterId, projectId, aspect }: { chapterId: string; projectId: string; aspect?: string }) {
  const q = useChapterVideo(chapterId);
  const compose = useComposeChapter(chapterId);
  const v = q.data;

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2">
        <button onClick={() => compose.mutate(aspect ?? '')} disabled={compose.isPending}
          className="px-3 py-1.5 rounded bg-slate-900 text-white disabled:opacity-50">
          {compose.isPending ? '排队中…' : (v?.video_url ? '重新合成' : '合成章节视频')}
        </button>
        {v?.status && (
          <span className="text-xs px-2 py-0.5 rounded bg-slate-100 text-slate-700">{v.status}</span>
        )}
        <Link className="text-sm text-slate-500 hover:underline" to={`/projects/${projectId}/chapters`}>← 返回</Link>
      </div>

      {v?.video_url ? (
        <video src={v.video_url} controls className="w-full max-w-md aspect-[9/16] bg-black rounded" />
      ) : (
        <div className="border rounded p-6 text-sm text-slate-500">
          {q.isLoading ? '加载中…' : '尚未合成。点击「合成章节视频」。'}
        </div>
      )}

      {v?.error && <p className="text-sm text-red-600">{v.error}</p>}
    </div>
  );
}
