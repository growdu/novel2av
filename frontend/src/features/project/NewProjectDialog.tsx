import { useState } from 'react';
import { useCreateProject } from './api';

export function NewProjectDialog({ onClose }: { onClose: () => void }) {
  const [title, setTitle] = useState('');
  const [author, setAuthor] = useState('');
  const [file, setFile] = useState<File | null>(null);
  const [aspect, setAspect] = useState<'9:16' | '16:9'>('9:16');
  const [style, setStyle] = useState('cinematic');

  const m = useCreateProject();

  const submit = async () => {
    if (!title.trim() || !file) return;
    try {
      await m.mutateAsync({ title, author, file, aspect, style });
      onClose();
    } catch { /* surfaced via m.error */ }
  };

  return (
    <div className="fixed inset-0 bg-black/40 flex items-center justify-center p-4 z-50">
      <div className="bg-white rounded-lg w-full max-w-md p-6 space-y-4">
        <h2 className="text-lg font-semibold">新建项目</h2>

        <label className="block">
          <span className="text-sm text-slate-700">书名</span>
          <input
            className="mt-1 w-full border rounded px-3 py-2"
            value={title} onChange={(e) => setTitle(e.target.value)}
            placeholder="《XXX》"
          />
        </label>

        <label className="block">
          <span className="text-sm text-slate-700">作者</span>
          <input
            className="mt-1 w-full border rounded px-3 py-2"
            value={author} onChange={(e) => setAuthor(e.target.value)}
            placeholder="可选"
          />
        </label>

        <label className="block">
          <span className="text-sm text-slate-700">小说文件 (.txt / .md)</span>
          <input
            type="file"
            accept=".txt,.md"
            className="mt-1 w-full border rounded px-3 py-2"
            onChange={(e) => setFile(e.target.files?.[0] ?? null)}
          />
        </label>

        <div className="grid grid-cols-2 gap-3">
          <label className="block">
            <span className="text-sm text-slate-700">比例</span>
            <select className="mt-1 w-full border rounded px-3 py-2"
              value={aspect} onChange={(e) => setAspect(e.target.value as '9:16' | '16:9')}>
              <option value="9:16">竖屏 9:16</option>
              <option value="16:9">横屏 16:9</option>
            </select>
          </label>
          <label className="block">
            <span className="text-sm text-slate-700">风格</span>
            <input className="mt-1 w-full border rounded px-3 py-2"
              value={style} onChange={(e) => setStyle(e.target.value)} />
          </label>
        </div>

        {m.error && <p className="text-sm text-red-600">{(m.error as Error).message}</p>}

        <div className="flex justify-end gap-2 pt-2">
          <button onClick={onClose} className="px-4 py-2 rounded border">取消</button>
          <button
            onClick={submit}
            disabled={m.isPending || !title || !file}
            className="px-4 py-2 rounded bg-slate-900 text-white disabled:opacity-50"
          >
            {m.isPending ? '上传中…' : '创建'}
          </button>
        </div>
      </div>
    </div>
  );
}
