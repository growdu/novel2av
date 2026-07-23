import { useComposeProjectVideo, useProjectVideo } from './api';

export function FullVideoPanel({ projectId, aspect }: { projectId: string; aspect?: string }) {
  const q = useProjectVideo(projectId);
  const compose = useComposeProjectVideo(projectId);
  const v = q.data;
  const aspectClass = aspect === '16:9' ? 'aspect-video' : 'aspect-[9/16]';

  return (
    <div className="border rounded bg-white p-4 space-y-3">
      <div className="flex items-center gap-2">
        <h3 className="font-medium">全本视频</h3>
        <span className="text-xs text-slate-400">{v?.duration_sec ? `${v.duration_sec.toFixed(1)}s` : '—'}</span>
        <span className="ml-auto text-xs px-2 py-0.5 rounded bg-slate-100 text-slate-700">{v?.status ?? 'PENDING'}</span>
      </div>

      {v?.video_url ? (
        <div className="space-y-2">
          <video src={v.video_url} controls className={`w-full max-w-sm ${aspectClass} bg-black rounded`} />
          <div className="flex gap-2 text-sm">
            <a href={v.video_url} download={`full.mp4`}
              className="px-3 py-1.5 rounded border hover:bg-slate-50">下载 mp4</a>
            <button onClick={() => compose.mutate()} disabled={compose.isPending}
              className="px-3 py-1.5 rounded border disabled:opacity-50">
              {compose.isPending ? '排队中…' : '重新合成'}
            </button>
          </div>
        </div>
      ) : (
        <div className="space-y-2">
          <p className="text-sm text-slate-500">
            {q.isLoading ? '加载中…' : '尚未合成全本视频。请先确保每个章节都已合成，再点击下方按钮。'}
          </p>
          <button onClick={() => compose.mutate()} disabled={compose.isPending}
            className="px-3 py-1.5 rounded bg-slate-900 text-white disabled:opacity-50">
            {compose.isPending ? '排队中…' : '一键合成全本视频'}
          </button>
        </div>
      )}

      {v?.error && <p className="text-sm text-red-600">{v.error}</p>}
      {compose.error && <p className="text-sm text-red-600">{(compose.error as Error).message}</p>}
    </div>
  );
}
