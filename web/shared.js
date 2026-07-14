// Shared helpers reused by pwagis_uat.js (workbench) and dashboard_uat.js
// (realtime dashboard). Kept intentionally small and dependency-free so both
// pages can include it with a single <script src="shared.js"> before their
// own script.

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

function layerLabel(value) {
  return LAYER_LABELS[value] ? `${LAYER_LABELS[value]} (${value})` : value;
}

// Thai Buddhist-era date picker wrapper around flatpickr, shared with
// pwagis_uat's own copy (kept separate there to avoid a cross-script global
// redeclaration since that page does not load shared.js).
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

function escapeHTML(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

function uniqueSorted(values) {
  return [...new Set(values)].sort((a, b) => a.localeCompare(b, "th"));
}

function refreshIcons() {
  if (window.lucide) window.lucide.createIcons();
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

// Generic light/dark theme helpers driven by a shared localStorage key and
// the `dark` class on <html>, matching pwagis_uat's own theme handling.
const SharedTheme = (() => {
  const STORAGE_KEY = "uat-theme";

  function init(toggleButton) {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    const shouldDark = stored ? stored === "dark" : window.matchMedia?.("(prefers-color-scheme: dark)").matches;
    document.documentElement.classList.toggle("dark", Boolean(shouldDark));
    updateIcon(toggleButton);
  }

  function toggle(toggleButton) {
    const isDark = !document.documentElement.classList.contains("dark");
    document.documentElement.classList.toggle("dark", isDark);
    window.localStorage.setItem(STORAGE_KEY, isDark ? "dark" : "light");
    updateIcon(toggleButton);
  }

  function updateIcon(toggleButton) {
    if (!toggleButton) return;
    const icon = document.documentElement.classList.contains("dark") ? "sun" : "moon";
    toggleButton.innerHTML = `<i data-lucide="${icon}" class="h-4 w-4"></i>`;
    refreshIcons();
  }

  return { init, toggle };
})();

function showToastFactory(toastEl) {
  return function showToast(message) {
    toastEl.textContent = message;
    toastEl.classList.remove("hidden");
    window.clearTimeout(showToast.timer);
    showToast.timer = window.setTimeout(() => toastEl.classList.add("hidden"), 3600);
  };
}
