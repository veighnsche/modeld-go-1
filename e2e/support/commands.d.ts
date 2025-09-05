/// <reference types="cypress" />

declare global {
  namespace Cypress {
    interface Chainable {
      healthCheck(url: string, timeoutMs?: number): Chainable<void>
    }
  }
}

export {}
