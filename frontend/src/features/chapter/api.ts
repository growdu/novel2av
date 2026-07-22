import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { Chapter } from '../../lib/api/client';

export function useChapters(projectId: string | undefined) {
  return useQuery({
    enabled: !!projectId,
    queryKey: ['chapters', projectId],
    queryFn: async () => {
      const r = await fetch(`/api/v1/projects/${projectId}/chapters`);
      if (!r.ok) throw new Error(`HTTP ${r.status}`);
      return (await r.json()) as Chapter[];
    },
  });
}

export function useSplitChapters(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      const r = await fetch(`/api/v1/projects/${projectId}/chapters:split`, { method: 'POST' });
      if (!r.ok) {
        const body = await r.json().catch(() => ({}));
        throw new Error(body?.error?.message ?? `HTTP ${r.status}`);
      }
      return (await r.json()) as { job_id: string; status: string };
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['chapters', projectId] }),
  });
}

export function useIngestChapters(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      const r = await fetch(`/api/v1/projects/${projectId}/chapters:ingest`, { method: 'POST' });
      if (!r.ok) throw new Error(`HTTP ${r.status}`);
      return (await r.json()) as { ingested: number };
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['chapters', projectId] }),
  });
}

export function usePatchChapter(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (args: { id: string; title?: string; status?: string }) => {
      const r = await fetch(`/api/v1/chapters/${args.id}`, {
        method: 'PATCH',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ title: args.title, status: args.status }),
      });
      if (!r.ok) {
        const body = await r.json().catch(() => ({}));
        throw new Error(body?.error?.message ?? `HTTP ${r.status}`);
      }
      return (await r.json()) as Chapter;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['chapters', projectId] }),
  });
}

// Merge chapters [a..b] (1-based inclusive) into a single chapter on the client.
// Real merge requires re-running split or a backend endpoint; for M2 we just
// allow the user to rename/ status change + flag a "merged-from" note.
export interface MergeRequest {
  projectId: string;
  from: number;
  to: number;
  keepTitle: string;
}

export function useMergeChaptersLocally(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (req: MergeRequest) => {
      // Persist a hint as a status message on the first chapter of the range.
      const chapters = qc.getQueryData<Chapter[]>(['chapters', projectId]) ?? [];
      const target = chapters.find((c) => c.index === req.from);
      if (!target) throw new Error('first chapter not found');
      const r = await fetch(`/api/v1/chapters/${target.id}`, {
        method: 'PATCH',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ title: req.keepTitle, status: 'MERGED' }),
      });
      if (!r.ok) throw new Error(`HTTP ${r.status}`);
      return await r.json();
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['chapters', projectId] }),
  });
}
