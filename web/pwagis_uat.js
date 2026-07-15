const LAYER_LABELS = {
  bldg: "อาคาร",
  firehydrant: "หัวดับเพลิง",
  leakpoint: "จุดซ่อมท่อ",
  meter: "มาตรวัดน้ำ",
  pipe: "เส้นท่อ",
  pwa_waterworks: "ที่ตั้งกิจการประปา",
  valve: "ประตูน้ำ",
  struct: "รั้ว",
  pipe_serv: "ท่อบริการ",
  dma_boundary: "ขอบเขต DMA",
  step_test: "ขอบเขตจ่ายน้ำย่อย",
  flow_meter: "อุปกรณ์มาตรวัดการไหล",
  additional_requirement: "ความต้องการเพิ่มเติม",
};

const ThaiDatePicker = (() => {
  const months = [
    "มกราคม", "กุมภาพันธ์", "มีนาคม", "เมษายน", "พฤษภาคม", "มิถุนายน",
    "กรกฎาคม", "สิงหาคม", "กันยายน", "ตุลาคม", "พฤศจิกายน", "ธันวาคม",
  ];
  const monthsShort = ["ม.ค.", "ก.พ.", "มี.ค.", "เม.ย.", "พ.ค.", "มิ.ย.", "ก.ค.", "ส.ค.", "ก.ย.", "ต.ค.", "พ.ย.", "ธ.ค."];

  function formatBE(date, format = "full") {
    if (!date) return "";
    if (typeof date === "string") date = new Date(`${date}T00:00:00`);
    if (Number.isNaN(date.getTime())) return "";
    const monthSet = format === "short" ? monthsShort : months;
    return `${date.getDate()} ${monthSet[date.getMonth()]} ${date.getFullYear() + 543}`;
  }

  function patchBE(fp, format) {
    window.requestAnimationFrame(() => {
      const yearInput = fp.calendarContainer?.querySelector(".cur-year");
      if (yearInput) {
        yearInput.value = fp.currentYear + 543;
        yearInput.readOnly = true;
        yearInput.style.pointerEvents = "none";
      }
      const monthEl = fp.calendarContainer?.querySelector(".flatpickr-current-month .cur-month");
      if (monthEl) monthEl.textContent = months[fp.currentMonth];
      if (fp.altInput && fp.selectedDates?.length) fp.altInput.value = formatBE(fp.selectedDates[0], format);
    });
  }

  function init(selector, options = {}) {
    if (!window.flatpickr) return null;
    if (flatpickr.l10ns?.th) flatpickr.localize(flatpickr.l10ns.th);
    const displayFormat = options.format || "full";
    const fp = flatpickr(selector, {
      locale: "th",
      dateFormat: "Y-m-d",
      altInput: true,
      altFormat: "j F Y",
      allowInput: false,
      defaultDate: options.defaultDate,
      onReady: (_, __, instance) => patchBE(instance, displayFormat),
      onOpen: (_, __, instance) => patchBE(instance, displayFormat),
      onMonthChange: (_, __, instance) => patchBE(instance, displayFormat),
      onYearChange: (_, __, instance) => patchBE(instance, displayFormat),
      onChange: (_, __, instance) => patchBE(instance, displayFormat),
    });
    return {
      fp,
      getDate() {
        return fp.selectedDates?.length ? fp.formatDate(fp.selectedDates[0], "Y-m-d") : "";
      },
      setDate(value) {
        fp.setDate(value, true);
      },
    };
  }

  return { init, formatBE };
})();

const state = {
  references: { test_versions: [], test_suites: [], layer_names: [], feature_changes: [], test_actions: [] },
  sessionInfo: null,
  sessions: [],
  currentSession: null,
  results: [],
  summary: { total: 0, passed: 0, failed: 0, pending: 0 },
  commentTimers: new Map(),
  reportRows: [],
  datePicker: null,
  reportDateFromPicker: null,
  reportDateToPicker: null,
  offices: [],
  selectedBranches: new Map(),
};

const els = {};

document.addEventListener("DOMContentLoaded", () => {
  bindElements();
  bindEvents();
  initTheme();
  initDatePicker();
  refreshIcons();
  boot();
});

async function boot() {
  setSaveState("กำลังโหลด");
  await checkHealth();
  await Promise.all([loadReferences(), loadSessionInfo(), loadOffices()]);
  await refreshAll();
  await loadReport();
  setSaveState("พร้อมใช้งาน");
  refreshIcons();
}

