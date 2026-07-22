import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useDeleteProject, useProjects, statusColor, statusLabel } from '../features/project/api';
import { NewProjectDialog } from '../features/project/NewProjectDialog';

export function ProjectsPage() {
  const q = useProjects();
  const del = useDeleteProject();
  const [open, setOpen] = useState(false);
  const [pendingDel, setPendingDel] = useState<string | null>(null);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-semibold">项目</h2>
        <button onClick={() => setOpen(true)}
          className="px-4 py-2 rounded bg-slate-900 text-white">新建项目</button>
      </div>

      {q.isLoading && <div className="text-slate-500">加载中…</div>}
      {q.error && <div className="text-red-600">加载失败</div>}

      {q.data && q.data.items.length === 0 && (
        <div className="border rounded p-8 text-center text-slate-500">
          还没有项目。点击「新建项目」开始。
        </div>
      )}

      <ul className="divide-y border rounded bg-white">
        {q.data?.items.map((p) => (
          <li key={p.id} className="flex items-center justify-between p-4">
            <div className="space-y-1">
              <Link to={`/projects/${p.id}`} className="font-medium text-slate-900 hover:underline">
                {p.title}
              </Link>
              <div className="text-sm text-slate-500">
                {p.author || '佚名'} · {p.word_count} 字 · 更新于 {new Date(p.updated_at).toLocaleString()}
              </div>
            </div>
            <div className="flex items-center gap-3">
              <span className={`px-2 py-0.5 rounded text-xs ${statusColor(p.status)}`}>
                {statusLabel(p.status)}
              </span>
              <button
                onClick={() => setPendingDel(p.id)}
                className="text-sm text-red-600 hover:underline"
              >
                删除
              </button>
            </div>
          </li>
        ))}
      </ul>

      {open && <NewProjectDialog onClose={() => setOpen(false)} />}

      {pendingDel && (
        <div className="fixed inset-0 bg-black/40 flex items-center justify-center p-4 z-50">
          <div className="bg-white rounded p-6 w-full max-w-sm space-y-4">
            <p>确认删除该项目及其所有素材？此操作不可恢复。</p>
            <div className="flex justify-end gap-2">
              <button onClick={() => setPendingDel(null)} className="px-3 py-1.5 rounded border">取消</button>
              <button
                onClick={async () => {
                  const id = pendingDel; setPendingDel(null);
                  await del.mutateAsync(id);
                }}
                disabled={del.isPending}
                className="px-3 py-1.5 rounded bg-red-600 text-white disabled:opacity-50"
              >
                确认删除
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
