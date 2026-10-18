const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const crypto = require('node:crypto');
const {spawnSync} = require('node:child_process');

// The reference's parser uses deep recursion even for short nested expressions.
if (!process.execArgv.includes('--stack-size=4096')) {
  const child = spawnSync(process.execPath, ['--stack-size=4096', __filename, ...process.argv.slice(2)], {stdio: 'inherit'});
  if (child.error) throw child.error;
  process.exit(child.status ?? 1);
}
const root = path.resolve(__dirname, '..');
const destination = path.join(root, 'tests/testdata/interpreter/reference.json');
const mode = process.argv[2];
assert(['--record', '--check'].includes(mode) && process.argv[3],
  'Usage: node scripts/interpreter-reference.cjs --record|--check /path/to/L0.js');
const bundle = fs.readFileSync(process.argv[3], 'utf8');
const sha256 = crypto.createHash('sha256').update(bundle).digest('hex');
const expectedHash = '6ca82c79572205be232413a9d310067736ba7cb835a4cf382261b9f0a9347343';
assert.equal(sha256, expectedHash, 'Reference version changed; inspect before updating its hash and fixtures');

function run(source, input) {
  const elements = Object.fromEntries(['title', 'input', 'output', 'message', 'args', 'res', 'parse'].map(id => [id, {
    tagName: id === 'title' ? 'TITLE' : id === 'parse' ? 'BUTTON' : 'TEXTAREA', value: '',
  }]));
  elements.title.text = 'Simple Imperative Language Reference Interpreter';
  elements.input.value = source;
  elements.args.value = input;
  // Supply browser encoders: the bundle's legacy fallback differs on BOMs.
  const context = vm.createContext({document: {getElementById: id => elements[id]}, console, TextDecoder, TextEncoder});
  vm.runInContext(bundle, context, {timeout: 5000});
  vm.runInContext("document.getElementById('parse').onclick({preventDefault(){}})", context, {timeout: 5000});
  return {program: elements.output.value, output: elements.res.value, message: elements.message.value};
}

let catalogue;
if (mode === '--record') {
  const sources = JSON.parse(fs.readFileSync(path.join(root, 'tests/testdata/sources.json'), 'utf8'));
  const probes = JSON.parse(fs.readFileSync(path.join(root, 'docs/reference-observations.json'), 'utf8')).cases;
  const extra = [
    ['invalid_utf8', '{s="a";s[0]=255;write(s);}', '0'],
    ['truncated_utf8', '{s="ab";s[0]=226;s[1]=130;write(s);}', '0'],
    ['invalid_utf8_sequence', '{s="abc";s[0]=224;s[1]=128;s[2]=128;write(s);}', '0'],
    ['bom', '{write("﻿hello");}', '0'],
    ['bom_after_output', '{write(1);write("﻿hello");}', '0'],
    ['bom_in_diagnostic', '{if("﻿hello")skip;}', '0'],
    ['consecutive_invalid_bytes', '{s="ab";s[0]=255;s[1]=255;write(s);}', '0'],
    ['nul_byte', '{s="a";s[0]=0;write(s);}', '0'],
    ['get_integer_before_index_type', '{write(1["x"]);}', '0'],
    ['get_base_error_before_index_type', '{write(base["x"]);}', '0'],
    ['set_integer_target', '{a=1;a[0]=2;}', '0'],
    ['set_index_type', '{a=[1];a["x"]=2;}', '0'],
    ['binary_left_eval_before_right_type', '{write(missing+"x");}', '0'],
    ['all_comparisons', '{write(1<=1);write(2>=3);write("b">"a");write([2]!=[3]);write([2]<=[3]);}', '0'],
    ['mismatched_ne', '{write(1!="a");}', '0'],
    ['comparison_stops_early', '{write([1,"a"]<[2,0]);}', '0'],
    ['array_compare_length', '{write([1,2]>[9]);}', '0'],
    ['string_mod_type', '{write(1%"x");}', '0'],
    ['utf8_string_order', '{write("я">"z");}', '0'],
    ['quote_in_diagnostic', '{write("a""b"+1);}', '0'],
    ['line_in_diagnostic', '{if("a\nb")skip;}', '0'],
    ['input_comments', '{read(a);read(b);write(a);write(b);}', '1, (* a (* b *) *) 2 -- end'],
  ].map(([name, source, input]) => ({name: 'extra_' + name, source, input}));
  const candidates = [
    ...sources.map(t => ({...t, name: 'source_' + t.name})),
    ...probes.map(t => ({...t, name: 'probe_' + t.name})), ...extra,
  ];
  const cases = [];
  for (const test of candidates) {
    if (test.input.includes('"')) continue;
    const input = test.input.trim() === '' ? '0' : test.input;
    const parsed = run(test.source, '0');
    if (!parsed.program) continue;
    const result = run(test.source, input);
    if (result.message.startsWith('Input error:')) continue;
    cases.push({name: test.name, source: test.source, program: JSON.parse(parsed.program),
      input, output: result.output, message: result.message});
  }
  catalogue = {reference: {url: 'https://dboulytchev.github.io/eval.html',
    bundle_url: 'https://dboulytchev.github.io/L0.js', sha256}, cases};
  fs.writeFileSync(destination, JSON.stringify(catalogue, null, 2) + '\n');
} else {
  catalogue = JSON.parse(fs.readFileSync(destination, 'utf8'));
  assert.equal(catalogue.reference.sha256, sha256);
  for (const test of catalogue.cases) {
    const result = run(test.source, test.input);
    assert.deepEqual(JSON.parse(result.program), test.program, test.name + ': AST');
    assert.equal(result.output, test.output, test.name + ': Output');
    assert.equal(result.message, test.message, test.name + ': Message');
  }
}
console.log(`${catalogue.cases.length} eval.html cases (${mode}); L0.js SHA-256 ${sha256}`);