function bindElements() {
  Object.assign(els, {
    healthBadge: document.querySelector("#health-badge"),
    refreshButton: document.querySelector("#refresh-button"),
    saveButton: document.querySelector("#save-button"),
    exportButton: document.querySelector("#export-button"),
    themeToggle: document.querySelector("#theme-toggle"),
    sessionForm: document.querySelector("#session-form"),
    testVersion: document.querySelector("#test-version"),
    testerName: document.querySelector("#tester-name"),
    testDate: document.querySelector("#test-date"),
    createSessionButton: document.querySelector("#create-session-button"),
    branchPicker: document.querySelector("#branch-picker"),
    branchSelectedCount: document.querySelector("#branch-selected-count"),
    currentSessionLabel: document.querySelector("#current-session-label"),
    sessionMeta: document.querySelector("#session-meta"),
    sessionList: document.querySelector("#session-list"),
    sessionCount: document.querySelector("#session-count"),
    sessionBranchFilter: document.querySelector("#session-branch-filter"),
    resultsBody: document.querySelector("#results-body"),
    summaryTotal: document.querySelector("#summary-total"),
    summaryPassed: document.querySelector("#summary-passed"),
    summaryFailed: document.querySelector("#summary-failed"),
    summaryPending: document.querySelector("#summary-pending"),
    saveState: document.querySelector("#save-state"),
    toast: document.querySelector("#toast"),
    suiteFilter: document.querySelector("#suite-filter"),
    layerFilter: document.querySelector("#layer-filter"),
    featureFilter: document.querySelector("#feature-filter"),
    actionFilter: document.querySelector("#action-filter"),
    clearCaseFilters: document.querySelector("#clear-case-filters"),
    reportVersionFilter: document.querySelector("#report-version-filter"),
    reportSuiteFilter: document.querySelector("#report-suite-filter"),
    reportAreaFilter: document.querySelector("#report-area-filter"),
    reportTesterFilter: document.querySelector("#report-tester-filter"),
    reportDateFrom: document.querySelector("#report-date-from"),
    reportDateTo: document.querySelector("#report-date-to"),
    loadReportButton: document.querySelector("#load-report-button"),
    reportBody: document.querySelector("#report-body"),
  });
}

function bindEvents() {
  els.refreshButton.addEventListener("click", async () => {
    await refreshAll();
    await loadReport();
  });
  els.saveButton.addEventListener("click", saveAllResults);
  els.exportButton.addEventListener("click", exportCurrentReport);
  els.themeToggle.addEventListener("click", toggleTheme);
  els.sessionForm.addEventListener("submit", createSession);
  document.querySelectorAll("[data-tab]").forEach((button) => button.addEventListener("click", () => switchTab(button.dataset.tab)));
  [els.suiteFilter, els.layerFilter, els.featureFilter, els.actionFilter].forEach((select) => select.addEventListener("change", renderRows));
  els.clearCaseFilters.addEventListener("click", () => {
    els.suiteFilter.value = "";
    els.layerFilter.value = "";
    els.featureFilter.value = "";
    els.actionFilter.value = "";
    renderRows();
  });
  els.sessionBranchFilter.addEventListener("change", renderSessions);
  els.loadReportButton.addEventListener("click", loadReport);
}

function initTheme() {
  const stored = window.localStorage.getItem("uat-theme");
  const shouldDark = stored ? stored === "dark" : window.matchMedia?.("(prefers-color-scheme: dark)").matches;
  document.documentElement.classList.toggle("dark", Boolean(shouldDark));
  updateThemeIcon();
}

function toggleTheme() {
  const isDark = !document.documentElement.classList.contains("dark");
  document.documentElement.classList.toggle("dark", isDark);
  window.localStorage.setItem("uat-theme", isDark ? "dark" : "light");
  updateThemeIcon();
}

function updateThemeIcon() {
  const icon = document.documentElement.classList.contains("dark") ? "sun" : "moon";
  els.themeToggle.innerHTML = `<i data-lucide="${icon}" class="h-4 w-4"></i>`;
  refreshIcons();
}

function initDatePicker() {
  const today = todayISO();
  state.datePicker = ThaiDatePicker.init("#test-date", { defaultDate: today, format: "full" });
  if (!state.datePicker) els.testDate.value = today;
  state.reportDateFromPicker = ThaiDatePicker.init("#report-date-from", { format: "short" });
  state.reportDateToPicker = ThaiDatePicker.init("#report-date-to", { format: "short" });
}

