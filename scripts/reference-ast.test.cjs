const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const {spawnSync} = require('node:child_process');

const root = path.resolve(__dirname, '..');
const script = path.join(__dirname, 'reference-ast.cjs');
const echo = {seq: {left: {read: 'x'}, right: {write: {var: 'x'}}}};

function fixture(t) {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'reference workflow '));
  t.after(() => fs.rmSync(directory, {recursive: true, force: true}));
  const env = {...process.env, SOURCE: '', AST: '', CODE: '', INPUT: '', OUTPUT: '',
    REFERENCE: 'web', INTERPRETER: path.join(root, 'interpreter.exe'),
    COMPILER: path.join(root, 'compiler.exe'), STACK_MACHINE: path.join(root, 'main.exe')};
  function file(name, data) { const filename = path.join(directory, name); fs.writeFileSync(filename, data); return filename; }
  env.SOURCE = file('source with spaces.l0', 'valid source');
  env.INPUT = file('input with spaces.txt', '[1,"hello",[]]');
  env.REFERENCE_JS = file('reference bundle.js', `
    document.getElementById('parse').onclick = () => {
      const source = document.getElementById('input').value;
      if (source === 'invalid') {document.getElementById('message').value = 'syntax error'; return;}
      if (source === 'bad json') {document.getElementById('output').value = '{'; return;}
      if (source === 'bad ast') {document.getElementById('output').value = '[1]'; return;}
      document.getElementById('output').value = JSON.stringify(${JSON.stringify(echo)});
      throw new Error('reference executed the source instead of stopping at AST');
    };
  `);
  return {directory, env, file,
    invoke(mode, overrides = {}) {return spawnSync(process.execPath, [script, mode], {env: {...env, ...overrides}, encoding: 'utf8', timeout: 15000});},
  };
}

function success(result) {
  assert.ifError(result.error);
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.stderr, '');
}

test('web: AST is saved intact in a path containing spaces without evaluating source', t => {
  const f = fixture(t);
  const target = path.join(f.directory, 'new directory', 'AST with spaces.json');
  success(f.invoke('ast', {AST: target}));
  assert.deepEqual(JSON.parse(fs.readFileSync(target, 'utf8')), echo);
  assert.equal(fs.readFileSync(f.env.SOURCE, 'utf8'), 'valid source');
  assert.deepEqual(fs.readdirSync(path.dirname(target)), ['AST with spaces.json']);
});

test('web: parse errors and malformed JSON do not replace existing AST', t => {
  const f = fixture(t);
  const target = f.file('existing.json', 'preserved');
  for (const source of ['invalid', 'bad json', 'bad ast']) {
    fs.writeFileSync(f.env.SOURCE, source);
    const result = f.invoke('ast', {AST: target});
    assert.notEqual(result.status, 0);
    assert.notEqual(result.stderr, '');
    assert.equal(fs.readFileSync(target, 'utf8'), 'preserved');
  }
});

test('source run uses generated AST and supports stdout and output file', t => {
  const f = fixture(t);
  let result = f.invoke('run');
  success(result);
  assert.equal(result.stdout, '[1; hello; []]');
  const target = path.join(f.directory, 'result file.txt');
  result = f.invoke('run', {OUTPUT: target});
  success(result);
  assert.equal(result.stdout, '');
  assert.equal(fs.readFileSync(target, 'utf8'), '[1; hello; []]');
});

test('existing AST does not require any reference or source', t => {
  const f = fixture(t);
  const ast = f.file('provided.json', JSON.stringify(echo));
  const result = f.invoke('run', {SOURCE: '', AST: ast, REFERENCE: 'unused', REFERENCE_JS: '/missing', DOCKER: '/missing'});
  success(result);
  assert.equal(result.stdout, '[1; hello; []]');
});

