import { useState, useEffect } from 'react'

function App() {
  const [jobs, setJobs] = useState([])
  const [name, setName] = useState('')
  const [schedule, setSchedule] = useState('*/1 * * * *')
  const [loading, setLoading] = useState(false)

  //function to get the jobs

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


  //function to add new job

  const handleSubmit = async (e) => {
    e.preventDefault()
    setLoading(true)

    try {
      const res = await fetch('http://localhost:8080/jobs', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ 
          name: name, 
          cron_schedule: schedule 
        }),
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

  //function for deletion of job

  const handleDelete = async (id) => {
    if (!confirm("Are you sure you want to delete this job?")) return;
    try {
      await fetch(`http://localhost:8080/jobs?id=${id}`, { method: 'DELETE' })
      fetchJobs() // Refresh list
    } catch (err) {
      console.error(err)
    }
  }

  //function to run job now
  const handleRunNow = async (id) => {
    try {
      await fetch(`http://localhost:8080/jobs/run?id=${id}`, { method: 'POST' })
      alert("Job scheduled to run immediately!")
      fetchJobs()
    } catch (err) {
      console.error(err)
    }
  }

  return (
    <div style={styles.container}>
      <h1 style={styles.header}>Distributed Cron Dashboard</h1>

      {/* Form Section */}
      <div style={styles.card}>
        <h2 style={{marginTop: 0}}>Add New Job</h2>
        <form onSubmit={handleSubmit} style={styles.form}>
          <input 
            style={styles.input}
            placeholder="Job Name (e.g. Email Report)" 
            value={name} 
            onChange={(e) => setName(e.target.value)} 
            required 
          />
          <input 
            style={styles.input}
            placeholder="Cron Schedule (e.g. */5 * * * *)" 
            value={schedule} 
            onChange={(e) => setSchedule(e.target.value)} 
            required 
          />
          <button style={styles.button} type="submit" disabled={loading}>
            {loading ? 'Scheduling...' : 'Schedule Job'}
          </button>
        </form>
      </div>

      {/* List Section */}
      <div style={styles.card}>
        <h2 style={{marginTop: 0}}>Active Jobs ({jobs.length})</h2>
        <table style={styles.table}>
          <thead>
            <tr>
              <th style={styles.th}>ID</th>
              <th style={styles.th}>Name</th>
              <th style={styles.th}>Schedule</th>
              <th style={styles.th}>Next Run</th>
              <th style={styles.th}>Actions</th> {/* NEW HEADER */}
            </tr>
          </thead>
          <tbody>
            {jobs.map((job) => (
              <tr key={job.id} style={styles.tr}>
                <td style={styles.td}><b>#{job.id}</b></td>
                <td style={styles.td}>{job.name}</td>
                <td style={styles.td}>
                  <span style={styles.cronBadge}>{job.cron_schedule}</span>
                </td>
                <td style={styles.td}>
                  {new Date(job.next_run_at).toLocaleTimeString()}
                </td>
                {/* delete and run now button */}
                <td style={styles.td}>
                  <button 
                    style={{...styles.actionButton, background: '#28a745'}} 
                    onClick={() => handleRunNow(job.id)}
                    title="Run Now"
                  >
                    ▶
                  </button>
                  <button 
                    style={{...styles.actionButton, background: '#dc3545', marginLeft: '8px'}} 
                    onClick={() => handleDelete(job.id)}
                    title="Delete"
                  >
                    🗑
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {jobs.length === 0 && <p style={{textAlign:'center', color: '#888', marginTop: '20px'}}>No jobs running.</p>}
      </div>
    </div>
  )
}


const styles = {
  container: { 
    maxWidth: '900px', 
    margin: '0 auto', 
    padding: '40px 20px', 
    fontFamily: "'Segoe UI', Roboto, Helvetica, Arial, sans-serif", 
    backgroundColor: '#eef2f5', 
    minHeight: '100vh' 
  },
  header: { 
    textAlign: 'center', 
    color: '#2c3e50', 
    marginBottom: '30px',
    fontSize: '2.5rem'
  },
  card: { 
    background: 'white', 
    padding: '25px', 
    borderRadius: '12px', 
    boxShadow: '0 4px 6px rgba(0,0,0,0.05)', 
    marginBottom: '30px',
    border: '1px solid #e1e4e8'
  },
  form: { 
    display: 'flex', 
    gap: '15px', 
    flexWrap: 'wrap' 
  },
  input: { 
    flex: 1, 
    padding: '12px 15px', 
    borderRadius: '6px', 
    border: '2px solid #e1e4e8', 
    fontSize: '16px',
    outline: 'none',
    transition: 'border-color 0.2s'
  },
  button: { 
    padding: '12px 25px', 
    background: '#007bff', 
    color: 'white', 
    border: 'none', 
    borderRadius: '6px', 
    cursor: 'pointer', 
    fontWeight: 'bold', 
    fontSize: '16px',
    transition: 'background 0.2s'
  },
  table: { 
    width: '100%', 
    borderCollapse: 'separate', 
    borderSpacing: '0', 
    marginTop: '15px',
    border: '1px solid #e1e4e8',
    borderRadius: '8px',
    overflow: 'hidden'
  },
  th: { 
    textAlign: 'left', 
    padding: '18px 15px', 
    background: '#007bff',  
    color: 'white',         
    fontWeight: '600',
    fontSize: '16px',
    borderBottom: '2px solid #0056b3'
  },
  td: { 
    padding: '15px', 
    borderBottom: '1px solid #eee', 
    color: '#333',
    fontSize: '15px'
  },
  tr: { 
    backgroundColor: '#fff' 
  },
  cronBadge: {
    background: '#f1f3f5',
    color: '#c92a2a', 
    padding: '4px 8px',
    borderRadius: '4px',
    fontFamily: 'monospace',
    fontWeight: 'bold',
    fontSize: '14px',
    border: '1px solid #dee2e6'
  },
  actionButton: {
    border: 'none',
    color: 'white',
    padding: '6px 12px',
    borderRadius: '4px',
    cursor: 'pointer',
    fontSize: '14px',
    transition: 'opacity 0.2s',
  }
}

export default App