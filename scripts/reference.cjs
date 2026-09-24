// Run the original page handler with a minimal DOM, without modifying L0.js.
// Node.js is needed only for regenerating/checking reference fixtures.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const root = path.resolve(__dirname, '..');
const bundle = fs.readFileSync(path.join(root, 'dboulytchev.github.io-main/L0.js'), 'utf8');
const titles = {
  eval: 'Simple Imperative Language Reference Interpreter',
  sm: 'Abstract Machine Compiler/Interpreter',
};

function run(mode, source, input) {
  const elements = Object.fromEntries(
    ['title', 'input', 'output', 'message', 'args', 'res', 'parse'].map(id => [id, {
      tagName: id === 'title' ? 'TITLE' : id === 'parse' ? 'BUTTON' : 'TEXTAREA',
      value: '',
    }]),
  );
  elements.title.text = titles[mode];
  elements.input.value = source;
  elements.args.value = input;
  const context = vm.createContext({document: {getElementById: id => elements[id]}, console});
  vm.runInContext(bundle, context, {timeout: 5000});
  vm.runInContext("document.getElementById('parse').onclick({preventDefault(){}})", context, {timeout: 5000});
  return {output: elements.res.value, error: elements.message.value, program: elements.output.value};
}

function normalize(output) {
  return output.trim().split(';').map(x => x.trim()).join(';');
}

function main() {
  const mode = process.argv[2];
  assert(['--check', '--update'].includes(mode), 'Usage: node scripts/reference.cjs --check|--update');
  const sources = JSON.parse(fs.readFileSync(path.join(root, 'tests/testdata/sources.json'), 'utf8'));
  const compiled = sources.map(test => {
    const results = ['eval', 'sm'].map(mode => run(mode, test.source, test.input));
    for (const [i, result] of results.entries()) {
      assert.equal(result.error, test.reference_error || '', `${test.name}: ${['eval', 'sm'][i]} error`);
      assert.equal(normalize(result.output), test.output, `${test.name}: ${['eval', 'sm'][i]} output`);
    }
    const result = {
      name: test.name,
      source: test.source,
      program: JSON.parse(results[1].program),
      input: test.input,
      output: test.output,
    };
    if (test.error) result.error = test.error;
    return result;
  });
  const destination = path.join(root, 'tests/testdata/compiled.json');
  if (mode === '--update') {
    fs.writeFileSync(destination, JSON.stringify(compiled, null, 2) + '\n');
  } else {
    assert.deepEqual(JSON.parse(fs.readFileSync(destination, 'utf8')), compiled,
      'compiled.json differs from L0.js; inspect before running --update');
  }
  console.log(`${compiled.length} source cases checked against both eval.html and sm.html (${mode}).`);
}

try { main(); } catch (error) { console.error(error); process.exitCode = 1; }
