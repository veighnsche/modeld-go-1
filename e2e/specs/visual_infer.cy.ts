/// <reference types="cypress" />

describe('Visual Harness Infer Flow', () => {
  it('streams and completes', () => {
    cy.visit('/')
    const healthUrl = Cypress.env('API_HEALTH_URL') || `${Cypress.config('baseUrl')}/healthz`
    cy.healthCheck(healthUrl, 60000)

    // Type prompt and optional model
    cy.get('[data-testid="prompt-input"]').clear().type('Hello')
    cy.get('[data-testid="model-input"]').clear()

    // Send
    cy.get('[data-testid="submit-btn"]').click()

    // Status flows
    cy.get('[data-testid="status"]').should('have.text', 'requesting')

    // Expect at least 2 lines and final indicates completion (contains done: true)
    cy.get('[data-testid="stream-log"] div').should('have.length.at.least', 2)
    cy.get('[data-testid="stream-log"] div').last().invoke('text').should((txt) => {
      expect(/\"done\"\s*:\s*true/.test(txt)).to.be.true
    })

    // result-json parseable and non-empty
    cy.get('[data-testid="result-json"]').invoke('text').then((t) => {
      expect(t.trim().length).to.be.greaterThan(0)
      expect(() => JSON.parse(t)).to.not.throw()
    })

    // success status
    cy.get('[data-testid="status"]').should('have.text', 'success')

    // latency under threshold
    const maxMs = Number(Cypress.env('MAX_LATENCY_MS')) || 5000
    cy.get('[data-testid="latency-ms"]').invoke('text').then((t) => {
      const v = Number(t)
      expect(v).to.be.greaterThan(0)
      expect(v).to.be.lessThan(maxMs)
    })

    // no console errors
    cy.window().then((win: any) => {
      const errs = win.__consoleErrors || []
      expect(errs, `console errors: ${errs.join('\n')}`).to.have.length(0)
    })
  })
})