test('a failed reference or interpreter preserves the result file', t => {
  const f = fixture(t);
  const target = f.file('result.txt', 'preserved');
  fs.writeFileSync(f.env.SOURCE, 'invalid');
  let result = f.invoke('run', {OUTPUT: target});
  assert.notEqual(result.status, 0);
  assert.equal(fs.readFileSync(target, 'utf8'), 'preserved');
  const ast = f.file('failure.json', '{"write":{"var":"undefined"}}');
  result = f.invoke('run', {SOURCE: '', AST: ast, OUTPUT: target});
  assert.equal(result.status, 1);
  assert.equal(result.stderr, 'ERROR: variable "undefined" is undefined\n');
  assert.equal(fs.readFileSync(target, 'utf8'), 'preserved');
});

test('invalid arguments, missing files and unknown backend are rejected', t => {
  const f = fixture(t);
  for (const [mode, env] of [
    ['ast', {SOURCE: ''}], ['ast', {AST: f.env.SOURCE}],
    ['ast', {REFERENCE: 'unknown'}], ['ast', {SOURCE: '/missing'}],
    ['run', {SOURCE: '', AST: ''}], ['run', {AST: 'also-provided'}],
    ['run', {INPUT: ''}], ['run', {INPUT: '/missing'}],
    ['run', {SOURCE: '', AST: '/missing'}], ['unknown', {}],
  ]) {
    const result = f.invoke(mode, env);
    assert.notEqual(result.status, 0);
    assert.notEqual(result.stderr, '');
  }
});

test('docker: source travels through stdin and errors with status 0 are rejected', t => {
  const f = fixture(t);
  const record = path.join(f.directory, 'docker call.json');
  const docker = f.file('fake docker', `#!/usr/bin/env node
    const fs = require('node:fs');
    if (process.argv[2] === 'image') process.exit(0);
    fs.writeFileSync(process.env.RECORD, JSON.stringify({args:process.argv.slice(2),source:fs.readFileSync(0,'utf8')}));
    process.stdout.write(process.env.REPLY);
    process.stderr.write(process.env.DIAGNOSTIC || '');
  `);
  fs.chmodSync(docker, 0o700);
  const target = path.join(f.directory, 'docker ast.json');
  const env = {REFERENCE: 'docker', DOCKER: docker, RECORD: record, REPLY: JSON.stringify(echo), AST: target,
    REFERENCE_IMAGE: 'example@sha256:pinned', REFERENCE_PLATFORM: 'linux/amd64'};
  const source = '{write("a b");}\n-- literal $HOME `command`';
  fs.writeFileSync(f.env.SOURCE, source);
  success(f.invoke('ast', env));
  const call = JSON.parse(fs.readFileSync(record, 'utf8'));
  assert.equal(call.source, source);
  assert.deepEqual(call.args, ['run', '--rm', '--pull=never', '-i', '--platform', 'linux/amd64', 'example@sha256:pinned', './L0', '.', '-input', '0']);
  assert.deepEqual(JSON.parse(fs.readFileSync(target, 'utf8')), echo);
  const before = fs.readFileSync(target, 'utf8');
  let result = f.invoke('ast', {...env, DIAGNOSTIC: 'ERROR: syntax error'});
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /syntax error/);
  assert.equal(fs.readFileSync(target, 'utf8'), before);
  result = f.invoke('ast', {...env, REPLY: ''});
  assert.notEqual(result.status, 0);
  assert.equal(fs.readFileSync(target, 'utf8'), before);
});

