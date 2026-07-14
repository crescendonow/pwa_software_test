// Realtime dashboard for PWA GIS UAT. Polls GET /api/dashboard/summary (and
// GET /api/dashboard/wordcloud) on an interval, pausing while the tab is
// hidden. Relies on shared.js (loaded before this file) for LAYER_LABELS,
// layerLabel, escapeHTML, requestJSON, refreshIcons, ThaiDatePicker and
// SharedTheme.

const state = {
  references: { test_versions: [], test_suites: [] },
  areas: [],
  filters: { test_version: "", test_suite: "", area: "", date_from: "", date_to: "" },
  dateFromPicker: null,
  dateToPicker: null,
  pollTimer: null,
  pollEnabled: true,
  pollIntervalSeconds: 20,
};

const els = {};

document.addEventListener("DOMContentLoaded", () => {
  bindElements();
  bindEvents();
  SharedTheme.init(els.themeToggle);
  initDatePickers();
  refreshIcons();
  boot();
});

function bindElements() {
  Object.assign(els, {
    healthBadge: document.querySelector("#health-badge"),
    updatedLabel: document.querySelector("#updated-label"),
    pollToggle: document.querySelector("#poll-toggle"),
    pollToggleLabel: document.querySelector("#poll-toggle-label"),
    refreshButton: document.querySelector("#refresh-button"),
    themeToggle: document.querySelector("#theme-toggle"),
    filterVersion: document.querySelector("#filter-version"),
    filterSuite: document.querySelector("#filter-suite"),
    filterArea: document.querySelector("#filter-area"),
    filterDateFrom: document.querySelector("#filter-date-from"),
    filterDateTo: document.querySelector("#filter-date-to"),
    applyFiltersButton: document.querySelector("#apply-filters-button"),
    pollInterval: document.querySelector("#poll-interval"),
    summaryTotal: document.querySelector("#summary-total"),
    summaryPassed: document.querySelector("#summary-passed"),
    summaryFailed: document.querySelector("#summary-failed"),
    summaryPending: document.querySelector("#summary-pending"),
    summaryPercent: document.querySelector("#summary-percent"),
    layerBreakdown: document.querySelector("#layer-breakdown"),
    suiteBreakdown: document.querySelector("#suite-breakdown"),
    wordcloudCanvas: document.querySelector("#wordcloud-canvas"),
    wordcloudEmpty: document.querySelector("#wordcloud-empty"),
    toast: document.querySelector("#toast"),
  });
}

function bindEvents() {
  els.themeToggle.addEventListener("click", () => SharedTheme.toggle(els.themeToggle));
  els.refreshButton.addEventListener("click", () => loadDashboard());
  els.applyFiltersButton.addEventListener("click", applyFilters);
  els.pollToggle.addEventListener("click", togglePolling);
  els.pollInterval.addEventListener("change", () => {
    state.pollIntervalSeconds = Number(els.pollInterval.value) || 20;
    if (state.pollEnabled) restartPolling();
  });
  document.addEventListener("visibilitychange", () => {
    if (document.hidden) {
      stopPollingTimer();
    } else if (state.pollEnabled) {
      restartPolling();
      loadDashboard();
    }
  });
}

function initDatePickers() {
  state.dateFromPicker = ThaiDatePicker.init("#filter-date-from", { format: "short" });
  state.dateToPicker = ThaiDatePicker.init("#filter-date-to", { format: "short" });
}

async function boot() {
  await checkHealth();
  await Promise.all([loadReferences(), loadAreas()]);
  await loadDashboard();
  startPolling();
  refreshIcons();
}

async function checkHealth() {
  try {
    await requestJSON("api/health");
    els.healthBadge.innerHTML = `<span class="h-2 w-2 rounded-full bg-emerald-400"></span> connected`;
  } catch (error) {
    els.healthBadge.innerHTML = `<span class="h-2 w-2 rounded-full bg-rose-400"></span> disconnected`;
  }
}

async function loadReferences() {
  try {
    const payload = await requestJSON("api/references");
    state.references = payload.references ?? state.references;
    populateSelect(els.filterVersion, state.references.test_versions ?? [], "ทุกเวอร์ชัน");
    populateSelect(els.filterSuite, state.references.test_suites ?? [], "ทุกหมวด");
  } catch (error) {
    showToast(error.message);
  }
}

async function loadAreas() {
  try {
    const payload = await requestJSON("api/sessions");
    const sessions = payload.sessions ?? [];
    state.areas = uniqueSorted(sessions.map((session) => session.area).filter(Boolean));
    populateSelect(els.filterArea, state.areas, "ทุกเขต");
  } catch (error) {
    // Non-fatal: area filter simply stays empty.
  }
}

function populateSelect(select, values, placeholder) {
  const current = select.value;
  select.innerHTML = `<option value="">${escapeHTML(placeholder)}</option>`;
  values.forEach((value) => {
    const option = document.createElement("option");
    option.value = value;
    option.textContent = value;
    select.appendChild(option);
  });
  if (values.includes(current)) select.value = current;
}

