import { Link, useParams } from 'react-router-dom';
import { useProject, statusColor, statusLabel } from '../features/project/api';

export function ProjectDetailPage() {
  const { id } = useParams();
  const q = useProject(id);

  if (q.isLoading) return <div>加载中…</div>;
  if (q.error || !q.data) return <div className="text-red-600">项目不存在</div>;
  const p = q.data;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-semibold">{p.title}</h2>
          <div className="text-sm text-slate-500">
            {p.author || '佚名'} · {p.word_count} 字
          </div>
        </div>
        <span className={`px-2 py-0.5 rounded text-xs ${statusColor(p.status)}`}>
          {statusLabel(p.status)}
        </span>
      </div>

      <div className="border rounded p-4 bg-white">
        <p className="text-slate-700">下一步将依次执行：</p>
        <ol className="list-decimal ml-6 mt-2 space-y-1 text-sm text-slate-600">
          <li>章节切分（M2）</li>
          <li>角色提取 + 形象图（M3）</li>
          <li>分镜 + 配音（M4）</li>
          <li>合成成片（M5）</li>
        </ol>
        <div className="mt-4 flex gap-3 text-sm">
          <Link className="text-slate-700 hover:underline" to={`/projects/${p.id}/characters`}>角色</Link>
          <Link className="text-slate-700 hover:underline" to={`/projects/${p.id}/shots`}>分镜</Link>
          <Link className="text-slate-700 hover:underline" to={`/projects/${p.id}/chapters/1`}>章节</Link>
        </div>
      </div>
    </div>
  );
}