test('docker image is pulled separately before AST capture when missing', t => {
  const f = fixture(t);
  const docker = f.file('fake docker', `#!/usr/bin/env node
    const fs = require('node:fs');
    fs.appendFileSync(process.env.RECORD, process.argv[2]+'\\n');
    if (process.argv[2] === 'image') process.exit(1);
    if (process.argv[2] === 'pull') {console.error('download progress');process.exit(0);}
    process.stdout.write(process.env.REPLY);
  `);
  fs.chmodSync(docker, 0o700);
  const record = f.file('docker log', '');
  const target = path.join(f.directory, 'ast.json');
  const result = f.invoke('ast', {REFERENCE: 'docker', DOCKER: docker, RECORD: record, REPLY: '"skip"', AST: target});
  assert.equal(result.status, 0, result.stderr);
  assert.deepEqual(JSON.parse(fs.readFileSync(target, 'utf8')), 'skip');
  assert.equal(fs.readFileSync(record, 'utf8'), 'image\npull\nrun\n');
});

test('make commands transport filenames containing shell metacharacters safely', t => {
  const f = fixture(t);
  const source = f.file('source "quoted" $name.l0', 'valid source');
  const target = path.join(f.directory, 'ast "quoted" $name.json');
  const result = spawnSync('make', ['--no-print-directory', 'ast', `SOURCE=${source}`, `AST=${target}`], {
    cwd: root, env: f.env, encoding: 'utf8', timeout: 15000,
  });
  success(result);
  assert.deepEqual(JSON.parse(fs.readFileSync(target, 'utf8')), echo);
});

test('make run writes only program results to stdout', t => {
  const f = fixture(t);
  const result = spawnSync('make', ['--no-print-directory', 'run', `SOURCE=${f.env.SOURCE}`, `INPUT=${f.env.INPUT}`], {
    cwd: root, env: f.env, encoding: 'utf8', timeout: 15000,
  });
  success(result);
  assert.equal(result.stdout, '[1; hello; []]');
});

test('compile accepts an existing AST without a reference and saves SM instructions', t => {
  const f = fixture(t);
  const ast = f.file('provided.json', JSON.stringify(echo));
  const target = path.join(f.directory, 'code "quoted" $name.json');
  success(f.invoke('compile', {SOURCE: '', AST: ast, CODE: target, REFERENCE: 'unused'}));
  assert.deepEqual(JSON.parse(fs.readFileSync(target, 'utf8')), ['READ', {ST: 'x'}, {LD: 'x'}, 'WRITE']);
});

test('source can compile through the reference parser and our compiler', t => {
  const f = fixture(t);
  const target = path.join(f.directory, 'compiled.json');
  success(f.invoke('compile', {CODE: target, INPUT: ''}));
  assert.deepEqual(JSON.parse(fs.readFileSync(target, 'utf8')), ['READ', {ST: 'x'}, {LD: 'x'}, 'WRITE']);
});

test('both routes execute the same provided AST without a reference', t => {
  const f = fixture(t);
  const ast = f.file('provided.json', JSON.stringify(echo));
  fs.writeFileSync(f.env.INPUT, '42');
  const env = {SOURCE: '', AST: ast, REFERENCE: 'unused'};
  for (const mode of ['run', 'run-sm']) {
    const result = f.invoke(mode, env);
    success(result);
    assert.equal(result.stdout, '42');
  }
  const target = path.join(f.directory, 'result.txt');
  const result = f.invoke('run-sm', {...env, OUTPUT: target});
  success(result);
  assert.equal(result.stdout, '');
  assert.equal(fs.readFileSync(target, 'utf8'), '42');
});

test('both routes accept strings, quotes, unicode and mixed arrays from input', t => {
  const f = fixture(t);
  const ast = f.file('provided.json', JSON.stringify(echo));
  for (const [input, expected] of [
    ['[1,"hello",[]]', '[1; hello; []]'],
    ['" a,b "', ' a,b '], ['"a""b"', 'a"b'], ['"я🙂"', 'я🙂'], ['[]', '[]'],
  ]) {
    fs.writeFileSync(f.env.INPUT, input);
    for (const mode of ['run', 'run-sm']) {
      const result = f.invoke(mode, {SOURCE: '', AST: ast, REFERENCE: 'unused'});
      success(result);
      assert.equal(result.stdout, expected);
    }
  }
});

