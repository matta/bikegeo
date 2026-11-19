(() => {
  "use strict";

  const root = document.querySelector("[data-trace-workbench]");
  if (!root) {
    return;
  }

  const svg = root.querySelector("svg.diagram");
  const tbody = root.querySelector("table.trace-table tbody");
  if (!svg || !tbody) {
    return;
  }

  const inspector = root.querySelector("[data-step-inspector]");
  const inspectorText = root.querySelector("[data-step-inspector-text]");
  const clearBtn = root.querySelector("[data-step-clear]");

  if (inspector) {
    inspector.hidden = false;
  }
  const defaultInspectorText = inspectorText ? inspectorText.textContent : "";

  const rowById = new Map();
  const parentById = new Map();
  const buttonById = new Map();
  const summaryById = new Map();
  const orderedIds = [];

  for (const row of tbody.querySelectorAll("tr[data-step-id]")) {
    const id = row.dataset.stepId;
    if (!id) {
      continue;
    }
    orderedIds.push(id);
    rowById.set(id, row);
    parentById.set(id, row.dataset.parentId || "0");
    summaryById.set(id, row.dataset.stepSummary || "");
    const btn = row.querySelector("button[data-step-select]");
    if (btn) {
      buttonById.set(id, btn);
    }
  }

  const stepElById = new Map();
  for (const el of svg.querySelectorAll(".trace-steps [data-step-id]")) {
    if (el.dataset.stepId) {
      stepElById.set(el.dataset.stepId, el);
    }
  }

  const frameElById = new Map();
  for (const el of svg.querySelectorAll(".step-frames [data-step-frame]")) {
    if (el.dataset.stepFrame) {
      frameElById.set(el.dataset.stepFrame, el);
    }
  }

  const landmarksById = new Map();
  for (const el of svg.querySelectorAll(".landmarks [data-step-id]")) {
    const id = el.dataset.stepId;
    if (!id) {
      continue;
    }
    let list = landmarksById.get(id);
    if (!list) {
      list = [];
      landmarksById.set(id, list);
    }
    list.push(el);
  }

  let selectedId = null;
  let hoveredId = null;
  let focusedId = null;

  let activeEls = [];
  let ancestorEls = [];
  let selectedRow = null;
  let selectedBtn = null;

  function ancestorIds(id) {
    const ancestors = [];
    const visited = new Set([id]);
    let cur = parentById.get(id);
    while (cur && cur !== "0" && rowById.has(cur) && !visited.has(cur)) {
      visited.add(cur);
      ancestors.push(cur);
      cur = parentById.get(cur);
    }
    return ancestors;
  }

  function markActive(el) {
    if (!el) {
      return;
    }
    el.classList.add("is-active");
    activeEls.push(el);
  }

  function markAncestor(el) {
    if (!el) {
      return;
    }
    el.classList.add("is-ancestor");
    ancestorEls.push(el);
  }

  function matchesFocusVisible(el) {
    try {
      return Boolean(el && el.matches(":focus-visible"));
    } catch {
      return true;
    }
  }

  function render() {
    for (const el of activeEls) {
      el.classList.remove("is-active");
    }
    activeEls = [];
    for (const el of ancestorEls) {
      el.classList.remove("is-ancestor");
    }
    ancestorEls = [];
    if (selectedRow) {
      selectedRow.classList.remove("is-selected");
      selectedRow = null;
    }
    if (selectedBtn) {
      selectedBtn.setAttribute("aria-pressed", "false");
      selectedBtn = null;
    }

    const activeId = hoveredId || focusedId || selectedId;
    svg.classList.toggle("has-active-step", Boolean(activeId));

    if (selectedId) {
      const row = rowById.get(selectedId);
      if (row) {
        row.classList.add("is-selected");
        selectedRow = row;
      }
      const btn = buttonById.get(selectedId);
      if (btn) {
        btn.setAttribute("aria-pressed", "true");
        selectedBtn = btn;
      }
    }

    if (activeId) {
      for (const ancId of ancestorIds(activeId)) {
        markAncestor(rowById.get(ancId));
        markAncestor(stepElById.get(ancId));
      }
      markActive(rowById.get(activeId));
      markActive(stepElById.get(activeId));
      markActive(frameElById.get(activeId));
      const landmarks = landmarksById.get(activeId);
      if (landmarks) {
        for (const lm of landmarks) {
          markActive(lm);
        }
      }
    }

    if (inspectorText) {
      inspectorText.textContent = (activeId && summaryById.get(activeId)) || defaultInspectorText;
    }
    if (clearBtn) {
      clearBtn.hidden = !selectedId;
    }
  }

  tbody.addEventListener("mouseover", (e) => {
    const row = e.target.closest("tr[data-step-id]");
    const nextId = row && tbody.contains(row) ? row.dataset.stepId : null;
    if (nextId !== hoveredId) {
      hoveredId = nextId;
      render();
    }
  });

  tbody.addEventListener("mouseleave", () => {
    if (hoveredId !== null) {
      hoveredId = null;
      render();
    }
  });

  tbody.addEventListener("focusin", (e) => {
    const btn = e.target.closest("button[data-step-select]");
    if (!btn || !tbody.contains(btn)) {
      return;
    }
    const nextId = matchesFocusVisible(btn) ? (btn.dataset.stepSelect || null) : null;
    if (nextId !== focusedId || (nextId !== null && hoveredId !== null)) {
      focusedId = nextId;
      if (nextId !== null) {
        hoveredId = null;
      }
      render();
    }
  });

  tbody.addEventListener("focusout", (e) => {
    if (e.relatedTarget && tbody.contains(e.relatedTarget)) {
      return;
    }
    if (focusedId !== null) {
      focusedId = null;
      render();
    }
  });

  tbody.addEventListener("click", (e) => {
    const btn = e.target.closest("button[data-step-select]");
    const row = e.target.closest("tr[data-step-id]");
    if (!row || !tbody.contains(row)) {
      return;
    }
    if (!btn) {
      const sel = window.getSelection();
      if (sel && sel.toString().length > 0) {
        return;
      }
    }
    const id = (btn && btn.dataset.stepSelect) || row.dataset.stepId;
    if (!id) {
      return;
    }
    if (btn && e.detail === 0) {
      hoveredId = null;
    }
    selectedId = selectedId === id ? null : id;
    if (selectedId === null && (!btn || !matchesFocusVisible(btn))) {
      focusedId = null;
    }
    render();
  });

  tbody.addEventListener("keydown", (e) => {
    const btn = e.target.closest("button[data-step-select]");
    if (!btn || !tbody.contains(btn)) {
      return;
    }
    if (e.altKey || e.ctrlKey || e.metaKey) {
      return;
    }
    const id = btn.dataset.stepSelect;
    const idx = orderedIds.indexOf(id);
    let targetIdx = -1;
    if (e.key === "ArrowDown") {
      targetIdx = Math.min(orderedIds.length - 1, idx + 1);
    } else if (e.key === "ArrowUp") {
      targetIdx = Math.max(0, idx - 1);
    } else if (e.key === "Home") {
      targetIdx = 0;
    } else if (e.key === "End") {
      targetIdx = orderedIds.length - 1;
    } else if (e.key === "Escape") {
      if (selectedId !== null || focusedId !== null || hoveredId !== null) {
        e.preventDefault();
        selectedId = null;
        focusedId = null;
        hoveredId = null;
        render();
      }
      return;
    } else {
      return;
    }
    if (idx === -1 || targetIdx === -1) {
      return;
    }
    e.preventDefault();
    const targetId = orderedIds[targetIdx];
    const targetBtn = buttonById.get(targetId);
    if (targetBtn) {
      targetBtn.focus();
      hoveredId = null;
      focusedId = targetId;
      render();
    }
  });

  svg.addEventListener("mouseover", (e) => {
    const el = e.target.closest(".landmarks [data-step-id], .trace-steps [data-step-id]");
    const nextId = el && svg.contains(el) ? el.dataset.stepId : null;
    if (nextId !== hoveredId) {
      hoveredId = nextId;
      render();
    }
  });

  svg.addEventListener("mouseleave", () => {
    if (hoveredId !== null) {
      hoveredId = null;
      render();
    }
  });

  svg.addEventListener("click", (e) => {
    const el = e.target.closest(".landmarks [data-step-id], .trace-steps [data-step-id]");
    if (el && svg.contains(el)) {
      const id = el.dataset.stepId;
      if (!id) {
        return;
      }
      selectedId = selectedId === id ? null : id;
      if (selectedId) {
        const row = rowById.get(selectedId);
        if (row) {
          row.scrollIntoView({ block: "nearest" });
        }
      }
      render();
      return;
    }
    if (selectedId !== null) {
      selectedId = null;
      render();
    }
  });

  document.addEventListener("keydown", (e) => {
    if (e.key !== "Escape") {
      return;
    }
    const tag = e.target && e.target.tagName;
    if (tag === "INPUT" || tag === "SELECT" || tag === "TEXTAREA") {
      return;
    }
    if (selectedId !== null || focusedId !== null || hoveredId !== null) {
      selectedId = null;
      focusedId = null;
      hoveredId = null;
      render();
    }
  });

  if (clearBtn) {
    clearBtn.addEventListener("click", () => {
      selectedId = null;
      focusedId = null;
      hoveredId = null;
      render();
    });
  }
})();
