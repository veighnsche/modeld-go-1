import { defineConfig } from 'cypress'
import * as fs from 'fs'
import * as path from 'path'

export default defineConfig({
  e2e: {
    specPattern: 'e2e/specs/**/*.cy.{ts,tsx}',
    baseUrl: process.env.CYPRESS_BASE_URL || 'http://localhost:5173',
    video: true,
    screenshotsFolder: 'e2e/artifacts/screenshots',
    videosFolder: 'e2e/artifacts/videos',
    fixturesFolder: 'e2e/fixtures',
    supportFile: 'e2e/support/e2e.ts',
    setupNodeEvents(on, config) {
      on('task', {
        saveText({ filename, text }: { filename: string, text: string }) {
          const dir = path.resolve('e2e/artifacts')
          fs.mkdirSync(dir, { recursive: true })
          fs.writeFileSync(path.join(dir, filename), text, 'utf8')
          return null
        },
      })
      return config
    },
  },
})
