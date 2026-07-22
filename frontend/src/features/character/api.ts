import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { Character } from '../../lib/api/client';

export const ROLE_LABEL = {
  protagonist: '主角', antagonist: '反派', supporting: '配角',
} as const;

export function roleLabel(r: string) { return (ROLE_LABEL as Record<string, string>)[r] ?? r; }
export function roleColor(r: string) {
  switch (r) {
    case 'protagonist': return 'bg-amber-100 text-amber-700';
    case 'antagonist': return 'bg-rose-100 text-rose-700';
    default: return 'bg-slate-100 text-slate-700';
  }
}

export function useCharacters(projectId: string | undefined) {
  return useQuery({
    enabled: !!projectId,
    queryKey: ['characters', projectId],
    queryFn: async () => {
      const r = await fetch(`/api/v1/projects/${projectId}/characters`);
      if (!r.ok) throw new Error(`HTTP ${r.status}`);
      return (await r.json()) as Character[];
    },
  });
}

export function useCharacter(id: string | undefined) {
  return useQuery({
    enabled: !!id,
    queryKey: ['character', id],
    queryFn: async () => {
      const r = await fetch(`/api/v1/characters/${id}`);
      if (!r.ok) throw new Error(`HTTP ${r.status}`);
      return (await r.json()) as Character;
    },
  });
}

export function useExtractCharacters(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      const r = await fetch(`/api/v1/projects/${projectId}/characters:extract`, { method: 'POST' });
      if (!r.ok) {
        const body = await r.json().catch(() => ({}));
        throw new Error(body?.error?.message ?? `HTTP ${r.status}`);
      }
      return (await r.json()) as { job_id: string; status: string };
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['characters', projectId] }),
  });
}

export function useIngestCharacters(projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      const r = await fetch(`/api/v1/projects/${projectId}/characters:ingest`, { method: 'POST' });
      if (!r.ok) throw new Error(`HTTP ${r.status}`);
      return (await r.json()) as { ingested: number };
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['characters', projectId] }),
  });
}

export function useRegenImage(characterId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (variants: number) => {
      const r = await fetch(`/api/v1/characters/${characterId}/image:regen`, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ variants }),
      });
      if (!r.ok) {
        const body = await r.json().catch(() => ({}));
        throw new Error(body?.error?.message ?? `HTTP ${r.status}`);
      }
      return (await r.json()) as { job_id: string; status: string };
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['character', characterId] }),
  });
}

export function usePatchCharacter(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (patch: Partial<Character>) => {
      const r = await fetch(`/api/v1/characters/${id}`, {
        method: 'PATCH',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify(patch),
      });
      if (!r.ok) {
        const body = await r.json().catch(() => ({}));
        throw new Error(body?.error?.message ?? `HTTP ${r.status}`);
      }
      return (await r.json()) as Character;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['character', id] });
      qc.invalidateQueries({ queryKey: ['characters'] });
    },
  });
}
