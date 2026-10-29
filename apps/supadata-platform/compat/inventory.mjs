import { readFileSync, writeFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import ts from 'typescript'

const root = path.dirname(fileURLToPath(import.meta.url))
const input = path.join(root, 'inventory-surfaces.ts')
const program = ts.createProgram([input], {
  module: ts.ModuleKind.NodeNext,
  moduleResolution: ts.ModuleResolutionKind.NodeNext,
  target: ts.ScriptTarget.ESNext,
  skipLibCheck: true,
  noEmit: true,
})
const diagnostics = ts.getPreEmitDiagnostics(program).filter((d) => d.file?.fileName === input)
if (diagnostics.length)
  throw new Error(
    ts.formatDiagnosticsWithColorAndContext(diagnostics, {
      getCanonicalFileName: (name) => name,
      getCurrentDirectory: () => root,
      getNewLine: () => '\n',
    })
  )
const checker = program.getTypeChecker()
const source = program.getSourceFile(input)
let declaration
ts.forEachChild(source, (node) => {
  if (ts.isVariableStatement(node))
    declaration =
      node.declarationList.declarations.find((d) => d.name.getText(source) === 'surfaces') ??
      declaration
})
if (!declaration) throw new Error('Missing SDK surfaces declaration')
const result = []
for (const surface of checker.getPropertiesOfType(checker.getTypeAtLocation(declaration.name))) {
  const surfaceType = checker.getTypeOfSymbolAtLocation(surface, surface.valueDeclaration)
  for (const method of checker.getPropertiesOfType(surfaceType)) {
    const declarations = method.getDeclarations() ?? []
    if (
      method.name.startsWith('_') ||
      declarations.some(
        (d) =>
          ts.getCombinedModifierFlags(d) & (ts.ModifierFlags.Private | ts.ModifierFlags.Protected)
      )
    )
      continue
    if (method.getJsDocTags(checker).some((tag) => tag.name === 'internal')) continue
    const type = checker.getTypeOfSymbolAtLocation(
      method,
      declarations[0] ?? surface.valueDeclaration
    )
    if (checker.getSignaturesOfType(type, ts.SignatureKind.Call).length === 0) continue
    result.push({
      id: `${surface.name}.${method.name}`,
      surface: surface.name,
      method: method.name,
      status: 'UNVERIFIED',
    })
  }
}
result.sort((a, b) => a.id.localeCompare(b.id))
const version = JSON.parse(
  readFileSync(path.join(root, 'node_modules/@supabase/supabase-js/package.json'), 'utf8')
).version
writeFileSync(
  path.join(root, 'sdk-inventory.json'),
  JSON.stringify(
    {
      sdk: '@supabase/supabase-js',
      version,
      generatedBy: 'node inventory.mjs',
      note: 'Public callable surfaces from the pinned SDK declarations. Method coverage alone does not establish behavioral parity. Status is UNVERIFIED until linked to local and live contract evidence.',
      capabilities: result,
    },
    null,
    2
  ) + '\n'
)
console.log(
  `Inventoried ${result.length} callable SDK surfaces across ${new Set(result.map((entry) => entry.surface)).size} areas for @supabase/supabase-js ${version}.`
)
