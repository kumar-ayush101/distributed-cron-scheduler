import { useState, useEffect, useCallback, useRef } from 'react'
import { api } from './api'
import './App.css'

const CRON_PRESETS = [
  { label: 'Every minute', value: '*/1 * * * *' },
  { label: 'Every 5 minutes', value: '*/5 * * * *' },
  { label: 'Hourly', value: '0 * * * *' },
  { label: 'Daily (midnight)', value: '0 0 * * *' },
  { label: 'Weekly (Mon)', value: '0 0 * * 1' },
]

function StatusBadge({ status }) {
  const s = (status || 'active').toLowerCase()
  return <span className={`badge badge-${s}`}>{s}</span>
}

function ExecBadge({ status }) {
  const ok = status === 'success'
  return <span className={`badge ${ok ? 'badge-success' : 'badge-failed'}`}>{status}</span>
}

function relativeTime(iso) {
  if (!iso) return '—'
  const diff = new Date(iso).getTime() - Date.now()
  const abs = Math.abs(diff)
  const mins = Math.round(abs / 60000)
  if (mins < 1) return diff >= 0 ? 'in <1 min' : 'just now'
  if (mins < 60) return diff >= 0 ? `in ${mins}m` : `${mins}m ago`
  const hrs = Math.round(mins / 60)
  if (hrs < 24) return diff >= 0 ? `in ${hrs}h` : `${hrs}h ago`
  const days = Math.round(hrs / 24)
  return diff >= 0 ? `in ${days}d` : `${days}d ago`
}

