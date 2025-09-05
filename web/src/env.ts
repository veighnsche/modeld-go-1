export type Env = {
  VITE_API_BASE_URL?: string
  VITE_HEALTH_PATH?: string
  VITE_READY_PATH?: string
  VITE_MODELS_PATH?: string
  VITE_STATUS_PATH?: string
  VITE_INFER_PATH?: string
  VITE_SEND_STREAM_FIELD?: string
}

export const env: Env = (import.meta as any).env as any

export function getEnv<K extends keyof Env>(key: K, fallback: string): string {
  const v = env[key]
  return (v ?? fallback) as string
}

export function parseBool(v: string | undefined, fallback = false) {
  if (v == null) return fallback
  return /^(1|true|yes|on)$/i.test(v)
}

export const API_BASE = getEnv('VITE_API_BASE_URL', '')
export const PATHS = {
  health: getEnv('VITE_HEALTH_PATH', '/healthz'),
  ready: getEnv('VITE_READY_PATH', '/readyz'),
  models: getEnv('VITE_MODELS_PATH', '/models'),
  status: getEnv('VITE_STATUS_PATH', '/status'),
  infer: getEnv('VITE_INFER_PATH', '/infer'),
}

export const SEND_STREAM_FIELD = parseBool(env.VITE_SEND_STREAM_FIELD, false)

export const fullUrl = (path: string) => (API_BASE ? API_BASE.replace(/\/$/, '') : '') + path
