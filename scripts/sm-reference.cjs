const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const crypto = require('node:crypto');
const {spawnSync} = require('node:child_process');

if (!process.execArgv.includes('--stack-size=4096')) {
  const child = spawnSync(process.execPath, ['--stack-size=4096', __filename, ...process.argv.slice(2)], {stdio: 'inherit'});
  if (child.error) throw child.error;
  process.exit(child.status ?? 1);
}
const root = path.resolve(__dirname, '..');
const mode = process.argv[2];
assert(['--record', '--check'].includes(mode) && process.argv[3],
  'Usage: node scripts/sm-reference.cjs --record|--check /path/to/L0.js');
const bundle = fs.readFileSync(process.argv[3], 'utf8');
const sha256 = crypto.createHash('sha256').update(bundle).digest('hex');
assert.equal(sha256, 'eefbbb9aa7564c7dd647a14b4050d87e54b50f236e6ed9b5f38f1afe77d82a0c',
  'Reference version changed; inspect differences before changing the pinned hash');
const destination = path.join(root, 'tests/testdata/sm_reference.json');

function run(mode, source, input) {
  const elements = Object.fromEntries(['title', 'input', 'output', 'message', 'args', 'res', 'parse'].map(id => [id, {
    tagName: id === 'title' ? 'TITLE' : id === 'parse' ? 'BUTTON' : 'TEXTAREA', value: '',
  }]));
  elements.title.text = mode === 'sm' ? 'Abstract Machine Compiler/Interpreter' : 'Simple Imperative Language Reference Interpreter';
  elements.input.value = source;
  elements.args.value = input;
  const context = vm.createContext({document: {getElementById: id => elements[id]}, console, TextEncoder, TextDecoder});
  vm.runInContext(bundle, context, {timeout: 5000});
  vm.runInContext("document.getElementById('parse').onclick({preventDefault(){}})", context, {timeout: 5000});
  return {program: elements.output.value, output: elements.res.value, message: elements.message.value};
}

const old = JSON.parse(fs.readFileSync(path.join(root, 'tests/testdata/interpreter/reference.json'), 'utf8')).cases;
const extra = [
  ['input_string', '{read(s);write(s);write(s.length);write(s[0]);}', '"hello"'],
  ['input_empty_string', '{read(s);write(s);write(s.length);}', '""'],
  ['input_string_spaces', '{read(s);write(s);write(s.length);}', '" a,b "'],
  ['input_nested_mixed', '{read(a);a[1][0]=90;write(a);}', '[1,"abc",[2,3]]'],
  ['input_alias', '{read(a);b=a;write(a);b[0]=9;write(b);}', '[1,2]'],
  ['byte_bounds', '{read(s);s[0]=0;s[1]=255;write(s);write(s[0]);write(s[1]);}', '"ab"'],
  ['char_negative', '{read(s);s[0]=0-1;}', '"a"'],
  ['char_overflow', '{read(s);s[0]=256;}', '"a"'],
  ['string_bounds', '{read(s);write(s[2]);}', '"ab"'],
  ['string_negative_index', '{read(s);s[0-1]=1;}', '"ab"'],
  ['string_set_type', '{read(s);s[0]=[1];}', '"ab"'],
  ['set_bounds_before_type', '{read(s);s[2]=[1];}', '"ab"'],
  ['input_comparison', '{read(a);read(b);write(a<b);write(a==b);}', '"abc","abd"'],
  ['fresh_literals', '{read(s);i=0;while(i<2){t="abc";if(i==0){a=t;a[1]=88;}else write(t);i+=1;}write(a);}', '0'],
  ['fresh_arrays', '{i=0;while(i<2){a=[1];if(i==0)b=a;else a[0]=9;i+=1;}write(b);write(a);}', '0'],
  ['deferred_error', '{a=[1];write(a);a[0]=9;write(missing);}', '0'],
  ['binary_type_order', '{read(a);read(b);write(a+b);}', '"left","right"'],
  ['index_before_target_type', '{read(i);write(1[i]);}', '"x"'],
  ['reference_set_order', '{base[index]=value;}', '0'],
  ['input_unicode', '{read(s);write(s);write(s.length);write(s[0]);}', '"я🙂"'],
  ['length_type', '{read(x);write(x.length);}', '1'],
  ['condition_string', '{read(x);if(x)write(1);else write(0);}', '"abc"'],
  ['condition_array', '{if([1])write(1);else write(0);}', '0'],
  ['do_condition_type', '{read(x);do skip;while(x);}', '[1]'],
  ['array_size_order', '{write([1,2]>[99]);write([1,"x"]<[2,0]);}', '0'],
].map(([name, source, input]) => ({name: 'sm_' + name, source, input}));

const cases = [], rejected = [];
for (const candidate of [...old, ...extra]) {
  const ast = run('eval', candidate.source, '0');
  const code = run('sm', candidate.source, '0');
  if (!ast.program || !code.program) {
    rejected.push({name: candidate.name, source: candidate.source, message: ast.message || code.message});
    continue;
  }
  const sm = run('sm', candidate.source, candidate.input);
  const direct = run('eval', candidate.source, candidate.input);
  assert(!sm.message.startsWith('Input error:'), `${candidate.name}: reference rejects the chosen input`);
  cases.push({name: candidate.name, source: candidate.source, program: JSON.parse(ast.program),
    instructions: JSON.parse(code.program), input: candidate.input, output: sm.output, message: sm.message,
    eval: {output: direct.output, message: direct.message}});
}
const result = {reference: {url: 'https://dboulytchev.github.io/sm.html',
  bundle_url: 'https://dboulytchev.github.io/L0.js', sha256}, cases, rejected};
if (mode === '--record') fs.writeFileSync(destination, JSON.stringify(result, null, 2) + '\n');
else assert.deepEqual(JSON.parse(fs.readFileSync(destination, 'utf8')), result);
const differences = cases.filter(c => c.output !== c.eval.output || c.message !== c.eval.message);
console.log(`${cases.length} sm.html cases (${mode}); ${differences.length} eval/sm differences; ${rejected.length} rejected sources; SHA-256 ${sha256}`);
