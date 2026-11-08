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
const mode = process.argv[2];
assert(['--record', '--check'].includes(mode) && process.argv[3],
  'Usage: node scripts/language-reference.cjs --record|--check /path/to/L0.js');
const bundle = fs.readFileSync(process.argv[3], 'utf8');
const sha256 = crypto.createHash('sha256').update(bundle).digest('hex');
assert.equal(sha256, '9536eb3ca339a92125f8cc4ddf02622391c973bcb09d60e26546c329b9254cb4',
  'Reference version changed; inspect before updating the hash or fixtures');

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
  assert(elements.output.value, `${mode}: no JSON for ${source}: ${elements.message.value}`);
  return {program: JSON.parse(elements.output.value), output: elements.res.value, message: elements.message.value};
}

const switches = [
  ['wildcard', '{switch(1){case _ -> write(1);}}', '1'],
  ['integer', '{switch(2){case 1 -> write(0); | case 2 -> write(1);}}', '1'],
  ['negative', '{switch(0-1){case -1 -> write(1);}}', '1'],
  ['character', "{switch('a'){case 'a' -> write(1);}}", '1'],
  ['string', '{switch("abc"){case "abc" -> write(1);}}', '1'],
  ['empty_string', '{switch(""){case "" -> write(1);}}', '1'],
  ['first_branch', '{switch(1){case _ -> write(1); | case _ -> write(missing);}}', '1'],
  ['mismatch_type', '{switch("1"){case 1 -> write(0); | case _ -> write(1);}}', '1'],
  ['array', '{switch([1,2]){case [x,y] -> write(x+y);}}', '3'],
  ['empty_array', '{read(a);switch(a){case [] -> write(1);}}', '1', '', '[]'],
  ['array_size', '{switch([1,2]){case [x] -> write(0); | case _ -> write(1);}}', '1'],
  ['constructor', '{switch(Pair(1,2)){case Pair(x,y) -> write(x+y);}}', '3'],
  ['nullary', '{switch(Leaf){case Leaf -> write(1);}}', '1'],
  ['constructor_tag', '{switch(Pair(1,2)){case Box(x,y) -> write(0); | case _ -> write(1);}}', '1'],
  ['constructor_size', '{switch(Pair(1,2)){case Pair(x) -> write(0); | case _ -> write(1);}}', '1'],
  ['nested', '{switch(Box([Pair(1,2),3])){case Box([Pair(x,y),z]) -> write(x+y+z);}}', '6'],
  ['empty_list', '{switch({}){case {} -> write(1);}}', '1'],
  ['list_exact', '{switch({1,2}){case {x,y} -> write(x+y);}}', '3'],
  ['list_tail', '{switch({1,2,3}){case h:t -> write(h);write(t);}}', '1; {2, 3}'],
  ['list_size', '{switch({1,2,3}){case {x,y} -> write(0); | case _ -> write(1);}}', '1'],
  ['type_int', '{switch(1){case x@#int -> write(x);}}', '1'],
  ['type_string', '{switch("a"){case x@#string -> write(x);}}', 'a'],
  ['type_array', '{switch([1]){case x@#array -> write(x);}}', '[1]'],
  ['type_sexp', '{switch(Leaf){case x@#sexp -> write(x);}}', 'Leaf'],
  ['type_mismatch', '{switch([1]){case #sexp -> write(0); | case _ -> write(1);}}', '1'],
  ['alias_pattern', '{switch([1,2]){case whole@[x,y] -> write(whole);write(x+y);}}', '[1; 2]; 3'],
  ['duplicate_name', '{switch(Pair(1,2)){case Pair(x,x) -> write(x);}}', '2'],
  ['outer_binding', '{switch(Box(1)){case x@Box(x) -> write(x);}}', 'Box (1)'],
  ['shadow_restore', '{x=9;switch(1){case x -> x=7;write(x);}write(x);}', '7; 9'],
  ['failed_bindings', '{x=9;switch(Pair(1,2)){case Pair(x,3) -> write(0); | case _ -> write(x);}write(x);}', '9; 9'],
  ['new_variable', '{switch(1){case x -> y=2;}write(y);}', '2'],
  ['external_assignment', '{y=1;switch(1){case x -> y=2;}write(y);}', '2'],
  ['nested_scope', '{x=9;switch(1){case x -> switch(2){case x -> write(x);}write(x);}write(x);}', '2; 1; 9'],
  ['alias_mutation', '{a=[1];switch(a){case x -> x[0]=2;}write(a);}', '[2]'],
  ['read_bound_variable', '{x=9;switch(1){case x -> read(x);write(x);}write(x);read(y);write(y);}', '2; 9; 3', '', '2,3'],
  ['loop', '{i=0;while(i<3){switch(i){case 0 -> write(0); | case x -> write(x);}i+=1;}}', '0; 1; 2'],
  ['no_match', '{switch(1){case 2 -> write(0);}}', '', 'ERROR: match failure on 1'],
  ['no_match_string', '{switch("a"){case 2 -> write(0);}}', '', 'ERROR: match failure on a'],
  ['no_match_array', '{switch([1,2]){case [] -> write(0);}}', '', 'ERROR: match failure on [1; 2]'],
  ['scrutinee_error', '{switch(missing){case _ -> write(0);}}', '', 'ERROR: variable "missing" is undefined'],
  ['local_removed', '{switch(1){case x -> skip;}write(x);}', '', 'ERROR: variable "x" is undefined'],
  ['error_discards_output', '{write(7);switch(1){case x -> write(missing);}}', '', 'ERROR: variable "missing" is undefined'],
  ['reserved_literal', '{switch("#int"){case "#int" -> write(1); | case _ -> write(0);}}', '0', '', '0', 'The exported string "#int" is reserved for the type pattern; the reference literal loses its distinction in JSON.'],
  ['reserved_literal_numeric', '{switch(1){case "#int" -> write(1); | case _ -> write(0);}}', '1', '', '0', 'The exported string "#int" is reserved for the type pattern.'],
];
const shared = [
  ['concat_strings', '{write("a"+"b");}', 'ab'],
  ['concat_integer_left', '{write(12+"x");}', '12x'],
  ['concat_integer_right', '{write("x"+(0-12));}', 'x-12'],
  ['concat_unicode', '{write("я"+"🙂");}', 'я🙂'],
  ['concat_empty', '{write(""+"");write(""+1);write(1+"");}', '; 1; 1'],
  ['concat_independent', '{s="abc";t=s+"";s[0]=90;write(s);write(t);}', 'Zbc; abc'],
  ['concat_independent_right', '{s="abc";t=""+s;t[0]=90;write(s);write(t);}', 'abc; Zbc'],
  ['concat_invalid_utf8', '{s="a";s[0]=226;t="b";t[0]=130;write(s+t);}', '�'],
  ['numeric_overflow', '{write(2147483647+1);write("x"+(2147483647+1));}', '-2147483648; x-2147483648'],
  ['concat_array', '{write([1]+[2]);}', '', 'ERROR: binary operator "+" is applied to [1] and [2]'],
  ['concat_constructor', '{write(Leaf+"x");}', '', 'ERROR: binary operator "+" is applied to Leaf and x'],
  ['input_constructor', '{read(x);write(x);}', 'Pair (1, {2, 3})', '', 'Pair(1,{2,3})'],
  ['input_nullary', '{read(x);write(x);}', 'Leaf', '', 'Leaf'],
  ['input_list', '{read(x);write(x);}', '{1, 2}', '', '{1,2}'],
  ['input_empty_list', '{read(x);write(x);}', '{}', '', '{}'],
  ['input_nested', '{read(x);write(x);}', 'Box ([1; x], {Pair (2, 3), Leaf})', '', 'Box([1,"x"],{Pair(2,3),Leaf})'],
  ['input_mixed', '{read(x);read(y);read(z);write(x);write(y);write(z);}', '[1; {2, 3}]; Leaf; -4', '', '[1,{2,3}],Leaf,-4'],
  ['input_comments', '{read(x);write(x);}', 'Pair (-1, {2, 3})', '', '(* outer (* nested *) *) Pair(-1, {2, -- line\n3})'],
  ['input_characters', '{read(a);read(b);read(c);read(d);write(a);write(b);write(c);write(d);}', '65; 10; 9; 39', '', "'A', '\\n', '\\t', ''''"],
  ['input_alias', '{read(x);y=x;write(x);y[0]=9;write(y);}', 'Pair (9, 2); Pair (9, 2)', '', 'Pair(1,2)'],
  ['input_list_mutation', '{read(x);x[1][0]=9;write(x);}', '{1, 9}', '', '{1,2}'],
  ['input_exhausted', '{read(x);read(y);}', '', 'ERROR: input exhausted', 'Leaf'],
];
const cases = [];
for (const [interpreterOnly, sources] of [[true, switches], [false, shared]]) {
  for (const [name, source, output, message = '', input = '0', difference] of sources) {
    const direct = run('eval', source, input);
    const sm = interpreterOnly ? null : run('sm', source, input);
    cases.push({name, source, program: direct.program, ...(sm ? {instructions: sm.program} : {}),
      input, interpreterOnly, reference: {eval: {output: direct.output, message: direct.message},
        sm: sm ? {output: sm.output, message: sm.message} : null}, expected: {output, message},
      ...(difference ? {difference} : {})});
  }
}
const result = {reference: {url: 'https://dboulytchev.github.io/eval.html',
  machine_url: 'https://dboulytchev.github.io/sm.html', sha256,
  differences: ['Project formatting uses semicolons and unquoted strings.',
    'Reserved type-pattern strings cannot represent identical string literals in exported AST.',
    'The official SM compiler does not support switch.']}, cases};
const filename = path.resolve(__dirname, '../tests/testdata/language_reference.json');
if (mode === '--record') fs.writeFileSync(filename, JSON.stringify(result, null, 2) + '\n');
else assert.deepEqual(JSON.parse(fs.readFileSync(filename, 'utf8')), result);
console.log(`${cases.length} language cases (${mode}); ${switches.length} interpreter-only; SHA-256 ${sha256}`);
