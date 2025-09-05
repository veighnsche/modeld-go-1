/// <reference types="cypress" />

Cypress.on('window:before:load', (win) => {
  const errors: string[] = []
  win.addEventListener('error', (ev) => {
    errors.push(String(ev.error || ev.message))
  })
  // capture console errors
  const origError = win.console.error
  win.console.error = (...args: any[]) => {
    errors.push(args.map(a => (typeof a === 'string' ? a : JSON.stringify(a))).join(' '))
    origError.apply(win.console, args)
  }
  ;(win as any).__consoleErrors = errors
})

Cypress.Commands.add('healthCheck', (url: string, timeoutMs = 60000) => {
  const started = Date.now()
  function once() {
    return cy.request({ url, failOnStatusCode: false }).then((res) => res.status)
  }
  function loop() {
    return once().then((status) => {
      if (status === 200) return
      const elapsed = Date.now() - started
      if (elapsed > timeoutMs) throw new Error(`healthCheck timeout: ${url}`)
      return cy.wait(500).then(loop)
    })
  }
  return loop()
})

// Save artifacts on failure (best-effort)
afterEach(() => {
  const runner: any = (Cypress as any).mocha?.getRunner?.()
  const testState = runner?.test?.state || (Cypress as any).state?.('test')?.state
  if (testState === 'failed') {
    cy.get('[data-testid="result-json"]').then(($el) => {
      const text = $el.text()
      cy.task('saveText', { filename: `result-json-${Date.now()}.txt`, text })
    })
  }
})

export {}
