import { useState, useEffect } from 'react'

function App() {
  const [jobs, setJobs] = useState([])
  const [name, setName] = useState('')
  const [schedule, setSchedule] = useState('*/1 * * * *')
  const [loading, setLoading] = useState(false)

  // NEW STATE FOR HISTORY MODAL
  const [selectedJob, setSelectedJob] = useState(null) // Which job is clicked
  const [historyLogs, setHistoryLogs] = useState([])   // The logs for that job
  const [historyLoading, setHistoryLoading] = useState(false)

  // 1. Fetch Jobs (Main List)
  const fetchJobs = async () => {
    try {
      const res = await fetch('http://localhost:8080/jobs')
      const data = await res.json()
      setJobs(data || [])
    } catch (err) {
      console.error("Failed to fetch jobs:", err)
    }
  }

  useEffect(() => {
    fetchJobs()
    const interval = setInterval(fetchJobs, 5000)
    return () => clearInterval(interval)
  }, [])

  // 2. fetch history
  const handleShowHistory = async (job) => {
    setSelectedJob(job) // open the modal
    setHistoryLoading(true)
    setHistoryLogs([])  // clear old logs

    try {
      const res = await fetch(`http://localhost:8080/jobs/history?job_id=${job.id}`)
      const data = await res.json()
      setHistoryLogs(data || [])
    } catch (err) {
      console.error("Failed to fetch history:", err)
    } finally {
      setHistoryLoading(false)
    }
  }

  const closeModal = () => {
    setSelectedJob(null) // close the modal
    setHistoryLogs([])
  }

  // 3. form submit
  const handleSubmit = async (e) => {
    e.preventDefault()
    setLoading(true)
    try {
      const res = await fetch('http://localhost:8080/jobs', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, cron_schedule: schedule }),
      })
      if (res.ok) {
        setName('')
        setSchedule('*/1 * * * *')
        fetchJobs() 
      } else {
        alert("Failed to create job")
      }
    } catch (err) {
      console.error(err)
    } finally {
      setLoading(false)
    }
  }

  const handleDelete = async (id) => {
    if (!confirm("Are you sure you want to delete this job?")) return;
    try {
      await fetch(`http://localhost:8080/jobs?id=${id}`, { method: 'DELETE' })
      fetchJobs() 
    } catch (err) { console.error(err) }
  }

  const handleRunNow = async (id) => {
    try {
      await fetch(`http://localhost:8080/jobs/run?id=${id}`, { method: 'POST' })
      alert("Job scheduled to run immediately!")
      fetchJobs()
    } catch (err) { console.error(err) }
  }

  return (
    <div style={styles.container}>
      <h1 style={styles.header}>Distributed Cron Dashboard</h1>

      {/* add job form */}
      <div style={styles.card}>
        <h2 style={{marginTop: 0}}>Add New Job</h2>
        <form onSubmit={handleSubmit} style={styles.form}>
          <input 
            style={styles.input}
            placeholder="Job Name (e.g. Email Report)" 
            value={name} onChange={(e) => setName(e.target.value)} required 
          />
          <input 
            style={styles.input}
            placeholder="Cron Schedule (e.g. */5 * * * *)" 
            value={schedule} onChange={(e) => setSchedule(e.target.value)} required 
          />
          <button style={styles.button} type="submit" disabled={loading}>
            {loading ? 'Scheduling...' : 'Schedule Job'}
          </button>
        </form>
      </div>

      {/* active jobs list */}
      <div style={styles.card}>
        <h2 style={{marginTop: 0}}>Active Jobs ({jobs.length})</h2>
        <table style={styles.table}>
          <thead>
            <tr>
              <th style={styles.th}>ID</th>
              <th style={styles.th}>Name</th>
              <th style={styles.th}>Schedule</th>
              <th style={styles.th}>Next Run</th>
              <th style={styles.th}>Actions</th>
            </tr>
          </thead>
          <tbody>
            {jobs.map((job) => (
              <tr key={job.id} style={styles.tr}>
                <td style={styles.td}><b>#{job.id}</b></td>
                <td style={styles.td}>{job.name}</td>
                <td style={styles.td}><span style={styles.cronBadge}>{job.cron_schedule}</span></td>
                <td style={styles.td}>{new Date(job.next_run_at).toLocaleTimeString()}</td>
                <td style={styles.td}>
                  {/* run now */}
                  <button 
                    style={{...styles.actionButton, background: '#28a745'}} 
                    onClick={() => handleRunNow(job.id)}
                    title="Run Now"
                  >▶</button>
                  {/* history menu */}
                  <button 
                    style={{...styles.actionButton, background: '#17a2b8', marginLeft: '8px'}} 
                    onClick={() => handleShowHistory(job)}
                    title="View History"
                  >📜</button>
                  {/* delete */}
                  <button 
                    style={{...styles.actionButton, background: '#dc3545', marginLeft: '8px'}} 
                    onClick={() => handleDelete(job.id)}
                    title="Delete"
                  >🗑</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* history modal */}
      {selectedJob && (
        <div style={styles.modalOverlay}>
          <div style={styles.modalContent}>
            <div style={styles.modalHeader}>
              <h3>History: {selectedJob.name}</h3>
              <button onClick={closeModal} style={styles.closeButton}>×</button>
            </div>
            
            {historyLoading ? (
              <p>Loading history...</p>
            ) : (
              <div style={styles.tableWrapper}>
                <table style={styles.table}>
                  <thead>
                    <tr>
                      <th style={styles.th}>Run At</th>
                      <th style={styles.th}>Status</th>
                      <th style={styles.th}>Details</th>
                    </tr>
                  </thead>
                  <tbody>
                    {historyLogs.map((log) => (
                      <tr key={log.id}>
                        <td style={styles.td}>{new Date(log.run_at).toLocaleString()}</td>
                        <td style={styles.td}>
                          <span style={{
                            ...styles.statusBadge, 
                            background: log.status === 'Success' ? '#d4edda' : '#f8d7da',
                            color: log.status === 'Success' ? '#155724' : '#721c24'
                          }}>
                            {log.status}
                          </span>
                        </td>
                        <td style={styles.td}>{log.details}</td>
                      </tr>
                    ))}
                    {historyLogs.length === 0 && (
                      <tr><td colSpan="3" style={styles.td}>No history found.</td></tr>
                    )}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}

// STYLES
const styles = {
  container: { maxWidth: '900px', margin: '0 auto', padding: '40px 20px', fontFamily: "'Segoe UI', Roboto, Helvetica, Arial, sans-serif", backgroundColor: '#eef2f5', minHeight: '100vh' },
  header: { textAlign: 'center', color: '#2c3e50', marginBottom: '30px', fontSize: '2.5rem' },
  card: { background: 'white', padding: '25px', borderRadius: '12px', boxShadow: '0 4px 6px rgba(0,0,0,0.05)', marginBottom: '30px', border: '1px solid #e1e4e8' },
  form: { display: 'flex', gap: '15px', flexWrap: 'wrap' },
  input: { flex: 1, padding: '12px 15px', borderRadius: '6px', border: '2px solid #e1e4e8', fontSize: '16px', outline: 'none' },
  button: { padding: '12px 25px', background: '#007bff', color: 'white', border: 'none', borderRadius: '6px', cursor: 'pointer', fontWeight: 'bold', fontSize: '16px' },
  table: { width: '100%', borderCollapse: 'separate', borderSpacing: '0', marginTop: '15px', border: '1px solid #e1e4e8', borderRadius: '8px', overflow: 'hidden' },
  th: { textAlign: 'left', padding: '15px', background: '#007bff', color: 'white', fontWeight: '600', borderBottom: '2px solid #0056b3' },
  td: { padding: '12px 15px', borderBottom: '1px solid #eee', color: '#333' },
  tr: { backgroundColor: '#fff' },
  cronBadge: { background: '#f1f3f5', color: '#c92a2a', padding: '4px 8px', borderRadius: '4px', fontFamily: 'monospace', fontWeight: 'bold', fontSize: '14px', border: '1px solid #dee2e6' },
  actionButton: { border: 'none', color: 'white', padding: '6px 12px', borderRadius: '4px', cursor: 'pointer', fontSize: '14px', transition: 'opacity 0.2s' },
  
  // modal styles
  modalOverlay: { position: 'fixed', top: 0, left: 0, right: 0, bottom: 0, backgroundColor: 'rgba(0,0,0,0.5)', display: 'flex', justifyContent: 'center', alignItems: 'center', zIndex: 1000 },
  modalContent: { background: 'white', padding: '20px', borderRadius: '8px', width: '90%', maxWidth: '600px', maxHeight: '80vh', overflowY: 'auto', boxShadow: '0 5px 15px rgba(0,0,0,0.3)' },
  modalHeader: { display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '20px', borderBottom: '1px solid #eee', paddingBottom: '10px' },
  closeButton: { background: 'none', border: 'none', fontSize: '24px', cursor: 'pointer', color: '#666' },
  statusBadge: { padding: '4px 8px', borderRadius: '12px', fontSize: '12px', fontWeight: 'bold' },
  tableWrapper: { maxHeight: '60vh', overflowY: 'auto' }
}

export default App