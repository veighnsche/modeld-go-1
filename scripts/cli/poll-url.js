#!/usr/bin/env node
const http = require('http');
const https = require('https');
const { URL } = require('url');

const urlStr = process.argv[2];
const expected = parseInt(process.argv[3] || '200', 10);
const timeoutSec = parseInt(process.argv[4] || '60', 10);

if (!urlStr) {
  console.error('Usage: node scripts/poll-url.js <url> [expectedStatus=200] [timeoutSec=60]');
  process.exit(2);
}

const url = new URL(urlStr);
const client = url.protocol === 'https:' ? https : http;

const started = Date.now();

function once() {
  return new Promise((resolve) => {
    const req = client.request(
      {
        method: 'GET',
        hostname: url.hostname,
        port: url.port || (url.protocol === 'https:' ? 443 : 80),
        path: url.pathname + url.search,
        timeout: 5000,
      },
      (res) => {
        res.resume();
        resolve(res.statusCode);
      }
    );
    req.on('error', () => resolve(0));
    req.on('timeout', () => {
      req.destroy();
      resolve(0);
    });
    req.end();
  });
}

(async () => {
  while (Date.now() - started < timeoutSec * 1000) {
    const status = await once();
    if (status === expected) {
      console.log(`OK ${urlStr} -> ${status}`);
      process.exit(0);
    }
    await new Promise((r) => setTimeout(r, 500));
  }
  console.error(`TIMEOUT waiting for ${urlStr} to become ${expected}`);
  process.exit(1);
})();
