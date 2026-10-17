// Centralized API client. The base URL is configurable via VITE_API_URL at
// build time. When empty (the single-binary deployment where the Go server
// serves the frontend), requests are made to same-origin relative paths.
const BASE = (import.meta.env.VITE_API_URL ?? '').replace(/\/$/, '')

async function request(path, options = {}) {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  const text = await res.text()
  const data = text ? JSON.parse(text) : null
  if (!res.ok) {
    const message = data?.error || `Request failed with ${res.status}`
    throw new Error(message)
  }
  return data
}

export const api = {
  listJobs: (limit = 50, offset = 0) =>
    request(`/api/jobs?limit=${limit}&offset=${offset}`),

  createJob: (name, cronSchedule) =>
    request('/api/jobs', {
      method: 'POST',
      body: JSON.stringify({ name, cron_schedule: cronSchedule }),
    }),

  updateJob: (id, patch) =>
    request(`/api/jobs/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(patch),
    }),

  deleteJob: (id) => request(`/api/jobs/${id}`, { method: 'DELETE' }),

  runNow: (id) => request(`/api/jobs/${id}/run`, { method: 'POST' }),

  jobHistory: (id, limit = 50) =>
    request(`/api/jobs/${id}/history?limit=${limit}`),

  health: () => request('/health'),
}
