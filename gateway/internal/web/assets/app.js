"use strict";

const element = (id) => document.getElementById(id);
const money = new Intl.NumberFormat("ru-RU", { style: "currency", currency: "RUB", maximumFractionDigits: 2 });
const dateFormat = new Intl.DateTimeFormat("ru-RU", { day: "2-digit", month: "short", year: "numeric" });
const state = { transactions: [], budgets: [], loaded: false, busy: false, importing: false, refresh: 0 };
let importController;

function today() {
  const date = new Date();
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}-${String(date.getDate()).padStart(2, "0")}`;
}

function message(id, text, kind = "success") {
  const target = element(id);
  target.textContent = text;
  target.className = `notice${kind === "success" ? "" : ` notice--${kind}`}`;
  target.hidden = !text;
}

function createNode(tag, text, className) {
  const node = document.createElement(tag);
  if (text !== undefined) node.textContent = text;
  if (className) node.className = className;
  return node;
}

async function request(path, options = {}) {
  const response = await fetch(path, { ...options, headers: { "Content-Type": "application/json" } });
  let data;
  try {
    data = await response.json();
  } catch (error) {
    if (error.name === "AbortError") throw error;
    throw new Error("Не удалось прочитать ответ сервера. Проверь, что запущен Gateway.");
  }
  if (!response.ok) {
    const error = new Error(data.error || `Ошибка сервера: HTTP ${response.status}`);
    error.status = response.status;
    error.data = data;
    throw error;
  }
  return { data, status: response.status };
}

function errorText(error) {
  if (error.status === 504) return "Время запроса истекло. Часть записей могла сохраниться. Проверь список расходов перед повторной отправкой.";
  if (error.name === "AbortError") return "Запрос отменён. Уже сохранённые расходы остаются в базе. Проверь список перед повторной отправкой.";
  if (error.status === 409) return "Расход превышает бюджет этой категории. Измени сумму или лимит.";
  if (error instanceof TypeError) return "Нет связи с Gateway. Проверь, что сервер запущен, и обнови данные. Если отправлял расходы, сначала проверь, сохранились ли они.";
  return error.message;
}

async function refreshData() {
  const sequence = ++state.refresh;
  element("refresh").disabled = true;
  try {
    const [transactions, budgets] = await Promise.all([request("/api/transactions"), request("/api/budgets")]);
    if (sequence !== state.refresh) return;
    state.transactions = transactions.data;
    state.budgets = budgets.data;
    state.loaded = true;
    element("connection").textContent = "API подключён";
    element("connection").className = "connection connection--online";
    message("load-message", "");
    renderData();
  } catch (error) {
    if (sequence !== state.refresh) return;
    element("connection").textContent = "Нет связи с API";
    element("connection").className = "connection connection--offline";
    message("load-message", `${errorText(error)}${state.loaded ? " На странице остались данные последнего обновления." : ""}`, "error");
    if (!state.loaded) {
      element("budget-list").replaceChildren(createNode("p", "Бюджеты пока недоступны.", "empty"));
      renderTransactions();
    }
  } finally {
    if (sequence === state.refresh) element("refresh").disabled = false;
  }
}

function renderData() {
  const total = state.transactions.reduce((sum, transaction) => sum + transaction.amount, 0);
  element("total-spent").textContent = money.format(total);
  element("transaction-count").textContent = state.transactions.length.toLocaleString("ru-RU");
  element("budget-count").textContent = state.budgets.length.toLocaleString("ru-RU");
  const categories = [...new Set([...state.budgets, ...state.transactions].map((item) => item.category))].sort();
  element("categories").replaceChildren(...categories.map((category) => {
    const option = createNode("option");
    option.value = category;
    return option;
  }));
  renderBudgets();
  renderTransactions();
}

function renderBudgets() {
  const list = element("budget-list");
  if (!state.budgets.length) {
    list.replaceChildren(createNode("p", "Бюджетов ещё нет. Добавь первый выше.", "empty"));
    return;
  }
  const spentByCategory = new Map();
  for (const transaction of state.transactions) {
    spentByCategory.set(transaction.category, (spentByCategory.get(transaction.category) || 0) + transaction.amount);
  }
  list.replaceChildren(...state.budgets.map((budget) => {
    const spent = spentByCategory.get(budget.category) || 0;
    const item = createNode("div", undefined, `budget-item${spent > budget.limit ? " budget-item--exceeded" : ""}`);
    const heading = createNode("div", undefined, "budget-item__heading");
    heading.append(createNode("strong", budget.category), createNode("span", money.format(budget.limit)));
    const progress = createNode("progress");
    progress.max = budget.limit;
    progress.value = Math.min(spent, budget.limit);
    progress.setAttribute("aria-label", `${budget.category}: потрачено ${money.format(spent)} из ${money.format(budget.limit)}`);
    item.append(heading, progress, createNode("p", `Потрачено ${money.format(spent)} · ${spent > budget.limit ? "выше лимита" : `осталось ${money.format(budget.limit - spent)}`}`));
    return item;
  }));
}

function renderTransactions() {
  const query = element("search").value.trim().toLocaleLowerCase("ru-RU");
  const filtered = state.transactions.filter((transaction) => `${transaction.category} ${transaction.description}`.toLocaleLowerCase("ru-RU").includes(query));
  const list = element("transaction-list");
  if (!filtered.length) {
    const row = createNode("tr");
    const text = !state.loaded ? "Не удалось загрузить расходы." : state.transactions.length ? "По этому запросу ничего не найдено." : "Пока пусто. Импортируй пример или добавь первый расход.";
    const cell = createNode("td", text, "empty");
    cell.colSpan = 4;
    row.append(cell);
    list.replaceChildren(row);
  } else {
    list.replaceChildren(...filtered.map((transaction) => {
      const row = createNode("tr");
      const category = createNode("td");
      category.append(createNode("span", transaction.category, "category"));
      row.append(
        createNode("td", dateFormat.format(new Date(`${transaction.date}T00:00:00`))),
        category,
        createNode("td", transaction.description || "—"),
        createNode("td", money.format(transaction.amount), "amount"),
      );
      return row;
    }));
  }
  element("transaction-info").textContent = state.loaded ? `Показано ${filtered.length} из ${state.transactions.length} · Сначала новые расходы` : "";
}

function parseImport() {
  const transactions = JSON.parse(element("import-json").value);
  if (!Array.isArray(transactions)) throw new Error("Нужен JSON-массив: записи внутри квадратных скобок [ … ].");
  return transactions;
}

function updateInputInfo() {
  const info = element("input-info");
  try {
    const transactions = parseImport();
    info.textContent = `${transactions.length} элементов · проверку выполнит Ledger`;
    info.className = "";
    element("import-submit").disabled = state.busy;
    element("import-submit").textContent = state.importing ? "Импортируем…" : "Импортировать расходы";
  } catch {
    info.textContent = "Проверь JSON: нужен корректный массив [ … ]";
    info.className = "invalid";
    element("import-submit").disabled = true;
  }
}

function insertSample() {
  const date = today();
  element("import-json").value = JSON.stringify([
    { amount: 1490, category: "Продукты", description: "Покупки на неделю", date },
    { amount: 320, category: "Транспорт", description: "Поездка в город", date },
    { amount: 0, category: "Продукты", description: "Пример ошибки в сумме", date },
    { amount: 6000, category: "Продукты", description: "Проверка лимита бюджета", date },
  ], null, 2);
  updateInputInfo();
}

function setBusy(busy, importing = false) {
  state.busy = busy;
  state.importing = busy && importing;
  for (const id of ["import-fields", "budget-fields", "transaction-fields"]) element(id).disabled = busy;
  element("import-progress").hidden = !state.importing;
  element("import-form").setAttribute("aria-busy", String(state.importing));
  updateInputInfo();
}

function showResponse(data, status) {
  element("api-response").textContent = `${status ? `HTTP ${status}\n` : ""}${JSON.stringify(data, null, 2)}`;
  element("response-details").hidden = false;
}

element("import-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  if (state.busy) return;
  let transactions;
  try {
    transactions = parseImport();
  } catch (error) {
    message("import-message", errorText(error), "error");
    return;
  }
  const workers = Number(element("workers").value);
  importController = new AbortController();
  setBusy(true, true);
  message("import-message", "");
  element("import-result").hidden = true;
  element("response-details").hidden = true;
  const started = performance.now();
  try {
    const { data, status } = await request(`/api/transactions/bulk?workers=${workers}`, { method: "POST", body: JSON.stringify(transactions), signal: importController.signal });
    element("accepted").textContent = data.accepted;
    element("rejected").textContent = data.rejected;
    element("import-duration").textContent = `${((performance.now() - started) / 1000).toFixed(2)} с · HTTP ${status}`;
    element("import-errors").replaceChildren(...data.errors.map((error) => {
      const item = createNode("li");
      item.append(createNode("strong", `Запись ${error.index + 1} · индекс ${error.index}`), createNode("span", error.error));
      return item;
    }));
    element("import-result").hidden = false;
    showResponse(data, status);
    const text = transactions.length === 0 ? "Массив пуст: сохранять нечего." : data.rejected ? "Импорт завершён. Ниже — результат и причины отказов." : "Все расходы сохранены. Они уже доступны в списке транзакций.";
    message("import-message", text, data.rejected ? "info" : "success");
  } catch (error) {
    message("import-message", errorText(error), "error");
    if (error.data) showResponse(error.data, error.status);
  } finally {
    importController = undefined;
    element("import-progress").hidden = true;
    await refreshData();
    setBusy(false);
  }
});

element("cancel-import").addEventListener("click", () => importController?.abort());
element("sample").addEventListener("click", insertSample);
element("import-json").addEventListener("input", updateInputInfo);
element("import-file").addEventListener("change", async (event) => {
  const file = event.target.files[0];
  if (!file) return;
  try {
    if (file.size > 2 * 1024 * 1024) throw new Error("Для интерфейса выбери JSON-файл не больше 2 МБ.");
    element("import-json").value = await file.text();
    message("import-message", "");
    updateInputInfo();
  } catch (error) {
    message("import-message", errorText(error), "error");
  } finally {
    event.target.value = "";
  }
});

element("budget-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  if (state.busy) return;
  const budget = { category: element("budget-category").value.trim(), limit: Number(element("budget-limit").value) };
  setBusy(true);
  message("budget-message", "");
  try {
    await request("/api/budgets", { method: "POST", body: JSON.stringify(budget) });
    message("budget-message", `Бюджет «${budget.category}» сохранён.`);
    await refreshData();
  } catch (error) {
    message("budget-message", errorText(error), "error");
  } finally {
    setBusy(false);
  }
});

element("toggle-transaction").addEventListener("click", () => {
  const form = element("transaction-form");
  form.hidden = !form.hidden;
  element("toggle-transaction").setAttribute("aria-expanded", String(!form.hidden));
  element("toggle-transaction").textContent = form.hidden ? "Добавить расход" : "Свернуть форму";
  if (!form.hidden) element("transaction-amount").focus();
});

element("transaction-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  if (state.busy) return;
  const transaction = {
    amount: Number(element("transaction-amount").value),
    category: element("transaction-category").value.trim(),
    description: element("transaction-description").value.trim(),
    date: element("transaction-date").value,
  };
  setBusy(true);
  message("transaction-message", "");
  try {
    await request("/api/transactions", { method: "POST", body: JSON.stringify(transaction) });
    message("transaction-message", "Расход сохранён.");
    element("transaction-amount").value = "";
    element("transaction-description").value = "";
    await refreshData();
  } catch (error) {
    message("transaction-message", errorText(error), "error");
  } finally {
    setBusy(false);
  }
});

element("search").addEventListener("input", renderTransactions);
element("refresh").addEventListener("click", refreshData);
element("transaction-date").value = today();
insertSample();
refreshData();
