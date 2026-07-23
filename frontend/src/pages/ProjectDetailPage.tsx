import { Link, useParams } from 'react-router-dom';
import { useProject, statusColor, statusLabel } from '../features/project/api';
import { useComposeProject } from '../features/composition/api';
import { FullVideoPanel } from '../features/project_video/FullVideoPanel';

export function ProjectDetailPage() {
  const { id } = useParams();
  const q = useProject(id);
  const compose = useComposeProject(id ?? '');

  if (q.isLoading) return <div>加载中…</div>;
  if (q.error || !q.data) return <div className="text-red-600">项目不存在</div>;
  const p = q.data;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-semibold">{p.title}</h2>
          <div className="text-sm text-slate-500">{p.author || '佚名'} · {p.word_count} 字</div>
        </div>
        <span className={`px-2 py-0.5 rounded text-xs ${statusColor(p.status)}`}>{statusLabel(p.status)}</span>
      </div>

      <div className="border rounded p-4 bg-white space-y-3">
        <p className="text-slate-700">步骤：</p>
        <ol className="list-decimal ml-6 space-y-1 text-sm text-slate-600">
          <li>章节切分（M2）</li>
          <li>角色提取 + 形象图（M3）</li>
          <li>分镜 + 配音（M4）</li>
          <li>章节合成（M5）</li>
          <li className="font-medium text-emerald-700">全本合并（M6）</li>
        </ol>
        <div className="flex flex-wrap gap-2">
          <button onClick={() => compose.mutate(p.config?.aspect ?? '9:16')}
            disabled={compose.isPending}
            className="px-3 py-1.5 rounded bg-slate-900 text-white disabled:opacity-50 text-sm">
            {compose.isPending ? '排队中…' : '一键合成全部章节'}
          </button>
          <Link to={`/projects/${p.id}/videos`}
            className="px-3 py-1.5 rounded border text-sm hover:bg-slate-50">
            查看章节成片
          </Link>
        </div>
      </div>

      <FullVideoPanel projectId={p.id} aspect={p.config?.aspect ?? '9:16'} />

      <div className="flex gap-3 text-sm">
        <Link className="text-slate-700 hover:underline" to={`/projects/${p.id}/characters`}>角色</Link>
        <Link className="text-slate-700 hover:underline" to={`/projects/${p.id}/shots`}>分镜</Link>
        <Link className="text-slate-700 hover:underline" to={`/projects/${p.id}/chapters`}>章节</Link>
        <Link className="text-slate-700 hover:underline" to={`/projects/${p.id}/videos`}>成片</Link>
      </div>
    </div>
  );
}
