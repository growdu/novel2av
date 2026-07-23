import openapiFetch from 'openapi-fetch';

export type ProjectStatus =
  | 'CREATED' | 'SPLITTING' | 'SPLIT' | 'EXTRACTING'
  | 'READY' | 'RUNNING' | 'DONE' | 'FAILED';

export interface Project {
  id: string; user_id: string; title: string; author: string;
  source_key: string; status: ProjectStatus; word_count: number;
  config: { aspect?: string; style?: string; voice?: string };
  created_at: string; updated_at: string;
}

export interface Chapter { id: string; project_id: string; index: number; title: string; word_count: number; status: string; content_key: string; created_at: string; }
export type CharacterRole = 'protagonist' | 'antagonist' | 'supporting';
export interface Character {
  id: string; project_id: string; name: string; aliases: string[];
  role: CharacterRole; appearance: string; personality: string; voice: string;
  ref_image_key: string; meta: { ref_image_url?: string } | null; created_at: string;
}
export interface Shot {
  id: string; chapter_id: string; scene_idx: number; shot_idx: number;
  type: string; description: string; narration: string; mood: string;
  duration_sec: number; status: string;
  image_key: string; tts_key: string; bgm_key: string; subtitle_key: string;
  meta: { image_url?: string; tts_url?: string; bgm_url?: string } | null;
  created_at: string;
}
export interface ChapterVideo {
  chapter_id: string; video_key: string; duration_sec: number;
  status: string; error: string; created_at: string; updated_at: string;
}
export interface ProjectVideoSigned {
  project_id: string; video_key?: string; video_url?: string;
  duration_sec: number; status: string; error: string;
}

export interface ListResponse<T> { items: T[]; limit: number; offset: number; }

export const api = openapiFetch<{
  '/projects': {
    GET: { parameters?: { query?: { limit?: number; offset?: number } }; responses: { 200: ListResponse<Project> } };
    POST: { responses: { 201: Project } };
  };
  '/projects/{id}': {
    GET: { responses: { 200: Project; 404: { error: { code: string; message: string } } } };
    DELETE: { responses: { 204: unknown } };
  };
  '/projects/{id}/chapters': { GET: { responses: { 200: Chapter[] } } };
  '/projects/{id}/chapters:split': { POST: { responses: { 202: { job_id: string; status: string } } } };
  '/projects/{id}/chapters:ingest': { POST: { responses: { 200: { ingested: number } } } };
  '/chapters/{id}': {
    PATCH: { requestBody?: { content?: { 'application/json': { title?: string; status?: string } } };
             responses: { 200: Chapter } };
  };
  '/projects/{id}/characters': { GET: { responses: { 200: Character[] } } };
  '/projects/{id}/characters:extract': { POST: { responses: { 202: { job_id: string; status: string } } } };
  '/projects/{id}/characters:ingest': { POST: { responses: { 200: { ingested: number } } } };
  '/characters/{id}': {
    GET: { responses: { 200: Character } };
    PATCH: { requestBody?: { content?: { 'application/json': Partial<Character> } }; responses: { 200: Character } };
    DELETE: { responses: { 204: unknown } };
  };
  '/characters/{id}/image:regen': { POST: { responses: { 202: { job_id: string; status: string } } } };
  '/characters/{id}/image:ingest': { POST: { responses: { 204: unknown } } };
  '/projects/{id}/shots': { GET: { responses: { 200: Shot[] } } };
  '/projects/{id}/shots:breakdown': { POST: { responses: { 202: { job_ids: string[]; status: string } } } };
  '/projects/{id}/shots:breakdown:ingest': { POST: { responses: { 200: { ingested: number } } } };
  '/shots/{id}': { GET: { responses: { 200: Shot } } };
  '/shots/{id}/image:regen': { POST: { responses: { 202: { job_id: string; status: string } } } };
  '/shots/{id}/tts:regen': { POST: { responses: { 202: { job_id: string; status: string } } } };
  '/shots/{id}/assets:ingest': { POST: { responses: { 200: Shot } } };
  '/chapters/{id}/video': { GET: { responses: { 200: ProjectVideoSigned } } };
  '/chapters/{id}/video:compose': { POST: { responses: { 202: { job_id: string; status: string } } } };
  '/chapters/{id}/video:ingest': { POST: { responses: { 204: unknown } } };
  '/projects/{id}/videos': { GET: { responses: { 200: ChapterVideo[] } } };
  '/projects/{id}/videos/compose': { POST: { responses: { 202: { job_ids: string[]; status: string } } } };
  '/projects/{id}/full': { GET: { responses: { 200: ProjectVideoSigned } } };
  '/projects/{id}/full/compose': { POST: { responses: { 202: { job_id: string; status: string } } } };
  '/projects/{id}/full/ingest': { POST: { responses: { 204: unknown } } };
}>({ baseUrl: '/api/v1' });
