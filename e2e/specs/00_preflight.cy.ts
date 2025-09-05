/// <reference types="cypress" />

// Preflight checks that must pass before running the rest of the suite.
// This spec is named with a leading 00_ to ensure it runs first.

describe('Preflight', () => {
  it('verifies UI base URL is reachable and core elements render', () => {
    const baseUrl = Cypress.config('baseUrl') || 'http://localhost:5173'
    // Basic reachability
    cy.request({ url: baseUrl, failOnStatusCode: false }).its('status').should('eq', 200)

    // App mounts and shows mode badge
    cy.visit('/')
    cy.get('[data-testid="mode"]').should('exist').and(($el) => {
      const txt = $el.text().trim().toLowerCase()
      expect(['mock', 'live']).to.include(txt)
    })

    // Prompt input present
    cy.get('[data-testid="prompt-input"]').should('exist')
  })

  it('verifies API health endpoints when provided (best-effort)', () => {
    const mockMode = Boolean(Cypress.env('USE_MOCKS'))
    const health = Cypress.env('API_HEALTH_URL') as string | undefined
    const ready = Cypress.env('API_READY_URL') as string | undefined
    const status = Cypress.env('API_STATUS_URL') as string | undefined

    if (mockMode) {
      cy.log('Mock mode enabled; skipping live API preflight checks')
      return
    }

    if (health) {
      // Wait until health is 200
      cy.healthCheck(health, 60000)
    }

    if (ready) {
      // Ready might be 503 prior to first infer; just ensure it responds (200 or 503)
      cy.request({ url: ready, failOnStatusCode: false }).its('status').should('be.oneOf', [200, 503])
    }

    if (status) {
      // Status should return JSON (may be partial prior to first infer)
      cy.request({ url: status, failOnStatusCode: false }).then((res) => {
        expect([200, 503]).to.include(res.status)
        const ct = String(res.headers['content-type'] || '')
        expect(ct).to.match(/application\/json/i)
        expect(res.body).to.be.an('object')
      })
    }
  })
})
