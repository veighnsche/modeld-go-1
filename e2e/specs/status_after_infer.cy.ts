/// <reference types="cypress" />

describe('Status after infer', () => {
  it('validates /status JSON after a successful infer (best-effort)', () => {
    const statusUrl = Cypress.env('API_STATUS_URL')
    const mockMode = Boolean(Cypress.env('USE_MOCKS'))

    // Trigger an infer via the UI
    cy.visit('/')
    cy.get('[data-testid="prompt-input"]').clear().type('Hello')
    cy.get('[data-testid="submit-btn"]').click()
    cy.get('[data-testid="status"]').should('have.text', 'success')

    // If API_STATUS_URL is provided and we're not in mock mode, assert JSON shape
    if (!statusUrl || mockMode) {
      cy.log('Skipping /status validation (no API_STATUS_URL or mock mode)')
      return
    }

    cy.request({ url: statusUrl, failOnStatusCode: false }).then((res) => {
      expect(res.status).to.eq(200)
      // Should be JSON
      const ct = String(res.headers['content-type'] || '')
      expect(ct).to.match(/application\/json/i)
      // Parse body and check it is an object with plausible keys
      const body = res.body
      expect(body).to.be.an('object')
      // If Instances is present and array, expect length >= 1 after infer
      if (Array.isArray(body.Instances)) {
        expect(body.Instances.length).to.be.greaterThan(0)
      }
    })
  })
})
