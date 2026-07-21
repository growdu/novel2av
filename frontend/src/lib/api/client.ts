import openapiFetch from 'openapi-fetch';

// Replace with the generated client once `pnpm gen:api` is wired.
// import type { paths } from './schema';
// export const api = openapiFetch<paths>({ baseUrl: '/api/v1' });

// Stub client used until the backend exposes a real OpenAPI document.
export const api = openapiFetch<Record<string, never>>({ baseUrl: '/api/v1' });
