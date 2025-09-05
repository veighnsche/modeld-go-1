import { useRef, useState } from 'react'
import { fullUrl, PATHS, SEND_STREAM_FIELD, USE_MOCKS } from '../env'

export default function HaikuPage() {
  const [status, setStatus] = useState<'idle'|'requesting'|'success'|'error'>('idle')
  const [poem, setPoem] = useState('')
  const abortRef = useRef<AbortController | null>(null)
  const mode = USE_MOCKS ? 'mock' : 'live'

  async function generate() {
    setStatus('requesting')
    setPoem('…')
    abortRef.current?.abort()
    const ac = new AbortController()
    abortRef.current = ac

    const prompt = 'Write a 3-line haiku about the ocean.'

    if (USE_MOCKS) {
      // Minimal mock fallback for layout; content replaced in live mode.
      setTimeout(() => {
        setPoem('Ocean whispers blue\nWaves teach rocks the art of time\nSalt stars fade at dawn')
        setStatus('success')
      }, 300)
      return
    }

    try {
      const body: any = { prompt }
      if (SEND_STREAM_FIELD) body.stream = true
      // Align with InferPage defaults for stability
      body.max_tokens = 128
      body.temperature = 0.7
      body.top_p = 0.95
      const res = await fetch(fullUrl(PATHS.infer), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
        signal: ac.signal,
      })
      if (!res.ok) {
        const text = await res.text().catch(() => '')
        setStatus('error')
        setPoem(text || 'Error generating haiku')
        return
      }
      const reader = res.body?.getReader()
      const decoder = new TextDecoder('utf-8')
      let buffered = ''
      let content = ''
      const lines: string[] = []
      // Fallback path: if streaming not supported in this browser, read the full body and parse NDJSON
      if (!reader) {
        const text = await res.text()
        for (const raw of text.split('\n')) {
          const line = raw.trim()
          if (!line) continue
          lines.push(line)
          try {
            const obj = JSON.parse(line)
            if (typeof obj.token === 'string') {
              content += obj.token
            }
            if (obj.done === true && typeof obj.content === 'string') {
              content = obj.content
            }
            if (!content && typeof obj.completion === 'string') {
              content = obj.completion
            }
          } catch {}
        }
        if (!content && lines.length > 0) {
          try {
            const obj = JSON.parse(lines[lines.length-1])
            if (obj && typeof obj.content === 'string') content = obj.content
          } catch {}
        }
        setPoem(content || 'Error: no content')
        setStatus(content ? 'success' : 'error')
        return
      }
      // Safety timeout: abort if no completion within 20s
      const timeout = setTimeout(() => {
        try { ac.abort() } catch {}
      }, 20000)
      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buffered += decoder.decode(value, { stream: true })
        let idx
        while ((idx = buffered.indexOf('\n')) !== -1) {
          const line = buffered.slice(0, idx).trim()
          buffered = buffered.slice(idx + 1)
          if (!line) continue
          lines.push(line)
          try {
            const obj = JSON.parse(line)
            if (typeof obj.token === 'string') {
              content += obj.token
              setPoem(content)
            }
            if (obj.done === true && typeof obj.content === 'string') {
              content = obj.content
              setPoem(content)
            }
          } catch {
            // ignore non-JSON lines
          }
        }
      }
      if (buffered.trim()) {
        lines.push(buffered.trim())
        try {
          const obj = JSON.parse(buffered.trim())
          if (obj && obj.done === true && typeof obj.content === 'string') {
            content = obj.content
            setPoem(content)
          }
        } catch {}
      }
      // Fallback: if poem still empty, attempt to derive from last line or aggregated tokens
      if (!content) {
        const last = lines.slice(-1)[0]
        try {
          const obj = last ? JSON.parse(last) : null
          if (obj) {
            if (typeof obj.content === 'string' && obj.content.trim()) {
              content = obj.content
            } else if (typeof obj.completion === 'string' && obj.completion.trim()) {
              content = obj.completion
            } else if (typeof obj.token === 'string' && obj.token.trim()) {
              content = obj.token
            }
          }
        } catch {}
        if (content) setPoem(content)
      }
      clearTimeout(timeout)
      setStatus('success')
    } catch (e) {
      setStatus('error')
      if (!poem) setPoem('Error generating haiku')
    }
  }

  return (
    <div style={{ minHeight: '80vh', display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 16 }}>
      <div style={{ width: '100%', maxWidth: 900 }}>
        <div style={{ display: 'flex', justifyContent: 'center', marginBottom: 12 }}>
          <button data-testid="make-haiku-btn" onClick={generate} disabled={status==='requesting'}>
            {status==='requesting' ? 'Generating…' : 'Generate Haiku'}
          </button>
        </div>
        <div style={{ display: 'flex', justifyContent: 'center', marginBottom: 8 }}>
          <small data-testid="mode">{mode}</small>
        </div>
        <div style={{ display: 'flex', justifyContent: 'center', marginBottom: 8 }}>
          <small data-testid="haiku-status">{status}</small>
        </div>
        <div style={{ display:'flex', alignItems:'center', justifyContent:'center', minHeight: '50vh' }}>
          <div
            data-testid="haiku-poem"
            style={{
              whiteSpace: 'pre-wrap',
              textAlign: 'center',
              fontSize: 22,
              lineHeight: 1.5,
              maxWidth: 700,
            }}
          >{poem}</div>
        </div>
      </div>
    </div>
  )
}
