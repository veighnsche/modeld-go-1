/// <reference types="cypress" />

describe('Models list', () => {
  it('renders models count if available', () => {
    cy.visit('/')
    cy.get('[data-testid="models-count"]').then(($el) => {
      // If present, should be a non-negative number
      if ($el.length) {
        const n = Number($el.text())
        expect(Number.isNaN(n)).to.be.false
        expect(n).to.be.greaterThan(-1)
      }
    })
  })
})
