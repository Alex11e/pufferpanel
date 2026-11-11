import { test } from 'node:test'
import assert from 'node:assert/strict'
import { ServerApi } from './servers.js'

test('create-only uploads use the exclusive endpoint and handle old nodes and conflicts', async () => {
  const calls = []
  let status = 204
  const api = new ServerApi({
    put: async (url, data, params, headers, options) => {
      calls.push({ url, options })
      if (status !== 204) return options.onError({ response: { status } })
      return { status }
    }
  })
  await api.uploadFile('id', 'folder/file name', 'contents', undefined, true)
  assert.equal(calls[0].url, '/api/servers/id/file-create/folder/file%20name')
  status = 409
  await assert.rejects(api.uploadFile('id', 'file', '', undefined, true), { code: 'FILE_EXISTS' })
  status = 404
  await assert.rejects(api.uploadFile('id', 'file', '', undefined, true), { code: 'CREATE_UNAVAILABLE' })
  status = 204
  await api.uploadFile('id', 'file', 'updated')
  assert.equal(calls[3].url, '/api/servers/id/file/file')
})
