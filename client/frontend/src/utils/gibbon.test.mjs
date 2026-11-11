import { test } from 'node:test'
import assert from 'node:assert/strict'
import { prepareGibbonDefinition } from './gibbon.mjs'

test('database linking can set its variables without conflicts or changing the template', () => {
  const template = {
    data: { db_password: { value: 'secret', internal: true } },
    run: {
      environmentVars: {
        WEB_DOCUMENT_ROOT: '/var/www/html/core-31.0.00',
        DB_HOST: '${db_host}',
        DB_PORT: '${db_port}',
        DB_NAME: '${db_name}',
        DB_USERNAME: '${db_user}',
        DB_PASSWORD: '${db_password}'
      }
    }
  }
  const definition = prepareGibbonDefinition(template)
  assert.deepEqual(definition.run.environmentVars, { WEB_DOCUMENT_ROOT: '/var/www/html/core-31.0.00' })
  assert.equal(template.run.environmentVars.DB_HOST, '${db_host}')
  assert.equal(definition.data.db_password.internal, true)
  assert.throws(() => prepareGibbonDefinition({ run: { environmentVars: {} } }), /DB_HOST/)
})