async function refreshAll() {
  setSaveState("กำลังโหลด sessions");
  await checkHealth();
  await loadSessions();
  if (state.currentSession) {
    await selectSession(state.currentSession.id);
  } else if (state.sessions.length > 0) {
    await selectSession(state.sessions[0].id);
  } else {
    renderWorkbench();
  }
  setSaveState("พร้อมใช้งาน");
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
    populateSelect(els.testVersion, state.references.test_versions, "เลือกเวอร์ชันทดสอบ");
    populateSelect(els.suiteFilter, state.references.test_suites ?? [], "ทุกหมวดทดสอบ");
    populateSelect(els.layerFilter, state.references.layer_names, "ทุกชั้นข้อมูล", layerLabel);
    populateSelect(els.featureFilter, state.references.feature_changes, "ทุก feature changes");
    populateSelect(els.actionFilter, state.references.test_actions, "ทุก action");
    populateSelect(els.reportVersionFilter, state.references.test_versions, "ทุกเวอร์ชัน");
    populateSelect(els.reportSuiteFilter, state.references.test_suites ?? [], "ทุกหมวด");
  } catch (error) {
    showToast(error.message);
  }
}

async function loadOffices() {
  try {
    const payload = await requestJSON("api/offices");
    state.offices = payload.offices ?? [];
  } catch (error) {
    state.offices = [];
  } finally {
    renderBranchPicker();
  }
}

function renderBranchPicker() {
  if (!els.branchPicker) return;
  if (state.offices.length === 0) {
    els.branchPicker.innerHTML = `<div class="py-6 text-center text-xs text-slate-500">ไม่พบรายชื่อสาขา (ใช้สาขาของผู้ทดสอบแทน)</div>`;
    updateBranchSelectedCount();
    return;
  }

  const byZone = new Map();
  state.offices.forEach((office) => {
    const zone = office.zone || "ไม่ระบุเขต";
    if (!byZone.has(zone)) byZone.set(zone, []);
    byZone.get(zone).push(office);
  });

  const zones = [...byZone.keys()].sort((a, b) => a.localeCompare(b, "th", { numeric: true }));
  els.branchPicker.innerHTML = zones
    .map((zone) => {
      const offices = byZone.get(zone).sort((a, b) => a.name.localeCompare(b.name, "th"));
      const selectedCount = offices.filter((office) => state.selectedBranches.has(office.pwa_code)).length;
      const rows = offices
        .map((office) => {
          const checked = state.selectedBranches.has(office.pwa_code) ? "checked" : "";
          return `
            <label class="branch-option">
              <input type="checkbox" data-branch-checkbox data-pwa-code="${escapeHTML(office.pwa_code)}" data-name="${escapeHTML(office.name)}" data-zone="${escapeHTML(zone)}" ${checked} />
              <span class="min-w-0"><span class="branch-option-name">${escapeHTML(office.name)}</span> <span class="branch-option-code">${escapeHTML(office.pwa_code)}</span></span>
            </label>
          `;
        })
        .join("");
      return `
        <details class="branch-zone-group" data-branch-zone="${escapeHTML(zone)}" ${selectedCount > 0 ? "open" : ""}>
          <summary class="branch-zone-summary">
            <span class="branch-zone-chevron" aria-hidden="true"></span>
            <span class="branch-zone-name">เขต ${escapeHTML(zone)}</span>
            <span class="branch-zone-total">${offices.length} สาขา</span>
            <span class="branch-zone-selected" data-zone-selected-count>${selectedCount > 0 ? `เลือก ${selectedCount}` : ""}</span>
          </summary>
          <div class="branch-zone-options">${rows}</div>
        </details>
      `;
    })
    .join("");

  els.branchPicker.querySelectorAll("[data-branch-checkbox]").forEach((checkbox) => {
    checkbox.addEventListener("change", () => {
      const { pwaCode, name, zone } = checkbox.dataset;
      if (checkbox.checked) {
        state.selectedBranches.set(pwaCode, { pwa_code: pwaCode, name, zone });
      } else {
        state.selectedBranches.delete(pwaCode);
      }
      updateBranchSelectedCount();
      updateZoneSelectedCounts();
    });
  });
  updateBranchSelectedCount();
  updateZoneSelectedCounts();
}

function updateZoneSelectedCounts() {
  if (!els.branchPicker) return;
  els.branchPicker.querySelectorAll("[data-branch-zone]").forEach((group) => {
    const count = group.querySelectorAll("[data-branch-checkbox]:checked").length;
    const badge = group.querySelector("[data-zone-selected-count]");
    if (badge) badge.textContent = count > 0 ? `เลือก ${count}` : "";
    group.classList.toggle("has-selection", count > 0);
  });
}

function updateBranchSelectedCount() {
  if (!els.branchSelectedCount) return;
  const count = state.selectedBranches.size;
  els.branchSelectedCount.textContent = count === 0 ? "ยังไม่ได้เลือก (จะใช้สาขาของผู้ทดสอบ)" : `เลือกแล้ว ${count} สาขา`;
}