export default function App() {
  const [jobs, setJobs] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [online, setOnline] = useState(true)

  const [name, setName] = useState('')
  const [schedule, setSchedule] = useState('*/5 * * * *')
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState('')

  const [historyJob, setHistoryJob] = useState(null)
  const [historyLogs, setHistoryLogs] = useState([])
  const [historyLoading, setHistoryLoading] = useState(false)

  const [busyIds, setBusyIds] = useState(() => new Set())
  const firstLoad = useRef(true)

  const loadJobs = useCallback(async () => {
    try {
      const page = await api.listJobs()
      setJobs(page.items || [])
      setOnline(true)
      setError('')
    } catch (err) {
      setOnline(false)
      setError(err.message)
    } finally {
      if (firstLoad.current) {
        setLoading(false)
        firstLoad.current = false
      }
    }
  }, [])

  useEffect(() => {
    loadJobs()
    const id = setInterval(loadJobs, 5000)
    return () => clearInterval(id)
  }, [loadJobs])

  const markBusy = (id, busy) =>
    setBusyIds((prev) => {
      const next = new Set(prev)
      busy ? next.add(id) : next.delete(id)
      return next
    })

  const handleCreate = async (e) => {
    e.preventDefault()
    setFormError('')
    setSubmitting(true)
    try {
      await api.createJob(name.trim(), schedule.trim())
      setName('')
      setSchedule('*/5 * * * *')
      await loadJobs()
    } catch (err) {
      setFormError(err.message)
    } finally {
      setSubmitting(false)
    }
  }

  const handleRun = async (id) => {
    markBusy(id, true)
    try {
      await api.runNow(id)
      await loadJobs()
    } catch (err) {
      setError(err.message)
    } finally {
      markBusy(id, false)
    }
  }

  const handleToggle = async (job) => {
    const next = job.status === 'paused' ? 'active' : 'paused'
    markBusy(job.id, true)
    try {
      await api.updateJob(job.id, { status: next })
      await loadJobs()
    } catch (err) {
      setError(err.message)
    } finally {
      markBusy(job.id, false)
    }
  }

  const handleDelete = async (id) => {
    if (!window.confirm('Delete this job? This cannot be undone.')) return
    markBusy(id, true)
    try {
      await api.deleteJob(id)
      await loadJobs()
    } catch (err) {
      setError(err.message)
    } finally {
      markBusy(id, false)
    }
  }

  const openHistory = async (job) => {
    setHistoryJob(job)
    setHistoryLoading(true)
    setHistoryLogs([])
    try {
      const page = await api.jobHistory(job.id)
      setHistoryLogs(page.items || [])
    } catch (err) {
      setError(err.message)
    } finally {
      setHistoryLoading(false)
    }
  }

  const closeHistory = () => {
    setHistoryJob(null)
    setHistoryLogs([])
  }

  return (
    <div className="app">
      <header className="header">
        <div className="brand">
          <div className="logo">⏱</div>
          <div>
            <h1>Distributed Cron Scheduler</h1>
            <p className="subtitle">Exactly-once job execution across stateless nodes</p>
          </div>
        </div>
        <div className={`status-pill ${online ? 'online' : 'offline'}`}>
          <span className="dot" />
          {online ? 'API Online' : 'API Unreachable'}
        </div>
      </header>

      <section className="stats">
        <StatCard label="Total Jobs" value={jobs.length} />
        <StatCard label="Active" value={jobs.filter((j) => j.status === 'active').length} />
        <StatCard label="Paused" value={jobs.filter((j) => j.status === 'paused').length} />
        <StatCard
          label="Total Runs"
          value={jobs.reduce((acc, j) => acc + (j.run_count || 0), 0)}
        />
      </section>

      <section className="card">
        <h2 className="card-title">Create Job</h2>
        <form className="form" onSubmit={handleCreate}>
          <div className="field">
            <label>Name</label>
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. Nightly database backup"
              required
            />
          </div>
          <div className="field">
            <label>Cron Expression</label>
            <input
              className="mono"
              value={schedule}
              onChange={(e) => setSchedule(e.target.value)}
              placeholder="*/5 * * * *"
              required
            />
            <div className="presets">
              {CRON_PRESETS.map((p) => (
                <button
                  type="button"
                  key={p.value}
                  className={`chip ${schedule === p.value ? 'chip-active' : ''}`}
                  onClick={() => setSchedule(p.value)}
                >
                  {p.label}
                </button>
              ))}
            </div>
          </div>
          <button className="btn btn-primary" type="submit" disabled={submitting}>
            {submitting ? 'Creating…' : 'Create Job'}
          </button>
          {formError && <div className="form-error">{formError}</div>}
        </form>
      </section>

      <section className="card">
        <div className="card-header">
          <h2 className="card-title">Jobs</h2>
          <span className="muted">auto-refreshing every 5s</span>
        </div>

        {error && !online && <div className="banner-error">{error}</div>}

        {loading ? (
          <div className="empty">Loading jobs…</div>
        ) : jobs.length === 0 ? (
          <div className="empty">No jobs yet. Create one above to get started.</div>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>ID</th>
                  <th>Name</th>
                  <th>Schedule</th>
                  <th>Status</th>
                  <th>Next Run</th>
                  <th>Runs</th>
                  <th className="right">Actions</th>
                </tr>
              </thead>
              <tbody>
                {jobs.map((job) => {
                  const busy = busyIds.has(job.id)
                  return (
                    <tr key={job.id}>
                      <td className="mono muted">#{job.id}</td>
                      <td className="strong">{job.name}</td>
                      <td>
                        <code className="cron">{job.cron_schedule}</code>
                      </td>
                      <td><StatusBadge status={job.status} /></td>
                      <td>
                        <span title={job.next_run_at}>{relativeTime(job.next_run_at)}</span>
                      </td>
                      <td>
                        <span className="runs">{job.run_count || 0}</span>
                        {job.fail_count > 0 && (
                          <span className="fails"> / {job.fail_count} failed</span>
                        )}
                      </td>
                      <td className="right actions">
                        <button className="btn btn-ghost" disabled={busy} onClick={() => handleRun(job.id)}>
                          Run
                        </button>
                        <button className="btn btn-ghost" disabled={busy} onClick={() => handleToggle(job)}>
                          {job.status === 'paused' ? 'Resume' : 'Pause'}
                        </button>
                        <button className="btn btn-ghost" onClick={() => openHistory(job)}>
                          Logs
                        </button>
                        <button className="btn btn-danger" disabled={busy} onClick={() => handleDelete(job.id)}>
                          Delete
                        </button>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </section>

      {historyJob && (
        <div className="modal-overlay" onClick={closeHistory}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h3>
                Execution History · <span className="muted">{historyJob.name}</span>
              </h3>
              <button className="btn btn-ghost" onClick={closeHistory}>Close</button>
            </div>
            {historyLoading ? (
              <div className="empty">Loading history…</div>
            ) : historyLogs.length === 0 ? (
              <div className="empty">No executions recorded yet.</div>
            ) : (
              <div className="table-wrap">
                <table>
                  <thead>
                    <tr>
                      <th>Time</th>
                      <th>Status</th>
                      <th>Duration</th>
                      <th>Node</th>
                      <th>Details</th>
                    </tr>
                  </thead>
                  <tbody>
                    {historyLogs.map((log) => (
                      <tr key={log.id}>
                        <td>{new Date(log.run_at).toLocaleString()}</td>
                        <td><ExecBadge status={log.status} /></td>
                        <td className="mono">{log.duration_ms}ms</td>
                        <td className="mono muted">{log.node_id || '—'}</td>
                        <td className="mono">{log.details}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>
      )}

      <footer className="footer">
        <span>Go · PostgreSQL · Redis · React</span>
      </footer>
    </div>
  )
}

function StatCard({ label, value }) {
  return (
    <div className="stat">
      <div className="stat-value">{value}</div>
      <div className="stat-label">{label}</div>
    </div>
  )
}
