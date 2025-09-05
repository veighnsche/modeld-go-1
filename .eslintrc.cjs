/* eslint-disable */
module.exports = {
  root: true,
  env: { browser: true, es2022: true, node: true },
  parser: "@typescript-eslint/parser",
  parserOptions: {
    ecmaVersion: 2022,
    sourceType: "module",
    ecmaFeatures: { jsx: true },
    project: false
  },
  settings: {
    react: { version: "detect" }
  },
  plugins: ["@typescript-eslint", "react", "react-hooks", "prettier"],
  extends: [
    "eslint:recommended",
    "plugin:@typescript-eslint/recommended",
    "plugin:react/recommended",
    "plugin:react-hooks/recommended",
    "plugin:prettier/recommended"
  ],
  rules: {
    "prettier/prettier": ["error"],
    // Reasonable defaults for TS/React
    "react/react-in-jsx-scope": "off",
    "react/prop-types": "off"
  },
  overrides: [
    {
      files: ["web/**/*.{ts,tsx}", "web/**/*.js"],
      env: { browser: true, node: false },
      rules: {}
    },
    {
      files: ["e2e/**/*.ts"],
      env: { browser: true, node: true, "cypress/globals": true },
      plugins: ["cypress"],
      extends: ["plugin:cypress/recommended"],
      rules: {}
    }
  ],
  ignorePatterns: [
    "node_modules/",
    "web/dist/",
    "e2e/artifacts/",
    "e2e/screenshots/",
    "e2e/videos/",
    "e2e/downloads/"
  ]
}
