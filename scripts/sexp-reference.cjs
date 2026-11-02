const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const {spawnSync} = require('node:child_process');

if (!process.execArgv.includes('--stack-size=4096')) {
  const child = spawnSync(process.execPath, ['--stack-size=4096', __filename, ...process.argv.slice(2)], {stdio: 'inherit'});
  if (child.error) throw child.error;
  process.exit(child.status ?? 1);
}

const mode = process.argv[2];
assert(['--record', '--check'].includes(mode) && process.argv[3],
  'Usage: node scripts/sexp-reference.cjs --record|--check /path/to/L0.js');
const bundle = fs.readFileSync(process.argv[3], 'utf8');
const sha256 = crypto.createHash('sha256').update(bundle).digest('hex');
assert.equal(sha256, '9536eb3ca339a92125f8cc4ddf02622391c973bcb09d60e26546c329b9254cb4',
  'Reference version changed; inspect before updating the hash and fixtures');

function run(mode, source) {
  const elements = Object.fromEntries(['title', 'input', 'output', 'message', 'args', 'res', 'parse'].map(id => [id, {
    tagName: id === 'title' ? 'TITLE' : id === 'parse' ? 'BUTTON' : 'TEXTAREA', value: '',
  }]));
  elements.title.text = mode === 'sm' ? 'Abstract Machine Compiler/Interpreter' : 'Simple Imperative Language Reference Interpreter';
  elements.input.value = source;
  elements.args.value = '0';
  const context = vm.createContext({document: {getElementById: id => elements[id]}, console, TextEncoder, TextDecoder});
  vm.runInContext(bundle, context, {timeout: 5000});
  vm.runInContext("document.getElementById('parse').onclick({preventDefault(){}})", context, {timeout: 5000});
  return {program: JSON.parse(elements.output.value), output: elements.res.value, message: elements.message.value};
}

// Explicit output overrides preserve the project's existing array and WRITE
// separators and unquoted strings. Constructor arguments and list elements use
// commas as on the site.
const sources = [
  ['pair', '{x=Pair(1,2);write(x);write(x[0]);write(x[1]);}', 'Pair (1, 2); 1; 2'],
  ['nullary', '{write(Leaf);}'],
  ['unary', '{write(Box(42));}'],
  ['nested', '{write(Tree(Pair(1,2),Leaf,Box(3)));}'],
  ['empty_list', '{write({});}'],
  ['singleton', '{write({1});}'],
  ['list', '{write({1,2,3,4});}'],
  ['nested_lists', '{write({{1,2},{},{3}});}'],
  ['list_constructors', '{write({Pair(1,2),Leaf,{3,4}});}'],
  ['capitalized_names', '{write(Cons(1,Nil));}'],
  ['colon', '{write(1:2:{});}'],
  ['prepend', '{xs={2,3};write(1:xs);}'],
  ['list_head', '{write({7,8,9}[0]);}'],
  ['list_tail', '{write({7,8,9}[1]);}'],
  ['nested_index', '{write(Box(Pair(4,5))[0][1]);}'],
  ['length', '{write(Pair(1,2).length);write(Leaf.length);write({1,2,3}.length);write({}.length);}', '2; 0; 2; 0'],
  ['mixed_arrays', '{write(Box([1,2],{3,4}));}', 'Box ([1; 2], {3, 4})'],
  ['sexp_in_array', '{write([Pair(1,2),{3,4}]);}', '[Pair (1, 2); {3, 4}]'],
  ['string_argument', '{write(Box("hello"));}', 'Box (hello)'],
  ['alias', '{x=Pair(1,2);y=x;write(x);y[0]=9;write(y);}', 'Pair (9, 2); Pair (9, 2)'],
  ['list_mutation', '{x={1,2,3};x[1][0]=8;write(x);}'],
  ['replace_tail', '{x={1};x[1]={2,3};write(x);}'],
  ['shared_tail', '{tail={2,3};write(Pair(1:tail,9:tail));}'],
  ['fresh_constructors', '{i=0;while(i<2){x=Pair(1,2);if(i==0)y=x;else x[0]=9;i+=1;}write(Pair(y,x));}'],
  ['argument_order', '{write(Pair(missing,other));}'],
  ['index_upper_bound', '{write(Pair(1,2)[2]);}'],
  ['index_negative', '{write(Pair(1,2)[0-1]);}'],
  ['index_nullary', '{write(Leaf[0]);}'],
  ['index_empty_list', '{write({}[0]);}'],
  ['index_type', '{write(Pair(1,2)[Leaf]);}'],
  ['set_bounds', '{x=Pair(1,2);x[2]=9;}'],
  ['condition_type', '{if(Leaf)write(1);}'],
  ['comparison', '{write(Pair(1,2)==Pair(1,2));write(A<B);write(A(1,2)<A(9));}', '1; 1; 0'],
  ['compare_list', '{write({1,2}==1:2:{});}'],
  ['compare_nested', '{write(Box({1,2})<Box({1,3}));}'],
  ['compare_incompatible', '{write(A==1);}'],
];

const cases = sources.map(([name, source, output]) => {
  const direct = run('eval', source);
  const sm = run('sm', source);
  return {name, source, program: direct.program, instructions: sm.program, input: '0',
    output: output ?? sm.output, message: sm.message,
    eval: {output: direct.output, message: direct.message},
    sm: {output: sm.output, message: sm.message}};
});
const result = {reference: {url: 'https://dboulytchev.github.io/eval.html',
  machine_url: 'https://dboulytchev.github.io/sm.html', bundle_url: 'https://dboulytchev.github.io/L0.js', sha256}, cases};
const filename = path.resolve(__dirname, '../tests/testdata/sexp_reference.json');
if (mode === '--record') fs.writeFileSync(filename, JSON.stringify(result, null, 2) + '\n');
else assert.deepEqual(JSON.parse(fs.readFileSync(filename, 'utf8')), result);
console.log(`${cases.length} S-expression cases (${mode}); L0.js SHA-256 ${sha256}`);
