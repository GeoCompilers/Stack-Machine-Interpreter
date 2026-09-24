// Materialize portable test files from the reviewed fixture catalogues.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const root = path.resolve(__dirname, '../tests/testdata');
const mode = process.argv[2];
assert(['--check', '--write'].includes(mode), 'Usage: node scripts/fixture-files.cjs --check|--write');
const cases = ['compiled.json', 'machine.json'].flatMap(file =>
  JSON.parse(fs.readFileSync(path.join(root, file), 'utf8')));
const names = new Set();
for (const test of cases) {
  assert(/^[a-z][a-z0-9_]*$/.test(test.name) && !names.has(test.name), `Invalid/duplicate name: ${test.name}`);
  names.add(test.name);
  const directory = path.join(root, 'cases', test.name);
  if (mode === '--write') fs.mkdirSync(directory, {recursive: true});
  const files = {
    'code.json': test.raw_program === undefined ? JSON.stringify(test.program, null, 2) + '\n' : test.raw_program,
    'input.txt': test.input,
    'output.txt': test.output === '' ? '' : test.output + '\n',
  };
  for (const [filename, expected] of Object.entries(files)) {
    const destination = path.join(directory, filename);
    if (mode === '--write') fs.writeFileSync(destination, expected);
    else assert.equal(fs.readFileSync(destination, 'utf8'), expected, `${test.name}/${filename} differs from catalogue`);
  }
}
assert.deepEqual(fs.readdirSync(path.join(root, 'cases')).sort(), [...names].sort(), 'Unexpected case directories');
console.log(`${cases.length} test directories, each with code.json, input.txt and output.txt (${mode}).`);
