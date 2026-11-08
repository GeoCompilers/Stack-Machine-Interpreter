# Интерпретатор и компилятор L0

Реализация на Go: выполнение JSON AST напрямую или компиляция AST в инструкции
стековой машины с последующим выполнением. Поддерживаются числа, строки, массивы,
S-expressions и списки. `switch` с образцами работает только в AST-интерпретаторе.

## Архитектура

- `internal/interpreter` — декодирование и выполнение AST.
- `internal/compiler` — преобразование AST в JSON инструкций SM.
- `internal/stackmachine` — выполнение инструкций SM.
- `internal/runtime` — общие значения, операции, образцы, ввод и вывод.
- `cmd` — CLI этих трёх программ.

Исходный текст разбирает внешний [референс](https://dboulytchev.github.io/eval.html).
Наш интерпретатор и компилятор принимают готовый AST.

## Запуск

Нужен Go версии из `go.mod`. Для `make run`, `make run-sm` и работы с референсом
также нужен Node.js. Все команды выполняются из корня репозитория.

Готовый пример через AST-интерпретатор и через компилятор с SM:

```sh
make run AST=examples/structured-input.ast.json INPUT=examples/structured-input.input.txt
make run-sm AST=examples/structured-input.ast.json INPUT=examples/structured-input.input.txt
```

Пример `switch`:

```sh
make run AST=examples/switch.ast.json INPUT=examples/switch.input.txt
```

Добавь `OUTPUT=result.txt`, чтобы сохранить результат в файл. Ввод — значения через
запятую; если `read` не используется, передавай пустой файл.

Сборка и прямой запуск без Node.js:

```sh
make build-interpreter build-compiler build
./interpreter.exe -code examples/structured-input.ast.json -input examples/structured-input.input.txt
./compiler.exe -code examples/structured-input.ast.json -output code.json
./main.exe -code code.json -input examples/structured-input.input.txt
```

Запуск исходника через веб-референс:

```sh
make reference-js
make run SOURCE=examples/switch.l0 INPUT=examples/switch.input.txt REFERENCE=web
```

Проверки: `make test`. Остальные команды: `make help`.
Техническое описание `switch`: [SWITCH.md](SWITCH.md).
