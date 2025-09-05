/// <reference types="cypress" />

describe('Ready flow', () => {
  it('readyz transitions after first infer (best-effort)', () => {
    const readyUrl = Cypress.env('API_READY_URL')
    if (!readyUrl) {
      cy.log('Skipping readyz check (no API_READY_URL)')
      return
    }

    // expect possibly 503 before
    cy.request({ url: readyUrl, failOnStatusCode: false }).then((r) => {
      expect([200, 503]).to.include(r.status)
    })

    // trigger one infer
    cy.visit('/')
    cy.get('[data-testid="prompt-input"]').clear().type('Hello')
    cy.get('[data-testid="submit-btn"]').click()
    cy.get('[data-testid="status"]').should('have.text', 'success')

    // expect 200 after
    cy.request({ url: readyUrl, failOnStatusCode: false }).its('status').should('eq', 200)
  })
})
