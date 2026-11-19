(() => {
  "use strict";

  function ancestorIds(id, parentById, validIds) {
    const ancestors = [];
    const visited = new Set([id]);
    let cur = parentById.get(id);
    while (cur && cur !== "0" && validIds.has(cur) && !visited.has(cur)) {
      visited.add(cur);
      ancestors.push(cur);
      cur = parentById.get(cur);
    }
    return ancestors;
  }

  function resolveInitialSelection(initialSelection, metaById) {
    if (!initialSelection || !initialSelection.id) {
      return null;
    }
    const meta = metaById.get(initialSelection.id);
    if (meta && meta.op === initialSelection.op && meta.label === initialSelection.label) {
      return initialSelection.id;
    }
    return null;
  }

  function mountTraceWorkbench(root, initialSelection) {
    if (!root) {
      return null;
    }

    const svg = root.querySelector("svg.diagram");
    const tbody = root.querySelector("table.trace-table tbody");
    if (!svg || !tbody) {
      return null;
    }

    const listenerController = new AbortController();
    const signal = listenerController.signal;

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
    const metaById = new Map();
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
      metaById.set(id, {
        id,
        op: row.dataset.stepOp || "",
        label: row.dataset.stepLabel || "",
      });
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

    let selectedId = resolveInitialSelection(initialSelection, metaById);
    let hoveredId = null;
    let focusedId = null;

    let activeEls = [];
    let ancestorEls = [];
    let selectedRow = null;
    let selectedBtn = null;

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
        for (const ancId of ancestorIds(activeId, parentById, rowById)) {
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

    if (selectedId) {
      render();
    }

    tbody.addEventListener("mouseover", (e) => {
      const row = e.target.closest("tr[data-step-id]");
      const nextId = row && tbody.contains(row) ? row.dataset.stepId : null;
      if (nextId !== hoveredId) {
        hoveredId = nextId;
        render();
      }
    }, { signal });

    tbody.addEventListener("mouseleave", () => {
      if (hoveredId !== null) {
        hoveredId = null;
        render();
      }
    }, { signal });

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
    }, { signal });

    tbody.addEventListener("focusout", (e) => {
      if (e.relatedTarget && tbody.contains(e.relatedTarget)) {
        return;
      }
      if (focusedId !== null) {
        focusedId = null;
        render();
      }
    }, { signal });

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
    }, { signal });

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
    }, { signal });

    svg.addEventListener("mouseover", (e) => {
      const el = e.target.closest(".landmarks [data-step-id], .trace-steps [data-step-id]");
      const nextId = el && svg.contains(el) ? el.dataset.stepId : null;
      if (nextId !== hoveredId) {
        hoveredId = nextId;
        render();
      }
    }, { signal });

    svg.addEventListener("mouseleave", () => {
      if (hoveredId !== null) {
        hoveredId = null;
        render();
      }
    }, { signal });

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
    }, { signal });

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
    }, { signal });

    if (clearBtn) {
      clearBtn.addEventListener("click", () => {
        selectedId = null;
        focusedId = null;
        hoveredId = null;
        render();
      }, { signal });
    }

    return {
      getSelection() {
        return selectedId ? (metaById.get(selectedId) || null) : null;
      },
      destroy() {
        listenerController.abort();
      },
    };
  }

  function buildFormURL(form) {
    const u = new URL(form.getAttribute("action") || "/bike", window.location.href);
    const params = new URLSearchParams();
    for (const [k, v] of new FormData(form)) {
      params.append(k, String(v));
    }
    u.search = params.toString();
    return u.pathname + u.search;
  }

  function historyURLForError(requestUrl, baseHref) {
    const u = new URL(requestUrl, baseHref);
    const fork = u.searchParams.get("fork");
    const prevFork = u.searchParams.get("prev_fork");
    if (prevFork !== null && prevFork === fork) {
      u.searchParams.delete("prev_fork");
    }
    return u.pathname + u.search;
  }

  function findBadInput(form, preferredTarget) {
    if (preferredTarget && !preferredTarget.disabled && preferredTarget.validity && preferredTarget.validity.badInput) {
      return preferredTarget;
    }
    for (const el of form.elements) {
      if (!el.disabled && el.validity && el.validity.badInput) {
        return el;
      }
    }
    return null;
  }

  function escapeSelectorValue(value) {
    if (typeof CSS !== "undefined" && typeof CSS.escape === "function") {
      return CSS.escape(value);
    }
    return value.replace(/["\\]/g, "\\$&");
  }

  function captureFocus(container) {
    const active = document.activeElement;
    if (!active || !container.contains(active)) {
      return null;
    }
    if (active.id) {
      return { id: active.id };
    }
    const stepBtn = active.closest && active.closest("button[data-step-select]");
    if (stepBtn && stepBtn.dataset.stepSelect) {
      return { stepSelect: stepBtn.dataset.stepSelect };
    }
    return null;
  }

  function isFocusableTarget(el) {
    if (!el || el.disabled) {
      return false;
    }
    if (el.closest && el.closest("[hidden], fieldset[disabled]")) {
      return false;
    }
    return typeof el.focus === "function";
  }

  function restoreFocus(container, savedFocus, status) {
    if (!savedFocus) {
      return;
    }
    if (status === 400 && savedFocus.id === "ctrl-submit") {
      const invalidEl = container.querySelector('[aria-invalid="true"]');
      if (isFocusableTarget(invalidEl)) {
        invalidEl.focus({ preventScroll: true });
        return;
      }
    }
    let target = null;
    if (savedFocus.id) {
      target = container.querySelector("#" + escapeSelectorValue(savedFocus.id));
    } else if (savedFocus.stepSelect) {
      target = container.querySelector('button[data-step-select="' + escapeSelectorValue(savedFocus.stepSelect) + '"]');
    }
    if (isFocusableTarget(target)) {
      target.focus({ preventScroll: true });
    }
  }

  function createBikeController(host) {
    let inFlightController = null;
    let inFlightUrl = null;
    let inFlightSource = null;
    let requestSeq = 0;
    let editVersion = 0;
    let staleFormAfterAbort = false;
    let currentLocationKey = host.getLocation();
    let presetPointerActive = false;

    function clearInFlight(controller) {
      if (inFlightController === controller) {
        inFlightController = null;
        inFlightUrl = null;
        inFlightSource = null;
      }
    }

    async function navigate(targetUrl, { pushHistory, source }) {
      const baseHref = host.origin + "/";
      const parsed = new URL(targetUrl, baseHref);
      if (parsed.origin !== host.origin || parsed.pathname !== "/bike") {
        host.navigateHard(parsed.href, true);
        return;
      }
      const requestUrl = parsed.pathname + parsed.search;

      // Pressing Enter in an edited input fires both change and submit with
      // the same serialized query; avoid aborting the identical in-flight fetch.
      if (source === "submit" && inFlightController && inFlightUrl === requestUrl) {
        return;
      }

      if (inFlightController) {
        inFlightController.abort();
      }
      staleFormAfterAbort = false;
      const startedEditVersion = editVersion;
      const controller = new AbortController();
      inFlightController = controller;
      inFlightUrl = requestUrl;
      inFlightSource = source;
      const seq = ++requestSeq;
      host.setBusy(true);

      let result;
      try {
        result = await host.fetchPage(requestUrl, controller.signal);
      } catch (err) {
        if (seq !== requestSeq || (err && err.name === "AbortError")) {
          return;
        }
        clearInFlight(controller);
        host.setBusy(false);
        host.navigateHard(requestUrl, pushHistory);
        return;
      }

      if (seq !== requestSeq) {
        return;
      }

      if (!result || (result.status !== 200 && result.status !== 400)) {
        clearInFlight(controller);
        host.setBusy(false);
        host.navigateHard(requestUrl, pushHistory);
        return;
      }

      const parsedPage = host.parsePage(result.html);
      if (!parsedPage) {
        clearInFlight(controller);
        host.setBusy(false);
        host.navigateHard(requestUrl, pushHistory);
        return;
      }

      if (source !== "popstate" && editVersion !== startedEditVersion) {
        clearInFlight(controller);
        host.setBusy(false);
        staleFormAfterAbort = true;
        return;
      }

      clearInFlight(controller);
      host.setBusy(false);
      host.applyPage(parsedPage, result.status);

      if (pushHistory) {
        const currentUrl = host.getLocation();
        const canonicalUrl = parsedPage.canonicalUrl || "";
        const nextHistoryUrl = (result.status === 200 && canonicalUrl) ? canonicalUrl : historyURLForError(requestUrl, baseHref);
        if (nextHistoryUrl !== currentUrl) {
          host.pushLocation(nextHistoryUrl);
        }
        currentLocationKey = host.getLocation();
      }
    }

    return {
      navigate,
      handlePresetPointerDown(active) {
        presetPointerActive = Boolean(active);
      },
      handlePresetPointerEnd() {
        presetPointerActive = false;
      },
      handleInput() {
        editVersion++;
        if (inFlightController && inFlightSource !== "popstate") {
          const controller = inFlightController;
          requestSeq++;
          clearInFlight(controller);
          host.setBusy(false);
          controller.abort();
          staleFormAfterAbort = true;
        }
      },
      handleFocusOut() {
        if (!staleFormAfterAbort || presetPointerActive) {
          return;
        }
        if (host.findBadInput(null)) {
          return;
        }
        staleFormAfterAbort = false;
        return navigate(host.readFormURL(), { pushHistory: true, source: "change" });
      },
      handleSubmit() {
        const badEl = host.findBadInput(null);
        if (badEl) {
          host.reportInvalid(badEl);
          return;
        }
        return navigate(host.readFormURL(), { pushHistory: true, source: "submit" });
      },
      handleChange(preferredTarget) {
        if (presetPointerActive) {
          return;
        }
        const badEl = host.findBadInput(preferredTarget || null);
        if (badEl) {
          host.reportInvalid(badEl);
          return;
        }
        return navigate(host.readFormURL(), { pushHistory: true, source: "change" });
      },
      handlePresetClick(href) {
        presetPointerActive = false;
        return navigate(href, { pushHistory: true, source: "preset" });
      },
      handlePopState() {
        const nextKey = host.getLocation();
        if (nextKey === currentLocationKey) {
          return;
        }
        currentLocationKey = nextKey;
        return navigate(host.origin + nextKey, { pushHistory: false, source: "popstate" });
      },
    };
  }

  function mountBikeWorkbench(main) {
    let activeInspector = null;
    function mountInspector(selection) {
      const traceRoot = main.querySelector("[data-trace-workbench]");
      activeInspector = traceRoot ? mountTraceWorkbench(traceRoot, selection) : null;
    }
    mountInspector(null);

    const controller = createBikeController({
      origin: window.location.origin,
      getLocation() {
        return window.location.pathname + window.location.search;
      },
      pushLocation(url) {
        window.history.pushState({ bikeWorkbench: true }, "", url);
      },
      navigateHard(url, pushHistory) {
        if (pushHistory) {
          window.location.assign(url);
        } else {
          window.location.reload();
        }
      },
      setBusy(busy) {
        if (busy) {
          main.setAttribute("aria-busy", "true");
        } else {
          main.removeAttribute("aria-busy");
        }
      },
      async fetchPage(requestUrl, signal) {
        const response = await fetch(requestUrl, {
          method: "GET",
          headers: { Accept: "text/html" },
          signal,
        });
        if (response.status !== 200 && response.status !== 400) {
          return { status: response.status, html: "" };
        }
        const html = await response.text();
        return { status: response.status, html };
      },
      parsePage(html) {
        const doc = new DOMParser().parseFromString(html, "text/html");
        const incomingMain = doc.querySelector("main[data-bike-workbench]");
        if (!incomingMain) {
          return null;
        }
        return {
          incomingMain,
          canonicalUrl: incomingMain.getAttribute("data-canonical-url") || "",
          title: doc.title || "",
        };
      },
      applyPage(parsedPage, status) {
        const savedFocus = captureFocus(main);
        const savedSelection = activeInspector ? activeInspector.getSelection() : null;
        if (activeInspector) {
          activeInspector.destroy();
          activeInspector = null;
        }

        if (parsedPage.canonicalUrl) {
          main.setAttribute("data-canonical-url", parsedPage.canonicalUrl);
        } else {
          main.removeAttribute("data-canonical-url");
        }

        main.replaceChildren(...Array.from(parsedPage.incomingMain.childNodes));
        if (parsedPage.title) {
          document.title = parsedPage.title;
        }

        mountInspector(savedSelection);
        restoreFocus(main, savedFocus, status);
      },
      readFormURL() {
        const form = main.querySelector("form.controls");
        return form ? buildFormURL(form) : (window.location.pathname + window.location.search);
      },
      findBadInput(preferredTarget) {
        const form = main.querySelector("form.controls");
        return form ? findBadInput(form, preferredTarget) : null;
      },
      reportInvalid(el) {
        if (el && typeof el.reportValidity === "function") {
          el.reportValidity();
        }
      },
    });

    main.addEventListener("pointerdown", (e) => {
      if (e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) {
        controller.handlePresetPointerDown(false);
        return;
      }
      const link = e.target && e.target.closest ? e.target.closest("a[data-preset]") : null;
      controller.handlePresetPointerDown(Boolean(link && main.contains(link)));
    });

    window.addEventListener("pointerup", () => {
      controller.handlePresetPointerEnd();
    });

    window.addEventListener("pointercancel", () => {
      controller.handlePresetPointerEnd();
    });

    main.addEventListener("input", (e) => {
      const form = e.target && e.target.closest ? e.target.closest("form.controls") : null;
      if (!form || !main.contains(form)) {
        return;
      }
      controller.handleInput();
    });

    main.addEventListener("focusout", (e) => {
      const form = e.target && e.target.closest ? e.target.closest("form.controls") : null;
      if (!form || !main.contains(form)) {
        return;
      }
      controller.handleFocusOut();
    });

    main.addEventListener("submit", (e) => {
      const form = e.target && e.target.closest ? e.target.closest("form.controls") : null;
      if (!form || !main.contains(form)) {
        return;
      }
      e.preventDefault();
      controller.handleSubmit();
    });

    main.addEventListener("change", (e) => {
      const form = e.target && e.target.closest ? e.target.closest("form.controls") : null;
      if (!form || !main.contains(form)) {
        return;
      }
      controller.handleChange(e.target);
    });

    main.addEventListener("click", (e) => {
      const link = e.target && e.target.closest ? e.target.closest("a[data-preset]") : null;
      if (!link || !main.contains(link)) {
        return;
      }
      if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) {
        return;
      }
      e.preventDefault();
      controller.handlePresetClick(link.href);
    });

    window.addEventListener("popstate", () => {
      controller.handlePopState();
    });
  }

  if (typeof document !== "undefined") {
    const bikeMain = document.querySelector("main[data-bike-workbench]");
    if (bikeMain) {
      mountBikeWorkbench(bikeMain);
    } else {
      const traceRoot = document.querySelector("[data-trace-workbench]");
      if (traceRoot) {
        mountTraceWorkbench(traceRoot, null);
      }
    }
  }

  if (typeof module !== "undefined" && module.exports) {
    module.exports = {
      ancestorIds,
      createBikeController,
      findBadInput,
      historyURLForError,
      resolveInitialSelection,
    };
  }
})();
