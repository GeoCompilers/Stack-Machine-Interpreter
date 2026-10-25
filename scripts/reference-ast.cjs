const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const vm = require('node:vm');
const {spawnSync} = require('node:child_process');

const root = path.resolve(__dirname, '..');
const image = process.env.REFERENCE_IMAGE || 'dboulytchev/pl-platforms@sha256:14acc7b02e71c0a9c69e4bfe9a0aed57e8e2e7e3c0bc6b75131cd489fd66e1dd';
const platform = process.env.REFERENCE_PLATFORM || 'linux/amd64';
const backend = process.env.REFERENCE || 'docker';
const docker = process.env.DOCKER || 'docker';

function required(name) {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required; see make help`);
  return value;
}

function execute(command, args, options = {}) {
  const result = spawnSync(command, args, {
    encoding: 'utf8', maxBuffer: 16 * 1024 * 1024, ...options,
  });
  if (result.error) throw new Error(`${command}: ${result.error.message}`);
  if (result.status !== 0) {
    const error = new Error((result.stderr || '').trim() || `${command} exited with ${result.status ?? result.signal}`);
    error.reported = options.stdio === 'inherit' || (Array.isArray(options.stdio) && options.stdio[2] === 2);
    error.status = result.status;
    throw error;
  }
  return result;
}

function fromDocker(source) {
  const inspect = spawnSync(docker, ['image', 'inspect', image], {encoding: 'utf8'});
  if (inspect.error) throw inspect.error;
  if (inspect.status !== 0) {
    execute(docker, ['pull', '--platform', platform, image], {stdio: ['ignore', 2, 2]});
  }
  const result = execute(docker, [
    'run', '--rm', '--pull=never', '-i', '--platform', platform,
    image, './L0', '.', '-input', '0',
  ], {input: source});
  if (result.stderr.trim()) throw new Error(result.stderr.trim());
  return result.stdout;
}

function fromWeb(source) {
  const filename = process.env.REFERENCE_JS || path.join(root, '.cache/L0.js');
  const bundle = fs.readFileSync(filename, 'utf8');
  const elements = Object.fromEntries(['title', 'input', 'output', 'message', 'args', 'res', 'parse'].map(id => [id, {
    tagName: id === 'title' ? 'TITLE' : id === 'parse' ? 'BUTTON' : 'TEXTAREA', value: '',
  }]));
  elements.title.text = 'Simple Imperative Language Reference Interpreter';
  elements.input.value = source;
  elements.args.value = '0';
  let ast;
  // Stop when the reference publishes the AST, before it evaluates the program.
  Object.defineProperty(elements.output, 'value', {
    get() { return ast || ''; },
    set(value) {
      if (value) { ast = value; throw new Error('AST captured'); }
    },
  });
  const context = vm.createContext({
    document: {getElementById: id => elements[id]}, console, TextDecoder, TextEncoder,
  });
  vm.runInContext(bundle, context, {timeout: 10000});
  try {
    vm.runInContext("document.getElementById('parse').onclick({preventDefault(){}})", context, {timeout: 10000});
  } catch (error) {
    if (!ast) throw error;
  }
  if (!ast) throw new Error(elements.message.value || 'Reference did not produce an AST');
  return ast;
}

function generate(sourcePath) {
  const source = fs.readFileSync(sourcePath, 'utf8');
  let raw;
  if (backend === 'docker') raw = fromDocker(source);
  else if (backend === 'web') raw = fromWeb(source);
  else throw new Error('REFERENCE must be docker or web');
  let ast;
  try { ast = JSON.parse(raw); }
  catch { throw new Error('Reference did not return valid JSON AST'); }
  if (ast !== 'skip' && (!ast || typeof ast !== 'object' || Array.isArray(ast))) {
    throw new Error('Reference did not return a statement AST');
  }
  return JSON.stringify(ast, null, 2) + '\n';
}

function saveFile(filename, contents) {
  const target = path.resolve(filename);
  fs.mkdirSync(path.dirname(target), {recursive: true});
  const directory = fs.mkdtempSync(path.join(path.dirname(target), '.reference-ast-'));
  try {
    const temporary = path.join(directory, 'ast.json');
    fs.writeFileSync(temporary, contents, {mode: 0o600});
    fs.renameSync(temporary, target);
  } finally {
    fs.rmSync(directory, {recursive: true, force: true});
  }
}

function main() {
  const mode = process.argv[2];
  if (!['ast', 'run', 'compile', 'run-sm'].includes(mode)) {
    throw new Error('Usage: reference-ast.cjs ast|run|compile|run-sm (paths from make variables)');
  }
  if (mode === 'ast') {
    const source = required('SOURCE');
    const target = process.env.AST || path.join(root, '.cache/program.ast.json');
    if (path.resolve(source) === path.resolve(target)) throw new Error('SOURCE and AST must be different files');
    saveFile(target, generate(source));
    return;
  }
  const source = process.env.SOURCE;
  const ast = process.env.AST;
  if ((!source && !ast) || (source && ast)) throw new Error('Provide exactly one of SOURCE or AST');
  const input = mode === 'compile' ? null : required('INPUT');
  if (input) fs.accessSync(input, fs.constants.R_OK);
  const target = mode === 'compile' ? process.env.CODE || path.join(root, '.cache/program.sm.json') : process.env.OUTPUT;
  if (target && [source, ast, input].filter(Boolean).some(file => path.resolve(file) === path.resolve(target))) {
    throw new Error('Output must differ from SOURCE, AST and INPUT');
  }
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'l0-ast-'));
  try {
    const code = source ? path.join(directory, 'program.json') : ast;
    if (source) fs.writeFileSync(code, generate(source), {mode: 0o600});
    if (mode === 'run') {
      const binary = process.env.INTERPRETER || path.join(root, 'interpreter.exe');
      const args = ['-code', code, '-input', input];
      if (target) args.push('-output', target);
      execute(binary, args, {stdio: 'inherit'});
    } else {
      const compiler = process.env.COMPILER || path.join(root, 'compiler.exe');
      const instructions = execute(compiler, ['-code', code]).stdout;
      if (mode === 'compile') {
        saveFile(target, instructions);
      } else {
        const smCode = path.join(directory, 'program.sm.json');
        fs.writeFileSync(smCode, instructions, {mode: 0o600});
        const machine = process.env.STACK_MACHINE || path.join(root, 'main.exe');
        const result = execute(machine, ['-code', smCode, '-input', input]).stdout;
        if (target) saveFile(target, result);
        else process.stdout.write(result);
      }
    }
  } finally {
    fs.rmSync(directory, {recursive: true, force: true});
  }
}

try {
  // L0.js's parser requires a larger stack for nested expressions.
  if (backend === 'web' && !process.execArgv.includes('--stack-size=4096')) {
    execute(process.execPath, ['--stack-size=4096', __filename, ...process.argv.slice(2)], {stdio: 'inherit'});
  } else main();
} catch (error) {
  if (!error.reported) console.error(error.message || String(error));
  process.exitCode = error.status > 0 ? error.status : 1;
}
