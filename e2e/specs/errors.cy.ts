/// <reference types="cypress" />

describe('Error surfacing', () => {
  it('invalid model name triggers error path', () => {
    cy.visit('/')
    cy.get('[data-testid="prompt-input"]').clear().type('Say hello')
    cy.get('[data-testid="model-input"]').clear().type('unknown-model-id')
    cy.get('[data-testid="submit-btn"]').click()
    cy.get('[data-testid="status"]').should('have.text', 'error')
    cy.get('[data-testid="result-json"]').invoke('text').then((t) => {
      // Should contain a JSON blob with error info, but don't assert schema
      expect(t.trim().length).to.be.greaterThan(0)
    })
  })
})
