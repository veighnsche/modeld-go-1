import { useEffect, useState } from 'react'
import { fullUrl, PATHS } from '../env'

export default function ReadyPage() {
  const [status, setStatus] = useState<number | null>(null)
  const [body, setBody] = useState('')

  useEffect(() => {
    let didCancel = false
    ;(async () => {
      try {
        const res = await fetch(fullUrl(PATHS.ready))
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
      <h2>Ready</h2>
      <div>HTTP Status: <span data-testid="ready-status">{status ?? ''}</span></div>
      <pre data-testid="ready-body" style={{ background: '#f6f8fa', padding: 8 }}>{body}</pre>
    </div>
  )
}
