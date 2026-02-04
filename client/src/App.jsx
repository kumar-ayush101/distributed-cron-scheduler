import { useState, useEffect } from 'react'

function App() {
  const [jobs, setJobs] = useState([])
  const [name, setName] = useState('')
  const [schedule, setSchedule] = useState('*/1 * * * *')
  const [loading, setLoading] = useState(false)

  // new state for history model
  const [selectedJob, setSelectedJob] = useState(null) // Which job is clicked
  const [historyLogs, setHistoryLogs] = useState([])   // The logs for that job
  const [historyLoading, setHistoryLoading] = useState(false)

  // --- LOGIC STARTS HERE (UNTOUCHED) ---

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

  // logic ending here

  return (
    <div style={styles.container}>
      <header style={styles.header}>
        <h1 style={styles.title}>DISTRIBUTED CRON SCHEDULER</h1>
        <div style={styles.statusIndicator}>● ONLINE</div>
      </header>

      {/* add job form */}
      <div style={styles.card}>
        <h2 style={styles.cardTitle}>CREATE NEW TASK</h2>
        <form onSubmit={handleSubmit} style={styles.form}>
          <div style={styles.inputGroup}>
            <label style={styles.label}>TASK NAME</label>
            <input 
              style={styles.input}
              placeholder="e.g. Database Backup" 
              value={name} onChange={(e) => setName(e.target.value)} required 
            />
          </div>
          <div style={styles.inputGroup}>
             <label style={styles.label}>CRON EXPRESSION</label>
            <input 
              style={styles.input}
              placeholder="e.g. */5 * * * *" 
              value={schedule} onChange={(e) => setSchedule(e.target.value)} required 
            />
          </div>
          <div style={{display: 'flex', alignItems: 'flex-end'}}>
             <button style={styles.primaryButton} type="submit" disabled={loading}>
              {loading ? 'PROCESSING...' : 'INITIALIZE TASK'}
            </button>
          </div>
        </form>
      </div>

      {/* active jobs list */}
      <div style={styles.card}>
        <div style={styles.cardHeader}>
            <h2 style={styles.cardTitle}>ACTIVE WORKERS ({jobs.length})</h2>
        </div>
        
        <div style={{overflowX: 'auto'}}>
            <table style={styles.table}>
            <thead>
                <tr>
                <th style={styles.th}>ID</th>
                <th style={styles.th}>OPERATION</th>
                <th style={styles.th}>SCHEDULE</th>
                <th style={styles.th}>NEXT EXECUTION</th>
                <th style={styles.thRight}>CONTROLS</th>
                </tr>
            </thead>
            <tbody>
                {jobs.map((job) => (
                <tr key={job.id} style={styles.tr}>
                    <td style={styles.td}>
                        <span style={styles.idBadge}>#{job.id}</span>
                    </td>
                    <td style={styles.td}><b>{job.name}</b></td>
                    <td style={styles.td}>
                        <span style={styles.cronText}>{job.cron_schedule}</span>
                    </td>
                    <td style={styles.td}>{new Date(job.next_run_at).toLocaleString()}</td>
                    <td style={styles.tdRight}>
                    <div style={styles.actionGroup}>
                        {/* run now */}
                        <button 
                            style={styles.iconButton} 
                            onClick={() => handleRunNow(job.id)}
                            title="Run Now"
                        >
                            RUN ▷
                        </button>
                        {/* history menu */}
                        <button 
                            style={styles.iconButton} 
                            onClick={() => handleShowHistory(job)}
                            title="View History"
                        >
                            LOGS ☰
                        </button>
                        {/* delete */}
                        <button 
                            style={styles.deleteButton} 
                            onClick={() => handleDelete(job.id)}
                            title="Delete"
                        >
                            DEL ✕
                        </button>
                    </div>
                    </td>
                </tr>
                ))}
                {jobs.length === 0 && (
                    <tr>
                        <td colSpan="5" style={{...styles.td, textAlign: 'center', padding: '40px', color: '#888'}}>
                            NO ACTIVE TASKS FOUND
                        </td>
                    </tr>
                )}
            </tbody>
            </table>
        </div>
      </div>

      {/* history modal */}
      {selectedJob && (
        <div style={styles.modalOverlay}>
          <div style={styles.modalContent}>
            <div style={styles.modalHeader}>
              <h3 style={{margin: 0, textTransform: 'uppercase'}}>
                LOGS: <span style={{color: '#666'}}>{selectedJob.name}</span>
              </h3>
              <button onClick={closeModal} style={styles.closeButton}>CLOSE [ESC]</button>
            </div>
            
            {historyLoading ? (
              <div style={styles.loadingState}>FETCHING DATA...</div>
            ) : (
              <div style={styles.tableWrapper}>
                <table style={styles.table}>
                  <thead>
                    <tr>
                      <th style={styles.th}>TIMESTAMP</th>
                      <th style={styles.th}>STATUS</th>
                      <th style={styles.th}>OUTPUT</th>
                    </tr>
                  </thead>
                  <tbody>
                    {historyLogs.map((log) => (
                      <tr key={log.id} style={styles.tr}>
                        <td style={styles.tdLogs}>{new Date(log.run_at).toLocaleString()}</td>
                        <td style={styles.tdLogs}>
                          <span style={{
                            ...styles.statusBadge, 
                            border: log.status === 'Success' ? '1px solid #000' : '1px solid #000',
                            backgroundColor: log.status === 'Success' ? '#fff' : '#000',
                            color: log.status === 'Success' ? '#000' : '#fff',
                          }}>
                            {log.status === 'Success' ? 'SUCCESS' : 'FAILURE'}
                          </span>
                        </td>
                        <td style={{...styles.tdLogs, fontFamily: 'monospace'}}>{log.details}</td>
                      </tr>
                    ))}
                    {historyLogs.length === 0 && (
                      <tr><td colSpan="3" style={styles.td}>No execution history recorded.</td></tr>
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

// MODERN BLACK & WHITE STYLES
const styles = {
  container: { 
    maxWidth: '1000px', 
    margin: '0 auto', 
    padding: '40px 20px', 
    fontFamily: "'Inter', 'Segoe UI', sans-serif", 
    backgroundColor: '#fff', 
    color: '#000', 
    minHeight: '100vh',
    letterSpacing: '-0.02em'
  },
  header: { 
    display: 'flex', 
    justifyContent: 'space-between', 
    alignItems: 'center', 
    borderBottom: '2px solid #000', 
    paddingBottom: '20px', 
    marginBottom: '40px' 
  },
  title: { 
    margin: 0, 
    fontSize: '1.5rem', 
    fontWeight: '800', 
    letterSpacing: '-0.05em' 
  },
  statusIndicator: { 
    fontSize: '0.8rem', 
    fontWeight: 'bold', 
    border: '1px solid #000', 
    padding: '4px 8px', 
    borderRadius: '50px' 
  },
  
  // Cards
  card: { 
    border: '1px solid #000', 
    padding: '0', 
    marginBottom: '40px', 
    backgroundColor: '#fff' 
  },
  cardHeader: {
    padding: '20px',
    borderBottom: '1px solid #000',
    backgroundColor: '#fafafa'
  },
  cardTitle: { 
    margin: 0, 
    fontSize: '1rem', 
    fontWeight: '700', 
    textTransform: 'uppercase', 
    letterSpacing: '0.05em' 
  },
  
  // Form
  form: { 
    display: 'flex', 
    gap: '20px', 
    padding: '20px', 
    flexWrap: 'wrap',
    alignItems: 'flex-end'
  },
  inputGroup: {
    flex: 1,
    display: 'flex',
    flexDirection: 'column',
    gap: '8px',
    minWidth: '200px'
  },
  label: {
    fontSize: '0.75rem',
    fontWeight: 'bold',
    color: '#666'
  },
  input: { 
    padding: '12px', 
    border: '1px solid #000', 
    fontSize: '0.9rem', 
    outline: 'none',
    fontFamily: 'monospace',
    backgroundColor: '#fff',
    borderRadius: 0, // Sharp edges
  },
  primaryButton: { 
    padding: '12px 24px', 
    background: '#000', 
    color: '#fff', 
    border: '1px solid #000', 
    cursor: 'pointer', 
    fontWeight: 'bold', 
    fontSize: '0.85rem',
    textTransform: 'uppercase',
    minWidth: '150px',
    transition: 'background 0.2s'
  },

  // Table
  table: { 
    width: '100%', 
    borderCollapse: 'collapse', 
    fontSize: '0.9rem' 
  },
  th: { 
    textAlign: 'left', 
    padding: '15px 20px', 
    borderBottom: '1px solid #000', 
    fontWeight: '700', 
    fontSize: '0.75rem', 
    textTransform: 'uppercase', 
    color: '#666'
  },
  thRight: { 
    textAlign: 'right', 
    padding: '15px 20px', 
    borderBottom: '1px solid #000', 
    fontWeight: '700', 
    fontSize: '0.75rem', 
    textTransform: 'uppercase', 
    color: '#666'
  },
  td: { 
    padding: '15px 20px', 
    borderBottom: '1px solid #eee', 
    color: '#000',
    verticalAlign: 'middle'
  },
  tdLogs: {
    padding: '12px 20px', 
    borderBottom: '1px solid #eee', 
    color: '#000',
    fontSize: '0.85rem'
  },
  tdRight: {
    padding: '15px 20px', 
    borderBottom: '1px solid #eee', 
    textAlign: 'right'
  },
  tr: { 
    transition: 'background 0.1s' 
  },
  
  // Badges & Text
  idBadge: { 
    fontFamily: 'monospace', 
    color: '#888' 
  },
  cronText: { 
    fontFamily: 'monospace', 
    background: '#eee', 
    padding: '2px 6px', 
    fontSize: '0.85rem' 
  },
  
  // Actions
  actionGroup: {
    display: 'flex',
    gap: '10px',
    justifyContent: 'flex-end'
  },
  iconButton: { 
    background: 'transparent', 
    border: '1px solid #ddd', 
    color: '#000', 
    padding: '6px 12px', 
    cursor: 'pointer', 
    fontSize: '0.75rem', 
    fontWeight: 'bold',
    transition: 'all 0.2s',
    borderRadius: 0
  },
  deleteButton: {
    background: '#000', 
    border: '1px solid #000', 
    color: '#fff', 
    padding: '6px 12px', 
    cursor: 'pointer', 
    fontSize: '0.75rem', 
    fontWeight: 'bold',
    borderRadius: 0
  },

  // Modal
  modalOverlay: { 
    position: 'fixed', 
    top: 0, 
    left: 0, 
    right: 0, 
    bottom: 0, 
    backgroundColor: 'rgba(255,255,255,0.9)', 
    backdropFilter: 'blur(2px)',
    display: 'flex', 
    justifyContent: 'center', 
    alignItems: 'center', 
    zIndex: 1000 
  },
  modalContent: { 
    background: '#fff', 
    border: '2px solid #000',
    padding: '0', 
    width: '90%', 
    maxWidth: '700px', 
    maxHeight: '80vh', 
    overflowY: 'auto', 
    boxShadow: '20px 20px 0px rgba(0,0,0,0.1)' 
  },
  modalHeader: { 
    display: 'flex', 
    justifyContent: 'space-between', 
    alignItems: 'center', 
    padding: '20px', 
    borderBottom: '1px solid #000',
    backgroundColor: '#fafafa'
  },
  closeButton: { 
    background: 'none', 
    border: 'none', 
    fontSize: '0.85rem', 
    fontWeight: 'bold',
    cursor: 'pointer', 
    textDecoration: 'underline' 
  },
  statusBadge: { 
    padding: '4px 8px', 
    fontSize: '0.75rem', 
    fontWeight: 'bold',
    textTransform: 'uppercase',
    display: 'inline-block',
    minWidth: '80px',
    textAlign: 'center'
  },
  tableWrapper: { 
    maxHeight: '60vh', 
    overflowY: 'auto' 
  },
  loadingState: {
    padding: '40px',
    textAlign: 'center',
    color: '#666',
    fontStyle: 'italic'
  }
}

export default App