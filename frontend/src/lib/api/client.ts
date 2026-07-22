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

export interface Chapter {
  id: string;
  project_id: string;
  index: number;
  title: string;
  word_count: number;
  status: string;
  created_at: string;
}

export interface ListResponse<T> { items: T[]; limit: number; offset: number; }

// Hand-written schema until the backend exposes real OpenAPI.
export const api = openapiFetch<{
  '/projects': {
    GET: { parameters?: { query?: { limit?: number; offset?: number } }; responses: { 200: ListResponse<Project> } };
    POST: { responses: { 201: Project } };
  };
  '/projects/{id}': {
    GET: { responses: { 200: Project; 404: { error: { code: string; message: string } } } };
    DELETE: { responses: { 204: unknown } };
  };
  '/projects/{id}/chapters': {
    GET: { responses: { 200: Chapter[] } };
  };
  '/projects/{id}/chapters:split': {
    POST: { responses: { 202: { job_id: string; status: string } } };
  };
  '/projects/{id}/chapters:ingest': {
    POST: { responses: { 200: { ingested: number } } };
  };
  '/chapters/{id}': {
    PATCH: {
      requestBody?: { content?: { 'application/json': { title?: string; status?: string } } };
      responses: { 200: Chapter };
    };
  };
}>({ baseUrl: '/api/v1' });
