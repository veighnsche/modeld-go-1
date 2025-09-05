/// <reference types="cypress" />

// DO NOT MOCK THE HAIKU FOR TESTING!!!
// This spec intentionally drives the live UI and hits the real backend.
// If the test environment is configured for mocks, we will skip the test entirely.
// DO NOT MOCK THE HAIKU FOR TESTING!!!

describe('Haiku Maker - live web harness', () => {
  const isMock = Boolean(Cypress.env('USE_MOCKS'))
  const STATUS_URL = Cypress.env('API_STATUS_URL') as string | undefined
  const READY_URL = Cypress.env('API_READY_URL') as string | undefined
  const MODELS_URL = (STATUS_URL ? STATUS_URL.replace(/\/?status$/i, '/models') : undefined) as string | undefined

  // DO NOT MOCK THE HAIKU FOR TESTING!!!
  if (isMock) {
    it('skips in mock mode - DO NOT MOCK THE HAIKU FOR TESTING!!!', () => {
      cy.log('Skipping haiku live test because USE_MOCKS is true')
      cy.log('DO NOT MOCK THE HAIKU FOR TESTING!!!')
    })
    return
  }

  it('generates a real haiku via the UI against the live backend', () => {
    // 1) Best-effort ready check. DO NOT MOCK THE HAIKU FOR TESTING!!!
    if (READY_URL) {
      cy.request({ url: READY_URL, failOnStatusCode: false }).then((res) => {
        expect([200, 503]).to.include(res.status)
      })
    }

    // 2) Get available models from API (if URL provided) and choose one. DO NOT MOCK THE HAIKU FOR TESTING!!!
    if (MODELS_URL) {
      cy.request({ url: MODELS_URL, failOnStatusCode: false }).then((res) => {
        expect(res.status).to.eq(200)
        const body = res.body
        let models: any[] = []
        if (Array.isArray(body)) models = body
        else if (body && Array.isArray(body.models)) models = body.models
        const ids = models.map((m: any) => (m && typeof m === 'object') ? (m.id || m.ID || m.name || '') : String(m))
        const valid = ids.filter((s: string) => typeof s === 'string' && s.length > 0)
        expect(valid.length, 'models available').to.be.greaterThan(0)
        cy.wrap(valid[0]).as('chosenModel')
      })
    }

    // 3) Drive the UI. DO NOT MOCK THE HAIKU FOR TESTING!!!
    cy.visit('/')
    cy.get('[data-testid="mode"]').should('have.text', 'live')

    // Use a real prompt that asks for a haiku; do not assert exact text, only presence of output.
    const prompt = 'Write a haiku about the ocean in English.'
    cy.get('[data-testid="prompt-input"]').clear().type(prompt)

    if (MODELS_URL) {
      cy.get<string>('@chosenModel').then((m) => {
        if (m && m.length > 0) {
          cy.get('[data-testid="model-input"]').clear().type(m)
        }
      })
    }

    cy.get('[data-testid="submit-btn"]').click()

    // 4) Expect success with a generous timeout for real LLMs. DO NOT MOCK THE HAIKU FOR TESTING!!!
    cy.get('[data-testid="status"]', { timeout: 90000 }).should('have.text', 'success')

    // 5) Basic validations that we actually received output from the real backend.
    // Avoid strict shape assertions as implementations may vary. DO NOT MOCK THE HAIKU FOR TESTING!!!
    cy.get('[data-testid="result-json"]').invoke('text').then((text) => {
      expect(text.length, 'result length').to.be.greaterThan(0)
    })

    // Stream log should have at least one line received. DO NOT MOCK THE HAIKU FOR TESTING!!!
    cy.get('[data-testid="stream-log"] div').its('length').should('be.greaterThan', 0)

    // Latency should be a number (string that parses to number) and > 0.
    cy.get('[data-testid="latency-ms"]').invoke('text').then((ms) => {
      const n = Number(ms)
      expect(Number.isFinite(n)).to.eq(true)
      expect(n).to.be.greaterThan(0)
    })

    // 6) Optionally validate /status shape if provided (best-effort). DO NOT MOCK THE HAIKU FOR TESTING!!!
    if (STATUS_URL) {
      cy.request({ url: STATUS_URL, failOnStatusCode: false }).then((res) => {
        expect(res.status).to.eq(200)
        const body = res.body
        expect(body).to.be.an('object')
        if (Array.isArray(body.Instances)) {
          expect(body.Instances.length).to.be.greaterThan(0)
        }
      })
    }
  })
})
