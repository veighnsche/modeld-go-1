import { useEffect, useState } from 'react'
import { fullUrl, PATHS } from '../env'

export default function ModelsPage() {
  const [status, setStatus] = useState<number | null>(null)
  const [json, setJson] = useState<any>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let didCancel = false
    ;(async () => {
      try {
        const res = await fetch(fullUrl(PATHS.models))
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

  const count = Array.isArray(json) ? json.length : Array.isArray(json?.models) ? json.models.length : null

  return (
    <div style={{ padding: 12 }}>
      <h2>Models</h2>
      <div>HTTP Status: <span data-testid="models-status">{status ?? ''}</span></div>
      {count != null && (
        <div>Count: <span data-testid="models-count-page">{count}</span></div>
      )}
      {error ? (
        <pre data-testid="models-error" style={{ background: '#f6f8fa', padding: 8 }}>{error}</pre>
      ) : (
        <pre data-testid="models-json" style={{ background: '#f6f8fa', padding: 8 }}>{JSON.stringify(json, null, 2)}</pre>
      )}
    </div>
  )
}
