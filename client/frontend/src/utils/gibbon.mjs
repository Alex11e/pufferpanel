const databaseVariables = {
  DB_HOST: '${db_host}',
  DB_PORT: '${db_port}',
  DB_NAME: '${db_name}',
  DB_USERNAME: '${db_user}',
  DB_PASSWORD: '${db_password}'
}

export function prepareGibbonDefinition(template) {
  const definition = JSON.parse(JSON.stringify(template))
  const variables = definition.run?.environmentVars
  for (const [name, expected] of Object.entries(databaseVariables)) {
    if (!variables || variables[name] !== expected) {
      throw new Error(`Missing or unexpected Gibbon environment variable: ${name}`)
    }
    delete variables[name]
  }
  return definition
}
