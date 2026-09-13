import assert from 'node:assert/strict'
import test from 'node:test'

import { isUploadResponse, uploadTorrents } from './client.ts'

test('accepts a structured single-file response', () => {
  assert.equal(isUploadResponse({
    results: [{ filename: 'example.torrent', status: 'saved', error: 'session did not start' }],
  }), true)
})

test('preserves structured multi-file results', () => {
  const payload = {
    results: [
      { filename: 'one.torrent', status: 'started', session_id: 'session-id' },
      { filename: 'two.torrent', status: 'error', error: 'invalid torrent' },
    ],
  }

  assert.equal(isUploadResponse(payload), true)
})

test('rejects legacy and unrelated payloads', () => {
  assert.equal(isUploadResponse({ status: 'ok' }), false)
  assert.equal(isUploadResponse({ id: 'id', name: 'name', info_hash: 'hash' }), false)
  assert.equal(isUploadResponse({ results: [{ filename: 'bad.torrent', status: 'unknown' }] }), false)
})

test('returns structured per-file errors even for a non-success HTTP status', async () => {
  const payload = {
    results: [{ filename: 'bad.torrent', status: 'error', error: 'invalid torrent' }],
  }
  const originalFetch = globalThis.fetch
  globalThis.fetch = async () => new Response(JSON.stringify(payload), {
    status: 400,
    headers: { 'Content-Type': 'application/json' },
  })
  try {
    const response = await uploadTorrents([new Blob(['invalid'])])
    assert.deepEqual(response, payload)
  } finally {
    globalThis.fetch = originalFetch
  }
})