async function loadSessionInfo() {
  try {
    const payload = await requestJSON("api/session-info");
    state.sessionInfo = payload.session_info;
    els.testerName.value = state.sessionInfo.uname;
    els.createSessionButton.disabled = false;
    renderSessionMeta();
  } catch (error) {
    state.sessionInfo = null;
    els.testerName.value = "";
    els.createSessionButton.disabled = true;
    renderSessionMeta(error.message);
    showToast(`โหลดข้อมูลผู้ทดสอบไม่สำเร็จ: ${error.message}`);
  }
}

function renderSessionMeta(errorMessage = "") {
  if (errorMessage) {
    els.sessionMeta.innerHTML = `<div class="rounded border border-rose-200 bg-rose-50 px-3 py-2 text-rose-700 dark:border-rose-900 dark:bg-rose-950/40 dark:text-rose-200">${escapeHTML(errorMessage)}</div>`;
    return;
  }
  if (!state.sessionInfo) {
    els.sessionMeta.innerHTML = "";
    return;
  }
  const info = state.sessionInfo;
  els.sessionMeta.innerHTML = [
    ["UID", info.uid],
    ["รหัส กปภ.", info.pwa_code],
    ["เขต", info.area],
    ["ตำแหน่ง", info.position],
    ["งาน", info.job_name],
    ["กอง", info.division],
    ["สำนัก", info.institution],
  ]
    .map(([label, value]) => `<div class="rounded border border-slate-200 bg-slate-50 px-3 py-2 dark:border-slate-700 dark:bg-slate-800"><span class="font-semibold">${escapeHTML(label)}:</span> ${escapeHTML(value || "-")}</div>`)
    .join("");
}

async function loadSessions() {
  try {
    const payload = await requestJSON("api/sessions");
    state.sessions = payload.sessions ?? [];
    renderSessions();
    syncReportFilters();
  } catch (error) {
    showToast(error.message);
  }
}

async function createSession(event) {
  event.preventDefault();
  if (!state.sessionInfo) {
    showToast("ยังไม่มีข้อมูลผู้ทดสอบจาก session info");
    return;
  }
  const branches = [...state.selectedBranches.values()];
  const input = {
    test_version: els.testVersion.value,
    tester_name: state.sessionInfo.uname,
    uid: state.sessionInfo.uid,
    pwa_code: state.sessionInfo.pwa_code,
    area: state.sessionInfo.area,
    job_name: state.sessionInfo.job_name,
    division: state.sessionInfo.division,
    institution: state.sessionInfo.institution,
    position: state.sessionInfo.position,
    test_date: selectedDateISO(),
  };
  if (branches.length > 0) input.branches = branches;

  try {
    els.createSessionButton.disabled = true;
    setSaveState("กำลังสร้าง session");
    const payload = await requestJSON("api/sessions", {
      method: "POST",
      body: JSON.stringify(input),
    });
    await loadSessions();
    await selectSession(payload.session.id);
    await loadReport();
    const createdCount = payload.sessions?.length ?? 1;
    showToast(createdCount > 1 ? `สร้าง ${createdCount} session (หนึ่งต่อสาขา) แล้ว` : "สร้าง session แล้ว");
  } catch (error) {
    showToast(error.message);
  } finally {
    els.createSessionButton.disabled = !state.sessionInfo;
    setSaveState("พร้อมใช้งาน");
    refreshIcons();
  }
}

async function selectSession(sessionID) {
  try {
    setSaveState("กำลังโหลดผลทดสอบ");
    const payload = await requestJSON(`api/sessions/${sessionID}/results`);
    state.currentSession = payload.session;
    state.results = payload.results ?? [];
    state.summary = payload.summary ?? summarizeLocal(state.results);
    renderSessions();
    renderWorkbench();
  } catch (error) {
    showToast(error.message);
  } finally {
    setSaveState("พร้อมใช้งาน");
    refreshIcons();
  }
}

