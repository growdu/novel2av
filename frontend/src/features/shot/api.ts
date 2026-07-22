import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { Shot } from '../../lib/api/client';

export function useShots(projectId: string | undefined) {
  return useQuery({
    enabled: !!projectId,
    queryKey: ['shots', projectId],
    queryFn: async () => {
      const r = await fetch(`/api/v1/projects/${projectId}/shots`);
      if (!r.ok) throw new Error(`HTTP ${r.status}`);
      return (await r.json()) as Shot[];
    },
  });
}

export function useShot(id: string | undefined) {
  return useQuery({
    enabled: !!id,
    queryKey: ['shot', id],
    queryFn: async () => {
      const r = await fetch(`/api/v1/shots/${id}`);
      if (!r.ok) throw new Error(`HTTP ${r.status}`);
      return (await r.json()) as Shot;
    },
  });
}

export function useTriggerBreakdown(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      const r = await fetch(`/api/v1/projects/${projectId}/shots:breakdown`, { method: 'POST' });
      if (!r.ok) throw new Error(`HTTP ${r.status}`);
      return (await r.json()) as { job_ids: string[]; status: string };
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['shots', projectId] }),
  });
}

export function useIngestBreakdown(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (chapterId: string) => {
      const r = await fetch(`/api/v1/projects/${projectId}/shots:breakdown:ingest`, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ chapter_id: chapterId }),
      });
      if (!r.ok) throw new Error(`HTTP ${r.status}`);
      return (await r.json()) as { ingested: number };
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['shots', projectId] }),
  });
}

export function useRegenShotImage(shotId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (aspect?: string) => {
      const r = await fetch(`/api/v1/shots/${shotId}/image:regen`, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ aspect: aspect ?? '' }),
      });
      if (!r.ok) throw new Error(`HTTP ${r.status}`);
      return (await r.json()) as { job_id: string; status: string };
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['shot', shotId] }),
  });
}

export function useRegenShotTTS(shotId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      const r = await fetch(`/api/v1/shots/${shotId}/tts:regen`, { method: 'POST' });
      if (!r.ok) throw new Error(`HTTP ${r.status}`);
      return (await r.json()) as { job_id: string; status: string };
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['shot', shotId] }),
  });
}

export function useIngestShotAssets(shotId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (body: { image_key?: string; tts_key?: string; bgm_key?: string; subtitle_key?: string }) => {
      const r = await fetch(`/api/v1/shots/${shotId}/assets:ingest`, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify(body),
      });
      if (!r.ok) throw new Error(`HTTP ${r.status}`);
      return (await r.json()) as Shot;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['shot', shotId] }),
  });
}
