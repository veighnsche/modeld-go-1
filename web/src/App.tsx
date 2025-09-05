import { useEffect, useMemo, useRef, useState } from 'react'

type Env = {
  VITE_API_BASE_URL?: string
  VITE_HEALTH_PATH?: string
  VITE_READY_PATH?: string
  VITE_MODELS_PATH?: string
  VITE_STATUS_PATH?: string
  VITE_INFER_PATH?: string
  VITE_USE_MOCKS?: string
  VITE_SEND_STREAM_FIELD?: string
}

const env: Env = import.meta.env as any

function getEnv(key: keyof Env, fallback: string) {
  const v = env[key]
  return (v ?? fallback) as string
}

function parseBool(v: string | undefined, fallback = false) {
  if (v == null) return fallback
  return /^(1|true|yes|on)$/i.test(v)
}

export default function App() {
  const API_BASE = getEnv('VITE_API_BASE_URL', '')
  const PATHS = {
    health: getEnv('VITE_HEALTH_PATH', '/healthz'),
    ready: getEnv('VITE_READY_PATH', '/readyz'),
    models: getEnv('VITE_MODELS_PATH', '/models'),
    status: getEnv('VITE_STATUS_PATH', '/status'),
    infer: getEnv('VITE_INFER_PATH', '/infer'),
  }
  const USE_MOCKS = parseBool(env.VITE_USE_MOCKS, false)
  const SEND_STREAM_FIELD = parseBool(env.VITE_SEND_STREAM_FIELD, false)

  const [prompt, setPrompt] = useState('')
  const [model, setModel] = useState('')
  const [status, setStatus] = useState<'idle'|'requesting'|'success'|'error'>('idle')
  const [streamLog, setStreamLog] = useState<string[]>([])
  const [resultJson, setResultJson] = useState('')
  const [latencyMs, setLatencyMs] = useState<number | null>(null)
  const [modelsCount, setModelsCount] = useState<number | null>(null)
  const abortRef = useRef<AbortController | null>(null)

  const mode = USE_MOCKS ? 'mock' : 'live'

  const fullUrl = (path: string) => (API_BASE ? API_BASE.replace(/\/$/, '') : '') + path

  useEffect(() => {
    let didCancel = false
    ;(async () => {
      try {
        const res = await fetch(fullUrl(PATHS.models))
        if (!res.ok) throw new Error(`models ${res.status}`)
        const list = await res.json()
        if (!didCancel) setModelsCount(Array.isArray(list) ? list.length : 0)
      } catch {
        // optional; ignore
      }
    })()
    return () => { didCancel = true }
  }, [])

  async function runMock() {
    setStatus('requesting')
    setStreamLog([])
    setResultJson('')
    setLatencyMs(null)
    const started = performance.now()

    const lines = [
      JSON.stringify({ type: 'token', content: 'Hello' }),
      JSON.stringify({ type: 'token', content: ' world' }),
      JSON.stringify({ done: true, message: 'mock-complete' })
    ]

    for (const line of lines) {
      await new Promise(r => setTimeout(r, 50))
      setStreamLog(prev => [...prev, line])
    }

    const finalObj = { ok: true, done: true, mode: 'mock', echo: { prompt, model: model || undefined } }
    setResultJson(JSON.stringify(finalObj, null, 2))
    setLatencyMs(Math.round(performance.now() - started))
    setStatus('success')
  }

  async function runLive() {
    setStatus('requesting')
    setStreamLog([])
    setResultJson('')
    setLatencyMs(null)
    abortRef.current?.abort()
    const ac = new AbortController()
    abortRef.current = ac
    const started = performance.now()

    const body: any = { prompt }
    if (model.trim()) body.model = model.trim()
    if (SEND_STREAM_FIELD) body.stream = true

    try {
      const res = await fetch(fullUrl(PATHS.infer), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
        signal: ac.signal,
      })

      if (!res.ok) {
        const text = await res.text().catch(() => '')
        setStatus('error')
        setResultJson(JSON.stringify({ error: true, status: res.status, body: text }, null, 2))
        setLatencyMs(Math.round(performance.now() - started))
        return
      }

      const reader = res.body?.getReader()
      const decoder = new TextDecoder('utf-8')
      let buffered = ''
      const lines: string[] = []

      if (!reader) throw new Error('No stream reader')

      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buffered += decoder.decode(value, { stream: true })
        let idx
        while ((idx = buffered.indexOf('\n')) !== -1) {
          const line = buffered.slice(0, idx).trim()
          buffered = buffered.slice(idx + 1)
          if (line) {
            lines.push(line)
            setStreamLog(prev => [...prev, line])
          }
        }
      }
      if (buffered.trim()) {
        lines.push(buffered.trim())
        setStreamLog(prev => [...prev, buffered.trim()])
      }

      // Try to summarize last JSON line as result
      const lastLine = lines.slice(-1)[0]
      try {
        const parsed = lastLine ? JSON.parse(lastLine) : { ok: true }
        setResultJson(JSON.stringify(parsed, null, 2))
      } catch {
        setResultJson(JSON.stringify({ ok: true, raw: lastLine ?? null }, null, 2))
      }

      setLatencyMs(Math.round(performance.now() - started))
      setStatus('success')
    } catch (e: any) {
      setStatus('error')
      setResultJson(JSON.stringify({ error: true, message: String(e?.message || e) }, null, 2))
      setLatencyMs(Math.round(performance.now() - started))
    }
  }

  const onSend = () => {
    if (USE_MOCKS) runMock()
    else runLive()
  }

  const modelsCountView = useMemo(() => {
    if (modelsCount == null) return null
    return (
      <div data-testid="models-count">{modelsCount}</div>
    )
  }, [modelsCount])

  return (
    <div style={{ padding: 12, fontFamily: 'ui-sans-serif, system-ui, sans-serif' }}>
      <div data-testid="mode">{mode}</div>
      {modelsCountView}
      <div style={{ display: 'grid', gap: 8, maxWidth: 800 }}>
        <textarea data-testid="prompt-input" value={prompt} onChange={e => setPrompt(e.target.value)} rows={5} />
        <input data-testid="model-input" value={model} onChange={e => setModel(e.target.value)} placeholder="optional model" />
        <button data-testid="submit-btn" onClick={onSend}>Send</button>
        <div data-testid="status">{status}</div>
        <div>Latency (ms): <span data-testid="latency-ms">{latencyMs ?? ''}</span></div>
        <pre data-testid="stream-log" style={{ background: '#f6f8fa', padding: 8, minHeight: 80 }}>
          {streamLog.map((l, i) => (<div key={i}>{l}</div>))}
        </pre>
        <pre data-testid="result-json" style={{ background: '#f6f8fa', padding: 8, minHeight: 80 }}>{resultJson}</pre>
      </div>
    </div>
  )
}