function renderSessions() {
  syncSessionBranchFilter();
  const branchFilter = els.sessionBranchFilter?.value ?? "";
  const visibleSessions = branchFilter ? state.sessions.filter((session) => session.pwa_code === branchFilter) : state.sessions;

  els.sessionCount.textContent = String(state.sessions.length);
  if (visibleSessions.length === 0) {
    els.sessionList.innerHTML = `<div class="flex min-w-full items-center justify-center px-4 py-8 text-center text-sm text-slate-500">ยังไม่มี session</div>`;
    return;
  }

  els.sessionList.innerHTML = visibleSessions
    .map((session) => {
      const active = state.currentSession?.id === session.id;
      const branchLabel = session.branch_name ? `${session.branch_name} · ` : "";
      const canDelete = state.sessionInfo?.uid === session.uid;
      return `
        <div class="session-nav-item ${active ? "active" : ""}">
          <button type="button" class="session-nav-select" data-session-id="${session.id}" ${active ? 'aria-current="page"' : ""}>
            <span class="block text-sm font-semibold">${escapeHTML(session.test_version)}</span>
            <span class="mt-1 block text-xs text-slate-600 dark:text-slate-300">${escapeHTML(formatDisplayDate(session.test_date))} · ${escapeHTML(session.tester_name)}</span>
            <span class="mt-1 block text-xs text-slate-500 dark:text-slate-400">${escapeHTML(branchLabel)}เขต ${escapeHTML(session.area || "-")} · ${escapeHTML(session.pwa_code || "-")}</span>
          </button>
          ${canDelete ? `
            <button type="button" class="session-delete-button" data-delete-session-id="${session.id}" title="ลบ session" aria-label="ลบ session ${escapeHTML(session.test_version)}">
              <i data-lucide="trash-2" class="h-4 w-4"></i>
            </button>
          ` : ""}
        </div>
      `;
    })
    .join("");

  els.sessionList.querySelectorAll("[data-session-id]").forEach((button) => {
    button.addEventListener("click", () => selectSession(Number(button.dataset.sessionId)));
  });
  els.sessionList.querySelectorAll("[data-delete-session-id]").forEach((button) => {
    button.addEventListener("click", () => deleteSession(Number(button.dataset.deleteSessionId)));
  });
  refreshIcons();
}

async function deleteSession(sessionID) {
  if (!window.confirm("ลบ session นี้ใช่หรือไม่? การดำเนินการนี้ไม่สามารถย้อนกลับได้")) return;
  try {
    setSaveState("กำลังลบ session");
    await requestJSON(`api/sessions/${sessionID}`, { method: "DELETE" });
    if (state.currentSession?.id === sessionID) {
      state.currentSession = null;
      state.results = [];
      state.summary = { total: 0, passed: 0, failed: 0, pending: 0 };
    }
    await refreshAll();
    await loadReport();
    showToast("ลบ session แล้ว");
  } catch (error) {
    showToast(error.message);
  } finally {
    setSaveState("พร้อมใช้งาน");
  }
}

function syncSessionBranchFilter() {
  if (!els.sessionBranchFilter) return;
  const current = els.sessionBranchFilter.value;
  const branches = uniqueSorted(state.sessions.map((session) => session.pwa_code).filter(Boolean));
  populateSelect(els.sessionBranchFilter, branches, "ทุกสาขา", (pwaCode) => {
    const session = state.sessions.find((item) => item.pwa_code === pwaCode);
    return session?.branch_name ? `${session.branch_name} (${pwaCode})` : pwaCode;
  });
  els.sessionBranchFilter.value = branches.includes(current) ? current : "";
}

function renderWorkbench() {
  renderSummary(state.summary);
  els.exportButton.disabled = !state.currentSession;
  els.saveButton.disabled = !state.currentSession || !state.sessionInfo;

  if (!state.currentSession) {
    els.currentSessionLabel.textContent = "ยังไม่ได้เลือก session";
    els.resultsBody.innerHTML = `
      <tr>
        <td colspan="6" class="px-3 py-10 text-center text-sm text-slate-500">ยังไม่มีข้อมูล UAT</td>
      </tr>
    `;
    refreshIcons();
    return;
  }

  els.currentSessionLabel.textContent = `${state.currentSession.test_version} · ${formatDisplayDate(state.currentSession.test_date)} · ${state.currentSession.tester_name}`;
  renderRows();
}

function renderSummary(summary) {
  els.summaryTotal.textContent = String(summary.total ?? 0);
  els.summaryPassed.textContent = String(summary.passed ?? 0);
  els.summaryFailed.textContent = String(summary.failed ?? 0);
  els.summaryPending.textContent = String(summary.pending ?? 0);
}

function filteredResults() {
  const suite = els.suiteFilter.value;
  const layer = els.layerFilter.value;
  const feature = els.featureFilter.value;
  const action = els.actionFilter.value;
  return state.results.filter((result) => {
    const testCase = result.test_case;
    return (
      (!suite || testCase.test_suite === suite) &&
      (!layer || testCase.layer_name === layer) &&
      (!feature || testCase.feature_changes === feature) &&
      (!action || testCase.test_action === action)
    );
  });
}

