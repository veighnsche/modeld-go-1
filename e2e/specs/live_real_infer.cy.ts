/// <reference types="cypress" />

// Live-only Cypress spec that runs against a real API and real local models.
// It will be automatically skipped when USE_MOCKS is enabled.

describe('Live API - real infer + status', () => {
  const isMock = Boolean(Cypress.env('USE_MOCKS'))
  const STATUS_URL = Cypress.env('API_STATUS_URL') as string | undefined
  const READY_URL = Cypress.env('API_READY_URL') as string | undefined
  // Derive MODELS_URL from STATUS_URL if not explicitly provided
  const MODELS_URL = (STATUS_URL ? STATUS_URL.replace(/\/?status$/i, '/models') : undefined) as string | undefined

  if (isMock) {
    it('skips in mock mode', () => {
      cy.log('Skipping live test because USE_MOCKS is true')
    })
    return
  }

  it('performs an infer via the UI and validates /status', () => {
    // 1) Ready check (best-effort)
    if (READY_URL) {
      cy.request({ url: READY_URL, failOnStatusCode: false }).then((res) => {
        // Allow initial 503 prior to first infer
        expect([200, 503]).to.include(res.status)
      })
    }

    // 2) Fetch models from API and pick one (store as Cypress alias to enforce ordering)
    if (MODELS_URL) {
      cy.request({ url: MODELS_URL, failOnStatusCode: false }).then((res) => {
        expect(res.status).to.eq(200)
        const body = res.body
        let models: any[] = []
        if (Array.isArray(body)) models = body
        else if (body && Array.isArray(body.models)) models = body.models
        // Coerce to string IDs if objects are returned
        const ids = models.map((m: any) => (m && typeof m === 'object') ? (m.id || m.ID || m.name || '') : String(m))
        const valid = ids.filter((s: string) => typeof s === 'string' && s.length > 0)
        expect(valid.length, 'models available').to.be.greaterThan(0)
        const first = valid[0]
        cy.wrap(first).as('chosenModel')
      })
    }

    // 3) Drive the UI
    cy.visit('/')
    cy.get('[data-testid="mode"]').should('have.text', 'live')
    cy.get('[data-testid="prompt-input"]').clear().type('What is 2 + 2?')
    if (MODELS_URL) {
      cy.get<string>('@chosenModel').then((m) => {
        if (m && m.length > 0) {
          cy.get('[data-testid="model-input"]').clear().type(m)
        }
      })
    }
    cy.get('[data-testid="submit-btn"]').click()

    // 4) Expect success with a generous timeout for real LLMs
    cy.get('[data-testid="status"]', { timeout: 20000 }).should('have.text', 'success')

    // 5) Result JSON should contain some meaningful content
    cy.get('[data-testid="result-json"]').invoke('text').then((text) => {
      expect(text.length).to.be.greaterThan(0)
    })

    // 6) Optionally validate /status shape if provided
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
