"use strict";

const { describe, test } = require("node:test");
const assert = require("node:assert/strict");

const {
  ancestorIds,
  createBikeController,
  findBadInput,
  hiddenLabelItems,
  historyURLForError,
  resolveInitialSelection,
  urlWithView,
} = require("./workbench.js");

function createTestHarness(initialLocation = "/bike") {
  let location = initialLocation;
  let formURL = initialLocation;
  const controls = [];
  const fetches = [];
  const busyChanges = [];
  let busy = false;
  const applied = [];
  const pushed = [];
  const hardNavs = [];
  const invalidReported = [];
  const deferred = [];

  const controller = createBikeController({
    origin: "http://127.0.0.1:8081",
    getLocation() {
      return location;
    },
    pushLocation(url) {
      pushed.push(url);
      location = url;
    },
    navigateHard(url, pushHistory) {
      hardNavs.push({ url, pushHistory });
    },
    setBusy(nextBusy) {
      busy = nextBusy;
      busyChanges.push(nextBusy);
    },
    fetchPage(requestUrl, signal) {
      const deferred = Promise.withResolvers();
      const entry = {
        requestUrl,
        signal,
        resolve(value) {
          deferred.resolve(value);
        },
        reject(err) {
          deferred.reject(err);
        },
      };
      fetches.push(entry);
      return deferred.promise;
    },
    parsePage(html) {
      if (html === null) {
        return null;
      }
      return typeof html === "string" ? { canonicalUrl: html } : html;
    },
    applyPage(parsedPage, status) {
      applied.push({ parsedPage, status });
    },
    readFormURL() {
      return formURL;
    },
    findBadInput(preferredTarget) {
      return findBadInput({ elements: controls }, preferredTarget);
    },
    reportInvalid(el) {
      invalidReported.push(el);
    },
    defer(fn) {
      deferred.push(fn);
    },
  });

  return {
    controller,
    controls,
    fetches,
    busyChanges,
    applied,
    pushed,
    hardNavs,
    invalidReported,
    runDeferred() {
      const pending = deferred.splice(0);
      return pending.map((fn) => fn());
    },
    deferredCount() {
      return deferred.length;
    },
    isBusy() {
      return busy;
    },
    setLocation(nextLocation) {
      location = nextLocation;
    },
    setFormURL(nextFormURL) {
      formURL = nextFormURL;
    },
    setControls(nextControls) {
      controls.length = 0;
      controls.push(...nextControls);
    },
  };
}

function abortError() {
  const err = new Error("The operation was aborted");
  err.name = "AbortError";
  return err;
}

