const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const {spawnSync} = require('node:child_process');

const filename = path.resolve(__dirname, '../docs/reference-observations.json');
const mode = process.argv[2];
assert(['--check', '--record'].includes(mode),
  'Usage: node scripts/probe-reference.cjs --check|--record [case-name]');
const catalogue = JSON.parse(fs.readFileSync(filename, 'utf8'));
const selected = catalogue.cases.filter(test => !process.argv[3] || test.name === process.argv[3]);
assert(selected.length, 'No matching cases');
let count = 0;
for (const test of selected) {
  const observations = {};
  for (const referenceMode of test.modes) {
    // Pass arguments directly: the supplied run.sh loses quoting in ARGS.
    const flags = referenceMode === 'ast' ? [] : ['-' + referenceMode];
    const result = spawnSync('docker', [
      'run', '--rm', '-i', '--platform', catalogue.platform,
      catalogue.image, './L0', '.', '-input', test.input, ...flags,
    ], {input: test.source, encoding: 'utf8', timeout: 15000, maxBuffer: 1024 * 1024});
    if (result.error) throw result.error;
    const actual = {status: result.status, stdout: result.stdout, stderr: result.stderr};
    assert.equal(actual.status, 0, `${test.name}/${referenceMode}: Docker/reference failed: ${actual.stderr}`);
    if (mode === '--check') {
      assert.deepEqual(actual, test.observations[referenceMode], `${test.name}/${referenceMode}`);
    }
    observations[referenceMode] = actual;
    count++;
  }
  if (mode === '--record') test.observations = observations;
  console.log(`${test.name}: ${test.modes.join(', ')} (${mode})`);
}
if (mode === '--record') fs.writeFileSync(filename, JSON.stringify(catalogue, null, 2) + '\n');
console.log(`${selected.length} cases, ${count} reference invocations.`);
