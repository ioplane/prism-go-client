# Prism SDK: план модернизации v7

Дата: 2026-10-15. Источник задач — локальный Beads `um-docs-nrsu` в um-docs.
[Полный план, инфраструктурные эпики и критерии](https://um-git.infra4.dev/p.lavrukhin/um-docs/-/blob/sprint/nutanix-premerge-2026-10-15/docs/plans/nutanix-rke2-modernization/README.md).

## Порядок

Baseline и действующие quality gates → исправления SDK/node driver → HTTP/auth/input/deadline → operation identity/reconcile → структура → измеренная оптимизация → Rancher wiring → release → согласованный canary → scale/replace/upgrade.

Критичность и порядок независимы: P1/C1 recovery не выполняется до проверки
входа/deadline и quality gates. Статусы проверять в Beads; код в PR не означает
merge или внедрение. Каждая semantic wave начинается с RED regression и
оставляет проверенный совместимый consumer contract.

## Состав

| Beads | Работа | Этап | P/C |
|:---|:---|:---|:---|
| `um-docs-nrsu.6.10.5` | Pre-merge аудит Go 1.16–1.27, API/RE и Testcontainers | closed audit | P1/C2 |
| `um-docs-nrsu.6.10.12` | SDK/driver: baseline пакетов, входных данных и производительности | W0 | P1/C3 |
| `um-docs-nrsu.6.10.10` | Fork governance: Keep a Changelog и Conventional Commits | W1 | P1/C2 |
| `um-docs-nrsu.6.10.6` | Go quality: полный lint, formatter, input inventory и fuzz gates | W2 | P1/C2 |
| `um-docs-nrsu.6.10.1` | Node driver: форк ioplane и обновление Go/компонентов | W1 | P1/C3 |
| `um-docs-nrsu.6.10.2` | Node driver: golangci-lint v2 строгий профиль и CI | W1 | P1/C2 |
| `um-docs-nrsu.6.10.3` | Node driver: optional vNIC queues и контрактные тесты | W1 | P2/C3 |
| `um-docs-nrsu.6.10.4` | Prism SDK: изоляция HTTP transport и TLS между клиентами | W1 | P1/C2 |
| `um-docs-nrsu.6.10.7` | SDK Go 1.27 и устранение standalone dependency findings | W1 | P1/C2 |
| `um-docs-nrsu.4.8.6` | Ограничение HTTP body и безопасная диагностика SDK | W2 | P1/C2 |
| `um-docs-nrsu.4.8.7` | SDK: session-auth, replay upload и конкурентное создание клиентов | W2 | P1/C2 |
| `um-docs-nrsu.4.8.2` | Единый deadline и автомат состояний задач Prism | W2 | P1/C2 |
| `um-docs-nrsu.4.8.3` | Порядок NIC и выбор management IP по идентичности | W2 | P1/C2 |
| `um-docs-nrsu.4.8.4` | Валидация cloud-init и числовых hardware параметров | W2 | P1/C2 |
| `um-docs-nrsu.4.8.8` | SDK v4: terminal task states и отменяемый Operation.Wait | W2 | P1/C2 |
| `um-docs-nrsu.4.8.1` | Сохранение operation identity и безопасное восстановление Create/Delete | W3 | P1/C1 |
| `um-docs-nrsu.6.10.8` | SDK: структура пакетов, функций и совместимость публичного API | W4 | P2/C3 |
| `um-docs-nrsu.4.8.5` | Пагинация и HTTP budget inventory/GPU | W5 | P2/C3 |
| `um-docs-nrsu.6.10.9` | SDK: измерения производительности и повторяемости операций | W5 | P2/C3 |
| `um-docs-nrsu.6.10.11` | Release: SBOM, provenance и неизменяемый consumer pin | W7 | P2/C3 |

## Обязательные контракты

Сохранить package/import paths, exported signatures, nil/omitted/zero JSON,
RPC/config/state совместимость. Разделить длинные файлы по обязанностям внутри
существующих packages; не создавать новый framework. Реальные headers/API
semantics подтверждать для каждого endpoint/version. После unknown outcome
сначала reconcile; не выполнять blind retry Create или Delete из-за отсутствия IP.
Auth/body replay, concurrent cache и v4 terminal states проверять адресно.

Go tools, exact images/dependencies, readonly/tidy/verify, race/shuffle/atomic,
vet и whole graph govulncheck выполнять в Podman. Full SDK strict lint/fuzz
остаются отдельной задачей W2 после адресных исправлений; compiler/module inventory W0 включает nested nusights. Testcontainers проверяет HTTP boundary,
не полную семантику Prism. Performance измерять по одинаковым fixtures и input:
request/TLS/auth counts, allocations, bytes, memory/FD/goroutines и latency.

Keep a Changelog и Conventional Commits проверяются target governance.
Published history сохраняется; новые releases используют dated SemVer и
immutable artifacts с SBOM/provenance. Merge, Rancher registration, изменения
VM и нагрузка имеют отдельные acceptance/authorization gates.

## W0: воспроизводимые измерения

`make -f Makefile.dev benchmark-check` проверяет assertions стенда с race detector;
`make -f Makefile.dev benchmark` сохраняет пять выборок при GOMAXPROCS=4, 100 ms на
benchmark. Время не является CI threshold. Все команды исполняются через
существующий pinned Podman image. Производительность production из этих
fixtures не выводится. Compiler/module/API inventory собирается средствами
`go list -export -compiled -deps -test -json`, `go list -m -json all` и
`go doc -all` отдельно для каждого Go module. Числа и границы измерения
публикуются в каноническом плане um-docs; полные исходные результаты локальны.

[Датированный W0 baseline: compiler/API inputs, fixtures и распределения](https://um-git.infra4.dev/p.lavrukhin/um-docs/-/blob/sprint/nutanix-premerge-2026-10-15/docs/plans/nutanix-rke2-modernization/baseline-2026-10-15.md).
Root `./...` не охватывает nested modules. Nusights имеет отдельный
module-level vulnerability gate; generated public API сохраняется.


## Приоритет разработки v3

По указанию владельца сначала исправлять SDK и node driver. Полный stylistic
lint/formatter SDK не блокирует адресные исправления `um-docs-nrsu.4.8.4` и
`um-docs-nrsu.4.8.6`; их prerequisite — закрытый W0 и действующие обязательные
checks. Новые тесты добавлять только для критичных исправляемых регрессий;
существующие race/vet/vuln/module/RPC и driver lint gates сохраняются.
Не добавлять новые benchmark campaigns и test frameworks для этого этапа.

Текущий batch: timeout считывается из запроса до minimum clamp; SDK filter
отклоняет nil filter, отсутствующий/null/non-array entities и non-object element
с ошибкой вместо panic. Корректные filter payload и optional queue/RPC JSON
контракты сохраняются. Response boundary продолжен этапом v4 ниже; driver
YAML/hardware validation реализован в каноническом плане v5. Review/delivery
остаются отдельными gates. Этап v6 начинает upload часть um-docs-nrsu.4.8.7.

Go release notes проверены через GitHub API
[`golang/website` — Go 1.27](https://go.dev/doc/go1.27), source blob
`2d38a2be7836deca1d17451c6e03dd444f282847`; stdlib HTTP body ownership проверен
через Context7 `/golang/go`. Новые зависимости и версии tools для этих
исправлений не требуются. Driver использует неизменяемый pin исправленного SDK.

SonarCloud Web API `issues/search`, PR #1, 2026-10-15: security gate сообщает
`docker:S6471` для root в dev/fixture containers и `go:S4036` для PATH тестового
helper driver; `go:S3776` в SDK transport test относится к cognitive complexity.
`docker:S8431` рекомендует убрать сочетание tag+digest; обязательный immutable
pin с читаемой версией сохраняется. Findings зарегистрированы в
`um-docs-nrsu.6.10.6`; suppression и отключение checks не выполняются.


## Контракт response boundary: этап v4

`um-docs-nrsu.4.8.6`: decoded/filtered JSON ограничивается 64 MiB, error body —
64 KiB. Это политика SDK, не лимит Nutanix API. Читается максимум budget+1
байт для обнаружения превышения; Content-Length не считается доверенным.
Нефильтрованный io.Writer остаётся потоковым. JSON больше budget требует
пагинации или отдельного согласованного контракта; unlimited option не добавляется.

Обычный Error/log не содержит server message, details, body, произвольный HTTP
status text или JSON field names. Публичный ResponseError сохраняет числовой
StatusCode и Unwrap; ErrResponseTooLarge доступен через errors.Is. Изменение
текста diagnostics несовместимо со сравнением server messages как strings;
потребители переходят на errors.As/Is. Existing converged/v4 categories не меняются.

Сохранить оригинального владельца Body.Close, cancellation и streaming; не
добавлять retry мутаций. Источники: Context7 /golang/go; Go 1.27 release notes
(golang/website blob2d38a2be7836deca1d17451c6e03dd444f282847) и io.LimitReader
в golang/go go1.27.2. Новых dependencies и toolchain upgrades нет.


Измерено локально: полный `Makefile.dev check` успешен, SDK lint — 0 issues.
RED воспроизводит marker в пяти error forms и журнале; JSON 67108892 bytes
принимается сверх budget; error body читает 65538 bytes. GREEN ограничивает
чтение budget+1 и сохраняет typed status/size/cancel/JSON causes. Старые
message-string assertions заменены проверкой ResponseError.StatusCode.

Public API сравнен по тем же 15 public packages W0: 14 без изменений; root
аддитивно содержит ResponseError, ErrResponseTooLarge и Error/Unwrap. Старый
W0 text hash не является текущим hash; Go binary ABI из этого сравнения не выводится.
Тексты diagnostics и ограничения body требуют consumer migration; SDK commit
помечается breaking. Internal/v4/converged error categories не переименовываются.


## Upload replay и владение файлом: этап v6

`um-docs-nrsu.4.8.7`, P1/C2, in_progress: upload substep. Session-cookie locking,
single refresh при конкурентном expiration, constructor cancellation,
DoWithFilters refresh и cache identity/concurrency ещё не реализованы.
Зависимость от HTTP acceptance сохраняется; развитие идёт поверх проверенного
response boundary в существующем SDK PR #1. Merge/release требуют review.

NewUploadRequest использует общий builder NewUnAuthUploadRequest и сохраняет
прежнюю auth/header схему. Источник — открытый regular file; начало диапазона
берётся из текущего offset, Content-Length равен оставшейся длине. EOF и offset
за EOF дают пустой http.NoBody; pipe/device/directory отклоняются. Body и каждый
GetBody получают отдельный SectionReader; их чтение и Close не перемещают и не
закрывают исходный descriptor. Владение остаётся у вызывающего кода: файл должен
оставаться открытым и неизменным до завершения запроса и всех его повторов.
Существующий v3 Operations.UploadImage уже закрывает свой файл через defer.

Содержимое образа не копируется в память и не фиксируется отдельным snapshot;
изменение файла во время запроса нарушает контракт вызывающего кода. Новых
retry правил для mutations/unknown outcome нет. Не изменены SessionAuth,
redirect policy, public signatures, пакеты, wire headers, tools и dependencies.
Это исправление replay bytes/ownership; общая идемпотентность UploadImage,
серверная приёмка Prism и production throughput здесь не квалифицируются.

Критичные regressions используют прежний HTTP fixture: оба builders, offset0,
ненулевой offset, EOF/за EOF, чтение после Body.Close, два независимых replay,
HTTP307 с идентичными bytes и отклонение pipe. Новый файл включён в прежний
fmt gate; новые frameworks, scripts и benchmark campaigns не добавлены.

Источники: Context7 `/golang/go` и точный source toolchain:
[Request.Body/GetBody](https://github.com/golang/go/blob/022c8636110ebac86a9b88724cda168ed713c21f/src/net/http/request.go#L175),
[io.NewSectionReader](https://github.com/golang/go/blob/022c8636110ebac86a9b88724cda168ed713c21f/src/io/io.go#L485).
Tag go1.27.2 подтверждён через GH API. Go1.27 release notes прочитаны повторно,
golang/website blob `2d38a2be7836deca1d17451c6e03dd444f282847`: автоматический
response-body drain не восстанавливает request file offset и не меняет
контракт GetBody. Public API/machine RPC совместимость проверяется отдельно
от утверждений о Go binary ABI.


## Session refresh: этап плана v7

`um-docs-nrsu.4.8.7` остаётся in_progress. После upload исправлен общий send
для Do и DoWithFilters. Mutex защищает cookies и номер session version; один
refresh выполняется владельцем первого 401, ожидающие получают его результат
через закрываемый channel. Сетевой вызов не удерживает mutex и не создаёт
фоновых goroutines. Уже завершённый refresh используется поздними 401 той же
версии. Контекст владельца ограничивает refresh; отмена отдельного ожидающего
не отменяет владельца. Ошибка владельца передаётся его ожидающим, следующий
новый вызов может повторить auth. Автоматического бесконечного retry нет.

Повтор ресурса допускается один раз только после явного HTTP401 и при nil,
http.NoBody или доступном GetBody. Нет повтора transport errors с неизвестным
результатом, ошибок refresh или GetBody. Ошибка401 для non-replayable тела
возвращается без нового auth/ресурсного запроса. Исходные headers клонируются и при первой отправке не изменяются, включая
NewUnAuthRequest; session cookies заменяются только в повторе после401;
каждое тело остаётся собственностью HTTP transport. Изменения касаются общего
internal HTTP клиента v3/Konnector, generated v4 не изменён.

Критичные regression: восемь одновременных истёкших запросов обоих путей,
два auth calls вместе с constructor и шестнадцать resource calls; одинаковые
POST bytes; non-replayable и повторный401; отмена ожидающего при заблокированном
refresh владельца; ошибки transport/auth/GetBody. Existing race/vet/vuln и
consumer driver checks сохраняются. Новых dependencies, framework или
performance campaigns нет. Constructor по-прежнему использует Background:
контекст создания и v3/v4 cache coalescing/rotation/options остаются следующими
подэтапами до закрытия задачи.

Источники: Context7 `/golang/go`; [Request.Clone/GetBody и context](https://github.com/golang/go/blob/022c8636110ebac86a9b88724cda168ed713c21f/src/net/http/request.go),
[Go1.27 release notes](https://go.dev/doc/go1.27). Версии runtime/tools сохранены;
этот этап не утверждает изменение TTL cookies или семантики server-side API.
