import { useEffect, useState } from 'react'
import { fullUrl, PATHS } from '../env'

export default function StatusPage() {
  const [status, setStatus] = useState<number | null>(null)
  const [json, setJson] = useState<any>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let didCancel = false
    ;(async () => {
      try {
        const res = await fetch(fullUrl(PATHS.status))
        const ct = String(res.headers.get('content-type') || '')
        let body: any = null
        if (/json/i.test(ct)) {
          body = await res.json().catch(() => null)
        } else {
          const txt = await res.text().catch(() => '')
          body = { raw: txt }
        }
        if (!didCancel) {
          setStatus(res.status)
          setJson(body)
        }
      } catch (e: any) {
        if (!didCancel) setError(String(e?.message || e))
      }
    })()
    return () => { didCancel = true }
  }, [])

  const instancesCount = Array.isArray(json?.Instances) ? json.Instances.length : null

  return (
    <div style={{ padding: 12 }}>
      <h2>Status</h2>
      <div>HTTP Status: <span data-testid="status-status">{status ?? ''}</span></div>
      {instancesCount != null && (
        <div>Instances: <span data-testid="status-instances">{instancesCount}</span></div>
      )}
      {error ? (
        <pre data-testid="status-error" style={{ background: '#f6f8fa', padding: 8 }}>{error}</pre>
      ) : (
        <pre data-testid="status-json" style={{ background: '#f6f8fa', padding: 8 }}>{JSON.stringify(json, null, 2)}</pre>
      )}
    </div>
  )}