test('failed cyclic output preserves an existing output file in both routes', t => {
  const f = fixture(t);
  fs.writeFileSync(f.env.INPUT, '');
  const ast = f.file('cyclic.json', JSON.stringify({seq: {
    left: {assn: {dst: 'a', src: {array: [{const: 0}]}}},
    right: {seq: {
      left: {write: {var: 'a'}},
      right: {set: {dst: {var: 'a'}, idx: {const: 0}, value: {var: 'a'}}},
    }},
  }}));
  const target = f.file('result.txt', 'preserved');
  for (const mode of ['run', 'run-sm']) {
    const result = f.invoke(mode, {SOURCE: '', AST: ast, OUTPUT: target});
    assert.equal(result.status, 1);
    assert.equal(result.stdout, '');
    assert.equal(result.stderr, 'ERROR: cyclic array\n');
    assert.equal(fs.readFileSync(target, 'utf8'), 'preserved');
  }
});

test('reference and compiler failures preserve existing SM code and output', t => {
  const f = fixture(t);
  const target = f.file('result.json', 'preserved');
  const ast = f.file('invalid.json', '{"write":{"string":null}}');
  for (const mode of ['compile', 'run-sm']) {
    for (const provided of [false, true]) {
      fs.writeFileSync(f.env.SOURCE, 'invalid');
      const result = f.invoke(mode, {SOURCE: provided ? '' : f.env.SOURCE, AST: provided ? ast : '',
        CODE: target, OUTPUT: target});
      assert.equal(result.status, 1);
      assert.equal(result.stdout, '');
      assert.notEqual(result.stderr, '');
      assert.equal(fs.readFileSync(target, 'utf8'), 'preserved');
    }
  }
});

test('SM runtime errors discard partial output and preserve existing result', t => {
  const f = fixture(t);
  fs.writeFileSync(f.env.INPUT, '');
  const ast = f.file('failure.json', '{"seq":{"left":{"write":{"const":7}},"right":{"write":{"var":"missing"}}}}');
  const target = f.file('result.txt', 'preserved');
  const result = f.invoke('run-sm', {SOURCE: '', AST: ast, OUTPUT: target});
  assert.equal(result.status, 1);
  assert.equal(result.stdout, '');
  assert.match(result.stderr, /is undefined/);
  assert.equal(fs.readFileSync(target, 'utf8'), 'preserved');
});

test('compile rejects overwriting its input AST', t => {
  const f = fixture(t);
  const ast = f.file('provided.json', JSON.stringify(echo));
  const result = f.invoke('compile', {SOURCE: '', AST: ast, CODE: ast});
  assert.equal(result.status, 1);
  assert.deepEqual(JSON.parse(fs.readFileSync(ast, 'utf8')), echo);
});

test('make compile and run-sm support source and AST paths with spaces and dollar signs', t => {
  const f = fixture(t);
  fs.writeFileSync(f.env.INPUT, '42');
  const ast = f.file('AST "quoted" $name.json', JSON.stringify(echo));
  const target = path.join(f.directory, 'code "quoted" $name.json');
  let result = spawnSync('make', ['--no-print-directory', 'compile', `SOURCE=`, `AST=${ast}`, `CODE=${target}`], {
    cwd: root, env: {...f.env, REFERENCE: 'unused'}, encoding: 'utf8', timeout: 15000,
  });
  success(result);
  assert.equal(result.stdout, '');
  assert.deepEqual(JSON.parse(fs.readFileSync(target, 'utf8')), ['READ', {ST: 'x'}, {LD: 'x'}, 'WRITE']);
  result = spawnSync('make', ['--no-print-directory', 'run-sm', `SOURCE=${f.env.SOURCE}`, `INPUT=${f.env.INPUT}`], {
    cwd: root, env: f.env, encoding: 'utf8', timeout: 15000,
  });
  success(result);
  assert.equal(result.stdout, '42');
});
