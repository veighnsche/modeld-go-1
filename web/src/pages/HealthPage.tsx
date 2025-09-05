import { useEffect, useState } from 'react'
import { fullUrl, PATHS } from '../env'

export default function HealthPage() {
  const [status, setStatus] = useState<number | null>(null)
  const [body, setBody] = useState('')

  useEffect(() => {
    let didCancel = false
    ;(async () => {
      try {
        const res = await fetch(fullUrl(PATHS.health))
        const text = await res.text().catch(() => '')
        if (!didCancel) {
          setStatus(res.status)
          setBody(text)
        }
      } catch (e: any) {
        if (!didCancel) {
          setStatus(-1)
          setBody(String(e?.message || e))
        }
      }
    })()
    return () => { didCancel = true }
  }, [])

  return (
    <div style={{ padding: 12 }}>
      <h2>Health</h2>
      <div>HTTP Status: <span data-testid="health-status">{status ?? ''}</span></div>
      <pre data-testid="health-body" style={{ background: '#f6f8fa', padding: 8 }}>{body}</pre>
    </div>
  )
}