describe("pure workbench helpers", () => {
  test("historyURLForError strips prev_fork only when equal to fork", () => {
    assert.equal(
      historyURLForError("/bike?fork=lithic&prev_fork=lithic&cup=0", "http://127.0.0.1:8081/"),
      "/bike?fork=lithic&cup=0",
    );
    assert.equal(
      historyURLForError("/bike?fork=fox34_120&prev_fork=lithic&spacers=-1", "http://127.0.0.1:8081/"),
      "/bike?fork=fox34_120&prev_fork=lithic&spacers=-1",
    );
    assert.equal(
      historyURLForError("/bike?cup=0&spacers=20", "http://127.0.0.1:8081/"),
      "/bike?cup=0&spacers=20",
    );
  });

  test("urlWithView replaces only the view and keeps pending build values", () => {
    const base = "http://127.0.0.1:8081/";
    assert.equal(
      urlWithView("/bike?view=bike&fork=fox34_120&prev_fork=lithic&spacers=35", "stem", base),
      "/bike?view=stem&fork=fox34_120&prev_fork=lithic&spacers=35",
    );
    assert.equal(urlWithView("/bike", "seat", base), "/bike?view=seat");
    assert.equal(urlWithView("http://127.0.0.1:8081/bike?spacers=30", "bike", base), "/bike?spacers=30&view=bike");
  });

  test("hiddenLabelItems hides labels that overlap a higher-priority label already shown", () => {
    const view = { x: 0, y: 0, width: 1000, height: 500 };
    const box = (x, y) => ({ x, y, width: 100, height: 20 });
    const items = [
      { name: "SpacerTop", priority: 3, order: 0, box: box(10, 10) },
      { name: "BarClamp", priority: 2, order: 1, box: box(50, 15) },
      { name: "CoverTop", priority: 3, order: 2, box: box(10, 40) },
      { name: "SteererClamp", priority: 3, order: 3, box: box(20, 45) },
      { name: "BB", priority: 1, order: 4, box: box(500, 300) },
      // Outside the view: never shown, so it blocks nothing.
      { name: "FrontAxle", priority: 1, order: 5, box: box(1200, 300) },
      { name: "RearAxle", priority: 1, order: 6, box: null },
    ];
    const names = (list) => list.map((item) => item.name);

    // BarClamp (priority 2) beats SpacerTop (priority 3) despite coming later;
    // among equal priorities, document order wins (CoverTop over SteererClamp).
    assert.deepEqual(names(hiddenLabelItems(items, view)), ["SpacerTop", "SteererClamp"]);

    // A small gap between boxes still counts as touching.
    const touching = [
      { name: "a", priority: 1, order: 0, box: box(0, 0) },
      { name: "b", priority: 1, order: 1, box: box(0, 21) },
    ];
    assert.deepEqual(names(hiddenLabelItems(touching, view)), ["b"]);
    const apart = [
      { name: "a", priority: 1, order: 0, box: box(0, 0) },
      { name: "b", priority: 1, order: 1, box: box(0, 30) },
    ];
    assert.deepEqual(hiddenLabelItems(apart, view), []);
  });

  test("findBadInput checks preferred target and other enabled controls", () => {
    const spacers = { id: "ctrl-spacers", disabled: false, validity: { badInput: false } };
    const stemLength = { id: "ctrl-stem-length", disabled: false, validity: { badInput: true } };
    const disabledBad = { id: "ctrl-disabled", disabled: true, validity: { badInput: true } };
    const form = { elements: [disabledBad, spacers, stemLength] };

    assert.equal(findBadInput(form, spacers), stemLength);
    assert.equal(findBadInput(form, stemLength), stemLength);

    stemLength.validity.badInput = false;
    assert.equal(findBadInput(form, spacers), null);
  });

  test("ancestorIds walks parent chain to root and guards against cycles", () => {
    const parentById = new Map([
      ["1", "0"],
      ["2", "1"],
      ["3", "2"],
      ["4", "99"],
      ["10", "11"],
      ["11", "10"],
    ]);
    const validIds = new Set(["1", "2", "3", "4", "10", "11"]);

    assert.deepEqual(ancestorIds("3", parentById, validIds), ["2", "1"]);
    assert.deepEqual(ancestorIds("4", parentById, validIds), []);
    assert.deepEqual(ancestorIds("10", parentById, validIds), ["11"]);
  });

  test("resolveInitialSelection requires matching id, op, and label", () => {
    const metaById = new Map([
      ["24", { id: "24", op: "advance", label: "spacers" }],
    ]);

    assert.equal(resolveInitialSelection({ id: "24", op: "advance", label: "spacers" }, metaById), "24");
    assert.equal(resolveInitialSelection({ id: "24", op: "turn", label: "spacers" }, metaById), null);
    assert.equal(resolveInitialSelection({ id: "24", op: "advance", label: "stem length" }, metaById), null);
    assert.equal(resolveInitialSelection({ id: "99", op: "advance", label: "spacers" }, metaById), null);
    assert.equal(resolveInitialSelection(null, metaById), null);
  });
});