function renderRows() {
  const rows = filteredResults();
  if (rows.length === 0) {
    els.resultsBody.innerHTML = `<tr><td colspan="6" class="px-3 py-10 text-center text-sm text-slate-500">ไม่พบรายการตามตัวกรอง</td></tr>`;
    return;
  }

  let lastLayer = "";
  let lastFeature = "";
  let lastGroup = "";

  els.resultsBody.innerHTML = rows
    .map((result) => {
      const testCase = result.test_case;
      const showLayer = testCase.layer_name !== lastLayer;
      const showFeature = showLayer || testCase.feature_changes !== lastFeature;
      const showGroup = testCase.case_group && (showLayer || testCase.case_group !== lastGroup);

      lastLayer = testCase.layer_name;
      lastFeature = testCase.feature_changes;
      lastGroup = testCase.case_group;

      const rowTone = result.is_failed
        ? "bg-rose-50/70 dark:bg-rose-950/25"
        : result.is_passed
          ? "bg-emerald-50/70 dark:bg-emerald-950/25"
          : "bg-white dark:bg-slate-900";
      return `
        <tr class="${rowTone}" data-result-row="${result.id}">
          <td class="align-top px-3 py-3 font-medium">${showLayer ? escapeHTML(layerLabel(testCase.layer_name)) : ""}</td>
          <td class="align-top px-3 py-3 text-slate-700 dark:text-slate-300">${showFeature ? escapeHTML(testCase.feature_changes) : ""}</td>
          <td class="align-top px-3 py-3">
            ${showGroup ? `<div class="mb-1 inline-flex rounded border border-slate-200 bg-slate-100 px-2 py-1 text-xs font-medium text-slate-600 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300">${escapeHTML(testCase.case_group)}</div>` : ""}
            <div>${escapeHTML(testCase.test_action)}</div>
          </td>
          <td class="align-top px-3 py-3 text-center">
            <input type="checkbox" class="h-5 w-5 rounded border-slate-300 text-emerald-700 focus:ring-emerald-600" data-result-id="${result.id}" data-field="is_passed" ${result.is_passed ? "checked" : ""} />
          </td>
          <td class="align-top px-3 py-3 text-center">
            <input type="checkbox" class="h-5 w-5 rounded border-slate-300 text-rose-700 focus:ring-rose-600" data-result-id="${result.id}" data-field="is_failed" ${result.is_failed ? "checked" : ""} />
          </td>
          <td class="align-top px-3 py-3">
            <textarea data-result-id="${result.id}" class="min-h-20 w-full resize-y rounded border border-slate-300 bg-white px-3 py-2 text-sm leading-6 text-slate-900 outline-none focus:border-primary-600 focus:ring-2 focus:ring-primary-100 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100 dark:focus:ring-sky-950">${escapeHTML(result.comment ?? "")}</textarea>
          </td>
        </tr>
      `;
    })
    .join("");

  els.resultsBody.querySelectorAll("input[type='checkbox']").forEach((input) => input.addEventListener("change", updateOutcome));
  els.resultsBody.querySelectorAll("textarea").forEach((textarea) => textarea.addEventListener("input", updateComment));
}

async function updateOutcome(event) {
  const result = findResult(Number(event.currentTarget.dataset.resultId));
  if (!result) return;

  const field = event.currentTarget.dataset.field;
  const checked = event.currentTarget.checked;

  if (field === "is_passed") {
    result.is_passed = checked;
    if (checked) result.is_failed = false;
  }
  if (field === "is_failed") {
    result.is_failed = checked;
    if (checked) result.is_passed = false;
  }

  state.summary = summarizeLocal(state.results);
  renderWorkbench();
  await saveResult(result);
}

function updateComment(event) {
  const result = findResult(Number(event.currentTarget.dataset.resultId));
  if (!result) return;

  result.comment = event.currentTarget.value;
  window.clearTimeout(state.commentTimers.get(result.id));
  state.commentTimers.set(result.id, window.setTimeout(() => saveResult(result), 600));
}

async function saveResult(result) {
  try {
    setSaveState("กำลังบันทึก");
    const payload = await requestJSON(`api/results/${result.id}`, {
      method: "PATCH",
      body: JSON.stringify({
        is_passed: result.is_passed,
        is_failed: result.is_failed,
        comment: result.comment ?? "",
      }),
    });
    Object.assign(result, payload.result);
    setSaveState(`บันทึกแล้ว ${new Date().toLocaleTimeString("th-TH", { hour: "2-digit", minute: "2-digit" })}`);
  } catch (error) {
    showToast(error.message);
    setSaveState("บันทึกไม่สำเร็จ");
  }
}

