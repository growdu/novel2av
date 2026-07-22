import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api, type Project, type ProjectStatus } from '../../lib/api/client';

const STATUS_LABEL: Record<ProjectStatus, string> = {
  CREATED: '已创建', SPLITTING: '切分中', SPLIT: '已切分',
  EXTRACTING: '提取角色', READY: '就绪', RUNNING: '生成中',
  DONE: '已完成', FAILED: '失败',
};

export function statusLabel(s: ProjectStatus) { return STATUS_LABEL[s] ?? s; }
export function statusColor(s: ProjectStatus) {
  switch (s) {
    case 'DONE': return 'bg-green-100 text-green-700';
    case 'FAILED': return 'bg-red-100 text-red-700';
    case 'RUNNING': case 'SPLITTING': case 'EXTRACTING':
      return 'bg-blue-100 text-blue-700';
    default: return 'bg-slate-100 text-slate-700';
  }
}

export function useProjects() {
  return useQuery({
    queryKey: ['projects'],
    queryFn: async () => {
      const { data, error } = await api.GET('/projects', { params: { query: { limit: 50 } } });
      if (error) throw error;
      return data;
    },
  });
}

export function useProject(id: string | undefined) {
  return useQuery({
    enabled: !!id,
    queryKey: ['projects', id],
    queryFn: async () => {
      const { data, error } = await api.GET('/projects/{id}', { params: { path: { id: id! } } });
      if (error) throw error;
      return data as Project;
    },
  });
}

export interface CreateProjectArgs {
  title: string;
  author: string;
  file: File;
  aspect?: '9:16' | '16:9';
  style?: string;
}

export function useCreateProject() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (args: CreateProjectArgs) => {
      const fd = new FormData();
      fd.append('title', args.title);
      fd.append('author', args.author);
      fd.append('config', JSON.stringify({ aspect: args.aspect ?? '9:16', style: args.style ?? 'cinematic' }));
      fd.append('file', args.file);
      const r = await fetch('/api/v1/projects', { method: 'POST', body: fd });
      if (!r.ok) {
        const body = await r.json().catch(() => ({}));
        throw new Error(body?.error?.message ?? `HTTP ${r.status}`);
      }
      return (await r.json()) as Project;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['projects'] }),
  });
}

export function useDeleteProject() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      const r = await fetch(`/api/v1/projects/${id}`, { method: 'DELETE' });
      if (!r.ok && r.status !== 204) {
        const body = await r.json().catch(() => ({}));
        throw new Error(body?.error?.message ?? `HTTP ${r.status}`);
      }
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['projects'] }),
  });
}