describe("createBikeController", () => {
  test("typing during a pending request aborts it and preserves uncommitted edits", async () => {
    const h = createTestHarness("/bike");
    h.setFormURL("/bike?fork=lithic&spacers=35&stem_length=60.0");

    const p1 = h.controller.handleChange();
    assert.equal(h.fetches.length, 1);
    assert.equal(h.isBusy(), true);

    h.controller.handleInput();
    assert.equal(h.fetches[0].signal.aborted, true);
    assert.equal(h.isBusy(), false);

    // Even if the superseded transport ignores signal.aborted and resolves late,
    // it must not overwrite the DOM or push history.
    h.fetches[0].resolve({
      status: 200,
      html: "/bike?fork=lithic&spacers=35.0&stem_length=60.0",
    });
    await p1;
    assert.equal(h.applied.length, 0);
    assert.equal(h.pushed.length, 0);

    // Completing the new edit sends both spacers=35 and stem_length=80 together.
    h.setFormURL("/bike?fork=lithic&spacers=35&stem_length=80");
    const p2 = h.controller.handleChange();
    assert.equal(h.fetches.length, 2);
    assert.equal(h.fetches[1].requestUrl, "/bike?fork=lithic&spacers=35&stem_length=80");

    const canonical = "/bike?fork=lithic&spacers=35.0&stem_length=80.0";
    h.fetches[1].resolve({ status: 200, html: canonical });
    await p2;

    assert.equal(h.applied.length, 1);
    assert.equal(h.applied[0].parsedPage.canonicalUrl, canonical);
    assert.deepEqual(h.pushed, [canonical]);
    assert.equal(h.isBusy(), false);
  });

  test("focusout resyncs after an aborted pending fetch if the user reverts the edited field", async () => {
    const h = createTestHarness("/bike");
    const stemLength = { id: "ctrl-stem-length", disabled: false, validity: { badInput: false } };
    h.setControls([stemLength]);

    // Normal change leaves no stale abort, so focusout is a no-op.
    h.setFormURL("/bike?spacers=35");
    const normal = h.controller.handleChange();
    h.fetches[0].resolve({ status: 200, html: "/bike?spacers=35.0" });
    await normal;
    assert.equal(h.controller.handleFocusOut(), undefined);
    assert.equal(h.fetches.length, 1);

    // Start spacers=30, abort via input, then revert stem_length so change does not fire.
    h.setFormURL("/bike?spacers=30");
    const pending = h.controller.handleChange();
    h.controller.handleInput();
    h.fetches[1].reject(abortError());
    await pending;

    // While badInput is active or a preset pointer is down, focusout must wait.
    stemLength.validity.badInput = true;
    assert.equal(h.controller.handleFocusOut(), undefined);
    stemLength.validity.badInput = false;

    h.controller.handlePresetPointerDown(true);
    assert.equal(h.controller.handleFocusOut(), undefined);
    h.controller.handlePresetPointerEnd();
    assert.equal(h.fetches.length, 2);

    // Valid blur without change resyncs the aborted form URL.
    const resync = h.controller.handleFocusOut();
    assert.equal(h.fetches.length, 3);
    assert.equal(h.fetches[2].requestUrl, "/bike?spacers=30");
    h.fetches[2].resolve({ status: 200, html: "/bike?spacers=30.0" });
    await resync;

    assert.equal(h.applied.at(-1).parsedPage.canonicalUrl, "/bike?spacers=30.0");
    assert.equal(h.pushed.at(-1), "/bike?spacers=30.0");
  });

  test("superseded requests arriving late never overwrite newer responses or busy state", async () => {
    const h = createTestHarness("/bike");
    h.setFormURL("/bike?spacers=25");
    const p1 = h.controller.handleChange();

    h.setFormURL("/bike?spacers=30");
    const p2 = h.controller.handleChange();
    assert.equal(h.fetches[0].signal.aborted, true);
    assert.equal(h.fetches[1].signal.aborted, false);
    assert.equal(h.isBusy(), true);

    h.fetches[1].resolve({ status: 200, html: "/bike?spacers=30.0" });
    await p2;
    assert.equal(h.isBusy(), false);
    assert.deepEqual(h.pushed, ["/bike?spacers=30.0"]);

    h.fetches[0].resolve({ status: 200, html: "/bike?spacers=25.0" });
    await p1;
    assert.equal(h.applied.length, 1);
    assert.deepEqual(h.pushed, ["/bike?spacers=30.0"]);

    // Rejecting an aborted first request while a newer request is still pending leaves busy=true.
    h.setFormURL("/bike?spacers=32");
    const p3 = h.controller.handleChange();
    h.setFormURL("/bike?spacers=34");
    const p4 = h.controller.handleChange();
    h.fetches[2].reject(abortError());
    await p3;
    assert.equal(h.isBusy(), true);
    assert.equal(h.hardNavs.length, 0);

    h.fetches[3].resolve({ status: 200, html: "/bike?spacers=34.0" });
    await p4;
    assert.equal(h.isBusy(), false);
  });

  test("badInput in another control blocks change and submit, while presets and popstate still work", async () => {
    const h = createTestHarness("/bike");
    const spacers = { id: "ctrl-spacers", disabled: false, validity: { badInput: false } };
    const stemLength = { id: "ctrl-stem-length", disabled: false, validity: { badInput: true } };
    h.setControls([spacers, stemLength]);
    h.setFormURL("/bike?spacers=25&stem_length=");

    assert.equal(h.controller.handleChange(spacers), undefined);
    assert.equal(h.controller.handleSubmit(), undefined);
    assert.equal(h.fetches.length, 0);
    assert.deepEqual(h.invalidReported, [stemLength, stemLength]);

    const preset = h.controller.handlePresetClick("http://127.0.0.1:8081/bike");
    assert.equal(h.fetches.length, 1);
    h.fetches[0].resolve({ status: 200, html: "/bike?spacers=20.0&stem_length=60.0" });
    await preset;
    assert.equal(h.pushed[0], "/bike?spacers=20.0&stem_length=60.0");

    // Popstate also bypasses invalid current DOM controls to restore the popped build.
    h.setLocation("/bike?spacers=30.0");
    const pop = h.controller.handlePopState();
    assert.equal(h.fetches.length, 2);
    h.fetches[1].resolve({ status: 200, html: "/bike?spacers=30.0" });
    await pop;
    assert.equal(h.applied.length, 2);
  });

  test("Enter change+submit deduplicates identical URLs and supersedes distinct URLs", async () => {
    const h = createTestHarness("/bike");
    h.setFormURL("/bike?spacers=38");

    const pChange = h.controller.handleChange();
    const pSubmitSame = h.controller.handleSubmit();
    assert.equal(h.fetches.length, 1);
    assert.equal(h.fetches[0].signal.aborted, false);

    h.fetches[0].resolve({ status: 200, html: "/bike?spacers=38.0" });
    await Promise.all([pChange, pSubmitSame]);
    assert.equal(h.applied.length, 1);

    h.setFormURL("/bike?spacers=40");
    const pChange2 = h.controller.handleChange();
    h.setFormURL("/bike?spacers=42");
    const pSubmitDiff = h.controller.handleSubmit();
    assert.equal(h.fetches.length, 3);
    assert.equal(h.fetches[1].signal.aborted, true);

    h.fetches[1].reject(abortError());
    h.fetches[2].resolve({ status: 200, html: "/bike?spacers=42.0" });
    await Promise.all([pChange2, pSubmitDiff]);
    assert.equal(h.pushed.at(-1), "/bike?spacers=42.0");
  });

  test("clicking a preset link from a dirty input suppresses blur change and fetches only the preset", async () => {
    const h = createTestHarness("/bike?spacers=35.0");
    h.setFormURL("/bike?spacers=45");

    h.controller.handlePresetPointerDown(true);
    assert.equal(h.controller.handleChange(), undefined);
    h.controller.handlePresetPointerEnd();
    assert.equal(h.fetches.length, 0);

    const preset = h.controller.handlePresetClick("http://127.0.0.1:8081/bike");
    assert.equal(h.fetches.length, 1);
    assert.equal(h.fetches[0].requestUrl, "/bike");
    h.fetches[0].resolve({ status: 200, html: "/bike?spacers=20.0" });
    await preset;

    assert.deepEqual(h.pushed, ["/bike?spacers=20.0"]);
  });

  test("HTTP 400 swaps error view and pushes cleaned error URL without pushing redundant prev_fork", async () => {
    const h = createTestHarness("/bike?fork=lithic&cup=12.0");
    h.setFormURL("/bike?fork=lithic&prev_fork=lithic&cup=0");

    const errNav = h.controller.handleChange();
    h.fetches[0].resolve({ status: 400, html: { canonicalUrl: "" } });
    await errNav;

    assert.equal(h.applied.length, 1);
    assert.equal(h.applied[0].status, 400);
    assert.deepEqual(h.pushed, ["/bike?fork=lithic&cup=0"]);

    h.setFormURL("/bike?fork=lithic&prev_fork=lithic&cup=12");
    const recover = h.controller.handleChange();
    h.fetches[1].resolve({ status: 200, html: "/bike?fork=lithic&cup=12.0" });
    await recover;
    assert.deepEqual(h.pushed, ["/bike?fork=lithic&cup=0", "/bike?fork=lithic&cup=12.0"]);
  });

  test("popstate aborts pending form fetch, refetches popped URL without pushing, and ignores hash-only pops", async () => {
    const h = createTestHarness("/bike?spacers=30.0");

    // Hash-only popstate keeps pathname + search identical and fetches nothing.
    assert.equal(h.controller.handlePopState(), undefined);
    assert.equal(h.fetches.length, 0);

    // In-flight form request gets aborted when user navigates Back/Forward.
    h.setFormURL("/bike?spacers=35");
    const pendingForm = h.controller.handleChange();
    h.setLocation("/bike?spacers=20.0");
    const pop = h.controller.handlePopState();

    assert.equal(h.fetches[0].signal.aborted, true);
    assert.equal(h.fetches[1].requestUrl, "/bike?spacers=20.0");

    // Input events do not abort an active popstate navigation.
    h.controller.handleInput();
    assert.equal(h.fetches[1].signal.aborted, false);

    h.fetches[0].reject(abortError());
    h.fetches[1].resolve({ status: 200, html: "/bike?spacers=20.0" });
    await Promise.all([pendingForm, pop]);

    assert.equal(h.applied.length, 1);
    assert.equal(h.pushed.length, 0);
  });

  test("unexpected statuses, non-abort errors, missing containers, and non-/bike URLs fall back cleanly", async () => {
    const h = createTestHarness("/bike");

    // HTTP 500
    h.setFormURL("/bike?spacers=25");
    const p500 = h.controller.handleChange();
    h.fetches[0].resolve({ status: 500, html: "Internal Error" });
    await p500;
    assert.equal(h.isBusy(), false);
    assert.deepEqual(h.hardNavs.at(-1), { url: "/bike?spacers=25", pushHistory: true });

    // Non-abort network error
    h.setFormURL("/bike?spacers=26");
    const pErr = h.controller.handleChange();
    h.fetches[1].reject(new TypeError("network down"));
    await pErr;
    assert.equal(h.isBusy(), false);
    assert.deepEqual(h.hardNavs.at(-1), { url: "/bike?spacers=26", pushHistory: true });

    // Missing main[data-bike-workbench] on popstate -> reload fallback
    h.setLocation("/bike?spacers=27");
    const pMissing = h.controller.handlePopState();
    h.fetches[2].resolve({ status: 200, html: null });
    await pMissing;
    assert.equal(h.isBusy(), false);
    assert.deepEqual(h.hardNavs.at(-1), { url: "/bike?spacers=27", pushHistory: false });

    // Cross-origin / non-/bike URL bypasses fetch
    await h.controller.handlePresetClick("http://127.0.0.1:8081/");
    await h.controller.handlePresetClick("https://example.com/bike");
    assert.equal(h.fetches.length, 3);
    assert.deepEqual(h.hardNavs.slice(-2), [
      { url: "http://127.0.0.1:8081/", pushHistory: true },
      { url: "https://example.com/bike", pushHistory: true },
    ]);
  });

  test("pressing a view link after an unsubmitted edit sends the edit and the view together", async () => {
    const h = createTestHarness("/bike?view=bike&spacers=20.0");
    h.setFormURL("/bike?view=bike&prev_fork=lithic&spacers=35");

    // pointerdown -> blur/change/focusout -> pointerup -> click.
    h.controller.handleViewPointerDown();
    assert.equal(h.controller.handleChange(), undefined);
    assert.equal(h.controller.handleFocusOut(), undefined);
    h.controller.handlePresetPointerEnd();
    assert.equal(h.deferredCount(), 0, "nothing held, nothing to resume");
    assert.equal(h.fetches.length, 0);

    const nav = h.controller.handleViewClick("stem");
    assert.equal(h.fetches.length, 1);
    assert.equal(h.fetches[0].requestUrl, "/bike?view=stem&prev_fork=lithic&spacers=35");
    h.fetches[0].resolve({ status: 200, html: "/bike?view=stem&spacers=35.0" });
    await nav;
    assert.deepEqual(h.pushed, ["/bike?view=stem&spacers=35.0"]);
  });

  test("keyboard activation after a blur-submitted edit supersedes it with the combined request", async () => {
    const h = createTestHarness("/bike?view=bike&spacers=20.0");
    h.setFormURL("/bike?view=bike&spacers=35");

    // Tab away from the edited input fires change; Enter on the link clicks it.
    const change = h.controller.handleChange();
    assert.equal(h.fetches.length, 1);
    const nav = h.controller.handleViewClick("seat");
    assert.equal(h.fetches[0].signal.aborted, true);
    assert.equal(h.fetches[1].requestUrl, "/bike?view=seat&spacers=35");

    // The superseded change response arriving late cannot overwrite the view.
    h.fetches[0].resolve({ status: 200, html: "/bike?view=bike&spacers=35.0" });
    await change;
    assert.equal(h.applied.length, 0);

    h.fetches[1].resolve({ status: 200, html: "/bike?view=seat&spacers=35.0" });
    await nav;
    assert.equal(h.applied.length, 1);
    assert.deepEqual(h.pushed, ["/bike?view=seat&spacers=35.0"]);
  });

  test("pressing a view link holds an in-flight control request so its swap cannot eat the click", async () => {
    const h = createTestHarness("/bike?view=bike&fork=lithic");
    const forkChange = "/bike?view=bike&fork=fox34_120&prev_fork=lithic&spacers=30";
    h.setFormURL(forkChange);
    const change = h.controller.handleChange();
    assert.equal(h.isBusy(), true);

    h.controller.handleViewPointerDown();
    assert.equal(h.fetches[0].signal.aborted, true);
    assert.equal(h.isBusy(), false);

    // A late response for the held request must not swap the page mid-press.
    h.fetches[0].resolve({ status: 200, html: "/bike?view=bike&fork=fox34_120&spacers=30.0" });
    await change;
    assert.equal(h.applied.length, 0);

    h.controller.handlePresetPointerEnd();
    const nav = h.controller.handleViewClick("stem");
    // The click claimed the held request, so the deferred resume does nothing.
    h.runDeferred();
    assert.equal(h.fetches.length, 2);
    assert.equal(h.fetches[1].requestUrl, "/bike?view=stem&fork=fox34_120&prev_fork=lithic&spacers=30");

    h.fetches[1].resolve({ status: 200, html: "/bike?view=stem&fork=fox34_120&spacers=30.0" });
    await nav;
    assert.deepEqual(h.pushed, ["/bike?view=stem&fork=fox34_120&spacers=30.0"]);
  });

  test("a view press released without a click resumes the held request unchanged", async () => {
    const h = createTestHarness("/bike?view=seat");
    h.setFormURL("/bike?view=seat&spacers=30");
    const change = h.controller.handleChange();
    h.controller.handleViewPointerDown();
    h.fetches[0].reject(abortError());
    await change;

    h.controller.handlePresetPointerEnd();
    assert.equal(h.fetches.length, 1);
    const [resume] = h.runDeferred();
    assert.equal(h.fetches.length, 2);
    assert.equal(h.fetches[1].requestUrl, "/bike?view=seat&spacers=30");
    h.fetches[1].resolve({ status: 200, html: "/bike?view=seat&spacers=30.0" });
    await resume;
    assert.deepEqual(h.pushed, ["/bike?view=seat&spacers=30.0"]);
  });

  test("switching view while a preset loads applies the view to the preset build", async () => {
    const h = createTestHarness("/bike?view=bike&spacers=35.0");
    h.setFormURL("/bike?view=bike&spacers=35");
    const preset = h.controller.handlePresetClick("http://127.0.0.1:8081/bike?view=bike&spacers=20.0");
    const nav = h.controller.handleViewClick("stem");
    assert.equal(h.fetches[0].signal.aborted, true);
    assert.equal(h.fetches[1].requestUrl, "/bike?view=stem&spacers=20.0");
    h.fetches[0].reject(abortError());
    h.fetches[1].resolve({ status: 200, html: "/bike?view=stem&spacers=20.0" });
    await Promise.all([preset, nav]);
    assert.deepEqual(h.pushed, ["/bike?view=stem&spacers=20.0"]);
  });

  test("an unparseable control blocks view navigation; a server error keeps the requested view", async () => {
    const h = createTestHarness("/bike?view=bike");
    const spacers = { id: "ctrl-spacers", disabled: false, validity: { badInput: true } };
    h.setControls([spacers]);
    h.setFormURL("/bike?view=bike&spacers=");

    h.controller.handleViewPointerDown();
    h.controller.handlePresetPointerEnd();
    assert.equal(h.controller.handleViewClick("seat"), undefined);
    assert.equal(h.fetches.length, 0);
    assert.deepEqual(h.invalidReported, [spacers]);
    assert.equal(h.applied.length, 0);

    spacers.validity.badInput = false;
    h.setFormURL("/bike?view=bike&prev_fork=lithic&fork=lithic&spacers=-1");
    const nav = h.controller.handleViewClick("seat");
    h.fetches[0].resolve({ status: 400, html: { canonicalUrl: "" } });
    await nav;
    assert.equal(h.applied[0].status, 400);
    assert.deepEqual(h.pushed, ["/bike?view=seat&fork=lithic&spacers=-1"]);
  });
});
