import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { ProjectVideoSigned } from '../../lib/api/client';

export function useProjectVideo(projectId: string | undefined) {
  return useQuery({
    enabled: !!projectId,
    queryKey: ['project-video', projectId],
    queryFn: async () => {
      const r = await fetch(`/api/v1/projects/${projectId}/full`);
      if (!r.ok) throw new Error(`HTTP ${r.status}`);
      return (await r.json()) as ProjectVideoSigned;
    },
  });
}

export function useComposeProjectVideo(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      const r = await fetch(`/api/v1/projects/${projectId}/full/compose`, { method: 'POST' });
      if (!r.ok) {
        const body = await r.json().catch(() => ({}));
        throw new Error(body?.error?.message ?? `HTTP ${r.status}`);
      }
      return (await r.json()) as { job_id: string; status: string };
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['project-video', projectId] }),
  });
}

export function useIngestProjectVideo(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (body: { video_key: string; duration_sec: number; status?: string; error?: string }) => {
      const r = await fetch(`/api/v1/projects/${projectId}/full/ingest`, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify(body),
      });
      if (!r.ok) throw new Error(`HTTP ${r.status}`);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['project-video', projectId] }),
  });
}
