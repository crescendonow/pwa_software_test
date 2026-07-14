(function () {
  "use strict";

  const MAX_ROWS = 100;

  function escapeHTML(value) {
    return String(value == null ? "" : value)
      .replaceAll("&", "&amp;")
      .replaceAll("<", "&lt;")
      .replaceAll(">", "&gt;")
      .replaceAll('"', "&quot;")
      .replaceAll("'", "&#039;");
  }

  function renderResult(output, payload) {
    const answer = escapeHTML(payload.answer || "ไม่พบคำตอบ");
    const rows = Array.isArray(payload.rows) ? payload.rows.slice(0, MAX_ROWS) : [];
    const columns = Array.isArray(payload.columns) ? payload.columns : [];

    let html = `<p class="mb-3 whitespace-pre-wrap text-sm">${answer}</p>`;
    if (payload.status === "rejected") {
      output.innerHTML = `<div class="rounded border border-amber-200 bg-amber-50 p-3 text-amber-800 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-200">${html}</div>`;
      return;
    }
    if (rows.length === 0 || columns.length === 0) {
      output.innerHTML = html;
      return;
    }

    html += '<div class="overflow-x-auto"><table class="min-w-full text-left text-xs"><thead><tr class="border-b border-slate-200 dark:border-slate-700">';
    html += columns.map((column) => `<th class="px-2 py-2 font-semibold">${escapeHTML(column)}</th>`).join("");
    html += "</tr></thead><tbody>";
    html += rows.map((row) => `<tr class="border-b border-slate-100 dark:border-slate-800">${columns.map((column) => `<td class="max-w-[260px] px-2 py-2 align-top">${escapeHTML(row && row[column])}</td>`).join("")}</tr>`).join("");
    html += "</tbody></table></div>";
    if (payload.truncated || (payload.row_count || 0) > MAX_ROWS) {
      html += `<p class="mt-2 text-xs text-slate-500 dark:text-slate-400">แสดง ${MAX_ROWS} แถวแรกจากผลลัพธ์ทั้งหมด</p>`;
    }
    output.innerHTML = html;
  }

  function init() {
    const form = document.querySelector("#free-text-query-form");
    if (!form) return;
    const input = document.querySelector("#free-text-query-input");
    const button = document.querySelector("#free-text-query-button");
    const status = document.querySelector("#free-text-query-status");
    const output = document.querySelector("#free-text-query-output");
    const count = document.querySelector("#free-text-query-count");

    input.addEventListener("input", () => {
      count.textContent = `${input.value.length}/500`;
    });
    form.addEventListener("submit", async (event) => {
      event.preventDefault();
      const prompt = input.value.trim();
      if (!prompt) {
        status.textContent = "กรุณาพิมพ์คำถาม";
        return;
      }
      button.disabled = true;
      status.textContent = "กำลังประมวลผล...";
      output.innerHTML = "";
      try {
        const response = await fetch("api/freetext-query", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          credentials: "same-origin",
          body: JSON.stringify({ prompt }),
        });
        const body = await response.json();
        if (!response.ok) throw new Error(body.error || "ไม่สามารถประมวลผลคำถามได้");
        renderResult(output, body);
        status.textContent = body.status === "success" ? "เสร็จแล้ว" : "ระบบปฏิเสธคำถามนี้";
      } catch (error) {
        status.textContent = error.message || "ไม่สามารถเชื่อมต่อบริการได้";
      } finally {
        button.disabled = false;
      }
    });
  }

  document.addEventListener("DOMContentLoaded", init);
})();
