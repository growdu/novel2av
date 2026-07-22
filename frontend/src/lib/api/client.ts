import openapiFetch from 'openapi-fetch';

// Replace with the generated client once `npm run gen:api` is wired against
// the backend's published OpenAPI document.
export type ProjectStatus =
  | 'CREATED' | 'SPLITTING' | 'SPLIT' | 'EXTRACTING'
  | 'READY' | 'RUNNING' | 'DONE' | 'FAILED';

export interface Project {
  id: string;
  user_id: string;
  title: string;
  author: string;
  source_key: string;
  status: ProjectStatus;
  word_count: number;
  config: { aspect?: string; style?: string; voice?: string };
  created_at: string;
  updated_at: string;
}

export interface ListResponse<T> {
  items: T[];
  limit: number;
  offset: number;
}

// Minimal hand-written schema until the backend exposes real OpenAPI.
export const api = openapiFetch<{
  '/projects': {
    GET: { parameters?: { query?: { limit?: number; offset?: number } }; responses: { 200: ListResponse<Project> } };
    POST: { requestBody?: { content?: never }; responses: { 201: Project } };
  };
  '/projects/{id}': {
    GET: { responses: { 200: Project; 404: { error: { code: string; message: string } } } };
    DELETE: { responses: { 204: unknown } };
  };
}>({ baseUrl: '/api/v1' });
