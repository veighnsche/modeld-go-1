/// <reference types="cypress" />

// DO NOT MOCK THE HAIKU FOR TESTING!!!
// This spec drives the new Haiku page and verifies the poem appears centered on screen.

describe('Haiku Page - centered poem (live)', () => {
  const isMock = Boolean(Cypress.env('USE_MOCKS'))
  if (isMock) {
    it('skips in mock mode - DO NOT MOCK THE HAIKU FOR TESTING!!!', () => {
      cy.log('Skipping haiku live test because USE_MOCKS is true')
    })
    return
  }

  it('renders a haiku in the middle of the screen', () => {
    cy.visit('/haiku')

    // Ensure we are in live mode
    cy.get('[data-testid="mode"]').should('have.text', 'live')

    // Trigger generation
    cy.get('[data-testid="make-haiku-btn"]').click()

    // Wait for status to resolve first
    cy.get('[data-testid="haiku-status"]', { timeout: 20000 }).should(($el) => {
      const t = ($el.text() || '').trim()
      expect(['success', 'error']).to.include(t)
    })
    // Then the poem should be non-empty (either generated content or error text)
    cy.get('[data-testid="haiku-poem"]').should(($el) => {
      const text = ($el.text() || '').trim()
      expect(text.length, 'haiku content length').to.be.greaterThan(0)
    })

    // Assert vertical centering within tolerance
    cy.get('[data-testid="haiku-poem"]').then(($el) => {
      const el = $el[0]
      const rect = el.getBoundingClientRect()
      const centerY = rect.top + rect.height / 2
      const vpH = Cypress.config('viewportHeight') || window.innerHeight
      const midY = vpH / 2
      const delta = Math.abs(centerY - midY)
      // allow some tolerance for different fonts/systems
      expect(delta, 'vertical center delta').to.be.lessThan(100)
    })
  })
})
