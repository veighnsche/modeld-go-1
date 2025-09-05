import React, { Suspense, lazy } from 'react'
import { Link, Navigate, Route, Routes } from 'react-router-dom'
import InferPage from './pages/InferPage'
import HealthPage from './pages/HealthPage'
const ReadyPage = lazy(() => import('./pages/ReadyPage'))
const ModelsPage = lazy(() => import('./pages/ModelsPage'))
const StatusPage = lazy(() => import('./pages/StatusPage'))

export default function App() {
  return (
    <div style={{ padding: 12, fontFamily: 'ui-sans-serif, system-ui, sans-serif' }}>
      <header style={{ marginBottom: 12 }}>
        <nav style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          <Link to="/">Infer</Link>
          <Link to="/health">Health</Link>
          <Link to="/ready">Ready</Link>
          <Link to="/models">Models</Link>
          <Link to="/status">Status</Link>
        </nav>
      </header>
      <main>
        <Suspense fallback={<div>Loading...</div>}>
          <Routes>
            <Route path="/" element={<InferPage />} />
            <Route path="/infer" element={<Navigate to="/" replace />} />
            <Route path="/health" element={<HealthPage />} />
            <Route path="/ready" element={<ReadyPage />} />
            <Route path="/models" element={<ModelsPage />} />
            <Route path="/status" element={<StatusPage />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </Suspense>
      </main>
    </div>
  )
}