// Save button: flush any pending debounced comment saves and push every
// result on the page immediately. Auto-save (updateOutcome / debounced
// updateComment -> saveResult) keeps running independently as a safety net.
async function saveAllResults() {
  if (!state.currentSession || state.results.length === 0) return;

  state.commentTimers.forEach((timerId) => window.clearTimeout(timerId));
  state.commentTimers.clear();

  try {
    els.saveButton.disabled = true;
    setSaveState("กำลังบันทึกทั้งหมด");
    await Promise.all(state.results.map((result) => saveResult(result)));
    setSaveState(`บันทึกทั้งหมดแล้ว ${new Date().toLocaleTimeString("th-TH", { hour: "2-digit", minute: "2-digit" })}`);
    showToast("บันทึกผลทั้งหมดแล้ว");
  } finally {
    els.saveButton.disabled = !state.currentSession || !state.sessionInfo;
  }
}

function reportFilterParams() {
  const params = new URLSearchParams();
  if (els.reportVersionFilter.value) params.set("test_version", els.reportVersionFilter.value);
  if (els.reportSuiteFilter.value) params.set("test_suite", els.reportSuiteFilter.value);
  if (els.reportAreaFilter.value) params.set("area", els.reportAreaFilter.value);
  if (els.reportTesterFilter.value) params.set("tester_name", els.reportTesterFilter.value);
  const dateFrom = state.reportDateFromPicker?.getDate();
  const dateTo = state.reportDateToPicker?.getDate();
  if (dateFrom) params.set("date_from", dateFrom);
  if (dateTo) params.set("date_to", dateTo);
  return params;
}

async function loadReport() {
  const query = reportFilterParams().toString();
  try {
    const payload = await requestJSON(`api/report${query ? `?${query}` : ""}`);
    state.reportRows = payload.rows ?? [];
    renderReport();
  } catch (error) {
    showToast(error.message);
  }
}

function renderReport() {
  if (state.reportRows.length === 0) {
    els.reportBody.innerHTML = `<tr><td colspan="10" class="px-3 py-10 text-center text-sm text-slate-500">ไม่พบข้อมูลรายงาน</td></tr>`;
    return;
  }
  els.reportBody.innerHTML = state.reportRows
    .map((row) => {
      const outcome = row.is_passed ? "ได้" : row.is_failed ? "ไม่ได้" : "ยังไม่ระบุ";
      const tone = row.is_passed ? "text-emerald-700 dark:text-emerald-300" : row.is_failed ? "text-rose-700 dark:text-rose-300" : "text-amber-700 dark:text-amber-300";
      return `
        <tr class="bg-white dark:bg-slate-900">
          <td class="px-3 py-3">${escapeHTML(formatDisplayDate(row.test_date))}</td>
          <td class="px-3 py-3">${escapeHTML(row.test_version)}</td>
          <td class="px-3 py-3">${escapeHTML(row.test_suite || "-")}</td>
          <td class="px-3 py-3">${escapeHTML(row.tester_name)}</td>
          <td class="px-3 py-3">${escapeHTML(row.area || "-")}</td>
          <td class="px-3 py-3">${escapeHTML(row.pwa_code || "-")}</td>
          <td class="px-3 py-3">${escapeHTML(layerLabel(row.layer_name))}</td>
          <td class="px-3 py-3">${escapeHTML(row.test_action)}</td>
          <td class="px-3 py-3 text-center font-semibold ${tone}">${outcome}</td>
          <td class="px-3 py-3">${escapeHTML(row.comment || "")}</td>
        </tr>
      `;
    })
    .join("");
}

async function exportCurrentReport() {
  if (!state.currentSession) return;

  try {
    setSaveState("กำลัง export");
    const payload = await requestJSON(`api/report?session_id=${state.currentSession.id}`);
    const rows = payload.rows ?? [];
    const headers = [
      "วันที่ทดสอบ",
      "เวอร์ชันทดสอบ",
      "หมวดทดสอบ",
      "ผู้ทดสอบ",
      "รหัสผู้ใช้",
      "รหัส กปภ.",
      "เขต",
      "งาน",
      "กอง",
      "สำนัก",
      "ตำแหน่ง",
      "ชั้นข้อมูล",
      "สิ่งที่ต่างจากเวอร์ชันเดิม",
      "หัวข้อย่อย",
      "หัวข้อการทดสอบ",
      "ได้",
      "ไม่ได้",
      "หมายเหตุ",
    ];
    const csvRows = rows.map((row) => [
      row.test_date,
      row.test_version,
      row.test_suite,
      row.tester_name,
      row.uid,
      row.pwa_code,
      row.area,
      row.job_name,
      row.division,
      row.institution,
      row.position,
      layerLabel(row.layer_name),
      row.feature_changes,
      row.case_group,
      row.test_action,
      row.is_passed ? "1" : "0",
      row.is_failed ? "1" : "0",
      row.comment,
    ]);
    downloadCSV([headers, ...csvRows], `uat-report-${state.currentSession.test_date}.csv`);
    setSaveState("พร้อมใช้งาน");
  } catch (error) {
    showToast(error.message);
    setSaveState("export ไม่สำเร็จ");
  }
}

