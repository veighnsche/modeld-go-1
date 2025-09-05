/// <reference types="cypress" />

// Preflight checks that must pass before running the rest of the suite.
// This spec is named with a leading 00_ to ensure it runs first.

describe('Preflight checks', () => {
  it('UI loads and basic elements exist', () => {
    const baseUrl = Cypress.config('baseUrl') || 'http://localhost:5173'
    // Basic reachability
    cy.request({ url: baseUrl, failOnStatusCode: false }).its('status').should('eq', 200)

    cy.visit('/')

    // Prompt input present
    cy.get('[data-testid="prompt-input"]').should('exist')
    // Submit button present
    cy.get('[data-testid="submit-btn"]').should('exist')
  })

  it('verifies API health endpoints when provided (best-effort)', () => {
    const health = Cypress.env('API_HEALTH_URL') as string | undefined
    const ready = Cypress.env('API_READY_URL') as string | undefined
    const status = Cypress.env('API_STATUS_URL') as string | undefined

    if (health) {
      cy.request({ url: health, failOnStatusCode: false }).then((res) => {
        expect([200]).to.include(res.status)
      })
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
