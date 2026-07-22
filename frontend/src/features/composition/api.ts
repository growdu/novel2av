import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { ChapterVideo } from '../../lib/api/client';

export interface ChapterVideoSigned extends ChapterVideo {
  video_url?: string;
}

export function useChapterVideo(chapterId: string | undefined) {
  return useQuery({
    enabled: !!chapterId,
    queryKey: ['chapter-video', chapterId],
    queryFn: async () => {
      const r = await fetch(`/api/v1/chapters/${chapterId}/video`);
      if (!r.ok) throw new Error(`HTTP ${r.status}`);
      return (await r.json()) as ChapterVideoSigned;
    },
  });
}

export function useProjectVideos(projectId: string | undefined) {
  return useQuery({
    enabled: !!projectId,
    queryKey: ['project-videos', projectId],
    queryFn: async () => {
      const r = await fetch(`/api/v1/projects/${projectId}/videos`);
      if (!r.ok) throw new Error(`HTTP ${r.status}`);
      return (await r.json()) as ChapterVideo[];
    },
  });
}

export function useComposeChapter(chapterId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (aspect?: string) => {
      const r = await fetch(`/api/v1/chapters/${chapterId}/video:compose`, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ aspect: aspect ?? '' }),
      });
      if (!r.ok) {
        const body = await r.json().catch(() => ({}));
        throw new Error(body?.error?.message ?? `HTTP ${r.status}`);
      }
      return (await r.json()) as { job_id: string; status: string };
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['chapter-video', chapterId] }),
  });
}

export function useComposeProject(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (aspect?: string) => {
      const r = await fetch(`/api/v1/projects/${projectId}/videos/compose`, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ aspect: aspect ?? '' }),
      });
      if (!r.ok) throw new Error(`HTTP ${r.status}`);
      return (await r.json()) as { job_ids: string[]; status: string };
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['project-videos', projectId] }),
  });
}

export function useIngestChapterVideo(chapterId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (body: { video_key: string; duration_sec: number; status?: string; error?: string }) => {
      const r = await fetch(`/api/v1/chapters/${chapterId}/video:ingest`, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify(body),
      });
      if (!r.ok) throw new Error(`HTTP ${r.status}`);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['chapter-video', chapterId] }),
  });
}