function syncReportFilters() {
  const currentArea = els.reportAreaFilter.value;
  const currentTester = els.reportTesterFilter.value;
  const areas = uniqueSorted(state.sessions.map((session) => session.area).filter(Boolean));
  const testers = uniqueSorted(state.sessions.map((session) => session.tester_name).filter(Boolean));
  populateSelect(els.reportAreaFilter, areas, "ทุกเขต");
  populateSelect(els.reportTesterFilter, testers, "ทุกคน");
  els.reportAreaFilter.value = areas.includes(currentArea) ? currentArea : "";
  els.reportTesterFilter.value = testers.includes(currentTester) ? currentTester : "";
}

function populateSelect(select, values, placeholder, labeler = (value) => value) {
  const current = select.value;
  select.innerHTML = `<option value="">${escapeHTML(placeholder)}</option>`;
  values.forEach((value) => {
    const option = document.createElement("option");
    option.value = value;
    option.textContent = labeler(value);
    select.appendChild(option);
  });
  if (values.includes(current)) select.value = current;
}

function switchTab(tab) {
  document.querySelectorAll("[data-tab]").forEach((button) => button.classList.toggle("active", button.dataset.tab === tab));
  document.querySelectorAll("[data-panel]").forEach((panel) => panel.classList.toggle("hidden", panel.dataset.panel !== tab));
  if (tab === "report") loadReport();
}

function downloadCSV(rows, filename) {
  const csv = rows.map((row) => row.map(csvCell).join(",")).join("\r\n");
  const blob = new Blob(["\ufeff", csv], { type: "text/csv;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

function csvCell(value) {
  const text = String(value ?? "");
  return `"${text.replaceAll('"', '""')}"`;
}

async function requestJSON(path, options = {}) {
  const response = await fetch(path, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...(options.headers ?? {}),
    },
  });
  const text = await response.text();
  const payload = text ? JSON.parse(text) : {};
  if (!response.ok) {
    const error = new Error(payload.error || `HTTP ${response.status}`);
    error.status = response.status;
    throw error;
  }
  return payload;
}

function authURL(path) {
  const base = window.location.pathname.replace(/\/pwagis_uat\.html\/?$/, "").replace(/\/$/, "");
  return `${base}/${path.replace(/^\//, "")}`;
}

function findResult(resultID) {
  return state.results.find((result) => result.id === resultID);
}

function summarizeLocal(results) {
  return results.reduce(
    (summary, result) => {
      summary.total += 1;
      if (result.is_passed) summary.passed += 1;
      else if (result.is_failed) summary.failed += 1;
      else summary.pending += 1;
      return summary;
    },
    { total: 0, passed: 0, failed: 0, pending: 0 },
  );
}

function selectedDateISO() {
  return state.datePicker?.getDate() || els.testDate.value || todayISO();
}

function todayISO() {
  const date = new Date();
  date.setMinutes(date.getMinutes() - date.getTimezoneOffset());
  return date.toISOString().slice(0, 10);
}

function formatDisplayDate(value) {
  return ThaiDatePicker.formatBE(value, "short");
}

function layerLabel(value) {
  return LAYER_LABELS[value] ? `${LAYER_LABELS[value]} (${value})` : value;
}

function uniqueSorted(values) {
  return [...new Set(values)].sort((a, b) => a.localeCompare(b, "th"));
}

function setSaveState(message) {
  els.saveState.textContent = message;
}

function showToast(message) {
  els.toast.textContent = message;
  els.toast.classList.remove("hidden");
  window.clearTimeout(showToast.timer);
  showToast.timer = window.setTimeout(() => els.toast.classList.add("hidden"), 3600);
}

function refreshIcons() {
  if (window.lucide) window.lucide.createIcons();
}

function escapeHTML(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}


// Keep the authenticated UAT session active only while the user is interacting.
let lastActivityPing = 0;
function recordActivity() {
  const now = Date.now();
  if (now - lastActivityPing < 60_000) return;
  lastActivityPing = now;
  fetch("api/session-info", { credentials: "same-origin" }).then((response) => {
    if (response.status === 401) window.location.assign(authURL("login"));
  }).catch(() => {});
}
["click", "keydown", "scroll", "touchstart", "change"].forEach((eventName) => {
  window.addEventListener(eventName, recordActivity, { passive: eventName !== "keydown" });
});