function applyFilters() {
  state.filters = {
    test_version: els.filterVersion.value,
    test_suite: els.filterSuite.value,
    area: els.filterArea.value,
    date_from: state.dateFromPicker?.getDate() || "",
    date_to: state.dateToPicker?.getDate() || "",
  };
  loadDashboard();
}

function filterParams() {
  const params = new URLSearchParams();
  Object.entries(state.filters).forEach(([key, value]) => {
    if (value) params.set(key, value);
  });
  return params;
}

async function loadDashboard() {
  const query = filterParams().toString();
  try {
    const summary = await requestJSON(`api/dashboard/summary${query ? `?${query}` : ""}`);
    renderSummary(summary);
    els.updatedLabel.textContent = `อัปเดตล่าสุด ${new Date().toLocaleTimeString("th-TH", { hour: "2-digit", minute: "2-digit", second: "2-digit" })}`;
  } catch (error) {
    showToast(error.message);
  }

  try {
    const wordcloud = await requestJSON(`api/dashboard/wordcloud${query ? `?${query}` : ""}`);
    renderWordcloud(wordcloud.words ?? []);
  } catch (error) {
    renderWordcloud([]);
  }
}

function renderSummary(summary) {
  els.summaryTotal.textContent = String(summary.total ?? 0);
  els.summaryPassed.textContent = String(summary.passed ?? 0);
  els.summaryFailed.textContent = String(summary.failed ?? 0);
  els.summaryPending.textContent = String(summary.pending ?? 0);
  els.summaryPercent.textContent = `${(summary.percent_passed ?? 0).toFixed(1)}%`;
  renderBreakdown(els.layerBreakdown, summary.by_layer ?? [], layerLabel);
  renderBreakdown(els.suiteBreakdown, summary.by_suite ?? [], (key) => key);
}

function renderBreakdown(container, items, labeler) {
  if (!items || items.length === 0) {
    container.innerHTML = `<div class="py-6 text-center text-sm text-slate-500">ไม่มีข้อมูล</div>`;
    return;
  }
  container.innerHTML = items
    .map((item) => {
      const percent = Math.max(0, Math.min(100, item.percent_passed ?? 0));
      return `
        <div>
          <div class="mb-1 flex items-center justify-between text-sm">
            <span class="font-medium">${escapeHTML(labeler(item.key))}</span>
            <span class="text-xs text-slate-500 dark:text-slate-400">${item.passed}/${item.total} ได้ (${percent.toFixed(0)}%)</span>
          </div>
          <div class="h-2.5 w-full overflow-hidden rounded-full bg-slate-200 dark:bg-slate-700">
            <div class="h-full rounded-full bg-emerald-500" style="width: ${percent}%"></div>
          </div>
        </div>
      `;
    })
    .join("");
}

function renderWordcloud(words) {
  const canvas = els.wordcloudCanvas;
  const ctx = canvas.getContext("2d");
  ctx.clearRect(0, 0, canvas.width, canvas.height);

  if (!words || words.length === 0 || typeof window.WordCloud !== "function") {
    els.wordcloudEmpty.classList.remove("hidden");
    canvas.classList.add("hidden");
    return;
  }

  els.wordcloudEmpty.classList.add("hidden");
  canvas.classList.remove("hidden");
  const isDark = document.documentElement.classList.contains("dark");
  window.WordCloud(canvas, {
    list: words.map((item) => [item.word, item.weight]),
    weightFactor: (size) => Math.max(10, Math.min(64, 10 + size * 3)),
    fontFamily: "Noto Sans Thai, sans-serif",
    color: isDark ? "random-light" : "random-dark",
    backgroundColor: "transparent",
    rotateRatio: 0.2,
  });
}

function startPolling() {
  state.pollEnabled = true;
  updatePollToggleUI();
  restartPolling();
}

function restartPolling() {
  stopPollingTimer();
  state.pollTimer = window.setInterval(() => {
    if (!document.hidden) loadDashboard();
  }, state.pollIntervalSeconds * 1000);
}

function stopPollingTimer() {
  if (state.pollTimer) {
    window.clearInterval(state.pollTimer);
    state.pollTimer = null;
  }
}

function togglePolling() {
  state.pollEnabled = !state.pollEnabled;
  if (state.pollEnabled) {
    restartPolling();
    loadDashboard();
  } else {
    stopPollingTimer();
  }
  updatePollToggleUI();
}

function updatePollToggleUI() {
  const icon = state.pollEnabled ? "pause" : "play";
  els.pollToggle.querySelector("i")?.remove();
  els.pollToggle.insertAdjacentHTML("afterbegin", `<i data-lucide="${icon}" class="h-4 w-4"></i>`);
  els.pollToggleLabel.textContent = `Realtime: ${state.pollEnabled ? "ON" : "OFF"}`;
  refreshIcons();
}

function showToast(message) {
  els.toast.textContent = message;
  els.toast.classList.remove("hidden");
  window.clearTimeout(showToast.timer);
  showToast.timer = window.setTimeout(() => els.toast.classList.add("hidden"), 3600);
}
