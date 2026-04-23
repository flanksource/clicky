import { expect, test, type APIRequestContext, type Locator, type Page } from "@playwright/test";

const stackRow = (page: Page, name: string) =>
  page.locator("tbody tr").filter({ hasText: name }).first();
const explorerSearch = (page: Page) => page.getByPlaceholder("Search api explorer...");
const responseBody = (scope: Page | Locator) =>
  scope.locator('[aria-label="Response body"]');

function escapeRegExp(value: string) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function explorerCard(page: Page, path: string) {
  return page.locator("a").filter({
    has: page.locator("code").filter({
      hasText: new RegExp(`^${escapeRegExp(path)}$`),
    }),
  });
}

function waitForJson(page: Page, pathname: string, expected: Record<string, string> = {}) {
  return page.waitForResponse((response) => {
    const url = new URL(response.url());
    if (url.pathname !== pathname || !response.ok()) {
      return false;
    }
    if (url.searchParams.get("__lookup") === "filters") {
      return false;
    }
    return Object.entries(expected).every(
      ([key, value]) => url.searchParams.get(key) === value,
    );
  });
}

function waitForLookup(page: Page, pathname: string, expected: Record<string, string>) {
  return page.waitForResponse((response) => {
    const url = new URL(response.url());
    if (url.pathname !== pathname || !response.ok()) {
      return false;
    }
    if (url.searchParams.get("__lookup") !== "filters") {
      return false;
    }
    return Object.entries(expected).every(
      ([key, value]) => url.searchParams.get(key) === value,
    );
  });
}

async function resetDemoStore(request: APIRequestContext) {
  const response = await request.post("/api/v1/stack/seed");
  expect(response.ok()).toBeTruthy();
  expect(await response.text()).toContain("reset demo store with 3 stacks and 3 clusters");
}

test.describe("entity demo", () => {
  test.beforeEach(async ({ request }) => {
    await resetDemoStore(request);
  });

  test("boots from / into stacks and narrows rows with the live lookup filter", async ({
    page,
  }) => {
    const openApi = waitForJson(page, "/api/openapi.json");
    const list = waitForJson(page, "/api/v1/stack");
    const lookup = waitForLookup(page, "/api/v1/stack", {});

    await page.goto("/", { waitUntil: "domcontentloaded" });
    await Promise.all([openApi, list, lookup]);

    await expect(page).toHaveURL(/\/stacks$/);
    await expect(page.getByRole("heading", { name: "Stacks" })).toBeVisible();
    await expect(page.getByRole("link", { name: "Stacks", exact: true })).toBeVisible();
    await expect(page.getByRole("link", { name: "Clusters" })).toBeVisible();
    await expect(page.getByRole("link", { name: "Admin — Stacks" })).toBeVisible();
    await expect(page.getByRole("link", { name: "API Explorer" })).toBeVisible();

    await expect(stackRow(page, "checkout")).toBeVisible();
    await expect(stackRow(page, "billing")).toBeVisible();
    await expect(page.getByText("marketing-site")).toHaveCount(0);
    await expect(page.getByRole("button", { name: /time range filter/i })).toBeVisible();

    const filteredLookup = waitForLookup(page, "/api/v1/stack", {
      team: "team/platform",
    });
    await page.getByLabel("Team").fill("team/platform");
    await filteredLookup;

    const filteredList = page.waitForResponse((response) => {
      const url = new URL(response.url());
      return (
        url.pathname === "/api/v1/stack" &&
        response.ok() &&
        !url.searchParams.has("__lookup") &&
        url.searchParams.get("team") === "team/platform"
      );
    });
    await page.getByRole("button", { name: "Apply" }).click();
    await filteredList;

    await expect(stackRow(page, "checkout")).toBeVisible();
    await expect(stackRow(page, "billing")).toHaveCount(0);
    await expect(page.getByText("marketing-site")).toHaveCount(0);
  });

  test("renders the special time range filter and applies from/to over the real stack list", async ({
    page,
  }) => {
    const stacks = waitForJson(page, "/api/v1/stack");
    const lookup = waitForLookup(page, "/api/v1/stack", {});

    await page.goto("/stacks", { waitUntil: "domcontentloaded" });
    await Promise.all([stacks, lookup]);

    await expect(page.getByRole("button", { name: /time range filter/i })).toBeVisible();
    await expect(stackRow(page, "checkout")).toBeVisible();
    await expect(stackRow(page, "billing")).toBeVisible();

    const rangedLookup = waitForLookup(page, "/api/v1/stack", {
      from: "now-24h",
      to: "now",
    });
    await page.getByRole("button", { name: /time range filter/i }).click();
    await page.getByRole("button", { name: /last 24 hours/i }).click();
    await rangedLookup;

    const rangedList = waitForJson(page, "/api/v1/stack", {
      from: "now-24h",
      to: "now",
    });
    await page.getByRole("button", { name: "Apply" }).click();
    await rangedList;

    await expect(stackRow(page, "checkout")).toBeVisible();
    await expect(stackRow(page, "billing")).toHaveCount(0);

    const adminList = waitForJson(page, "/api/v1/admin/stack");
    await page.goto("/admin-stacks", { waitUntil: "domcontentloaded" });
    await adminList;
    await expect(page.getByRole("button", { name: /time range filter/i })).toBeVisible();
  });

  test("shows cluster rows and the real cluster endpoint list", async ({ page }) => {
    const clusters = waitForJson(page, "/api/v1/catalog/cluster");

    await page.goto("/clusters", { waitUntil: "domcontentloaded" });
    await clusters;

    await expect(page.getByRole("heading", { name: "Clusters" })).toBeVisible();
    await expect(stackRow(page, "shared-eu1")).toBeVisible();
    await expect(stackRow(page, "payments-us1")).toBeVisible();

    await page.getByRole("button", { name: "Endpoint list view" }).click();

    await expect(page.getByText("/api/v1/catalog/cluster/{id}")).toBeVisible();
    await expect(page.getByText("Get a cluster by ID")).toBeVisible();
  });

  test("clicking a list row navigates to detail and locks the row action id", async ({
    page,
  }) => {
    const stacks = waitForJson(page, "/api/v1/stack");

    await page.goto("/stacks", { waitUntil: "domcontentloaded" });
    await stacks;

    await Promise.all([
      page.waitForURL(/\/entity\/stacks\/stk-001$/),
      stackRow(page, "checkout").getByRole("link", { name: "stk-001" }).click(),
    ]);

    await expect(responseBody(page).first()).toContainText("stk-001");
    await expect(responseBody(page).first()).toContainText("checkout");
    await expect(page.getByRole("radiogroup", { name: /clicky view mode/i })).toBeVisible();
    await expect(page.getByRole("button", { name: /^download$/i })).toBeVisible();

    await page.getByRole("button", { name: /restart/i }).click();

    const dialog = page.getByRole("dialog");
    await expect(dialog.getByLabel("Id")).toHaveValue("stk-001");
    await expect(dialog.getByLabel("Id")).toBeDisabled();

    const actionRequest = page.waitForResponse((response) => {
      const url = new URL(response.url());
      return (
        url.pathname === "/api/v1/stack/stk-001/restart" &&
        response.request().method() === "POST" &&
        response.ok()
      );
    });

    await dialog.getByLabel("Reason").fill("row-action-test");
    await dialog.getByLabel("Drain").uncheck();
    await dialog.getByRole("button", { name: "Execute request" }).click();
    await actionRequest;

    await expect(responseBody(dialog)).toContainText("row-action-test");
  });

  test("surfaces archived admin rows and explorer endpoints from the generated spec", async ({
    page,
  }) => {
    const adminList = waitForJson(page, "/api/v1/admin/stack");

    await page.goto("/admin-stacks", { waitUntil: "domcontentloaded" });
    await adminList;

    await expect(page.getByRole("heading", { name: "Admin — Stacks" })).toBeVisible();
    await expect(stackRow(page, "marketing-site")).toBeVisible();

    await page.goto("/explorer", { waitUntil: "domcontentloaded" });

    await expect(page.getByRole("heading", { name: "API Explorer" })).toBeVisible();
    await expect(page.getByText("/api/v1/stack/{id}/restart")).toBeVisible();
    await expect(page.getByText("/api/v1/admin/stack/{id}/reconcile")).toBeVisible();
  });

  test("API Explorer stack endpoints can be accessed and executed", async ({
    page,
  }) => {
    const openApi = waitForJson(page, "/api/openapi.json");

    await page.goto("/explorer", { waitUntil: "domcontentloaded" });
    await openApi;

    await expect(page.getByRole("heading", { name: "API Explorer" })).toBeVisible();

    const search = explorerSearch(page);

    await search.fill("list stack resources");
    const listCard = explorerCard(page, "/api/v1/stack");
    await expect(listCard).toBeVisible();
    const listRequest = waitForJson(page, "/api/v1/stack");
    await listCard.click();
    await expect(page).toHaveURL(/\/commands\/stack_list$/);
    await page.getByRole("button", { name: "Execute request" }).click();
    await listRequest;
    await expect(responseBody(page)).toContainText("checkout");
    await expect(responseBody(page)).toContainText("billing");
    await expect(page.getByRole("radiogroup", { name: /clicky view mode/i })).toBeVisible();
    await expect(page.getByRole("button", { name: /^download$/i })).toBeVisible();

    await page.goto("/explorer", { waitUntil: "domcontentloaded" });
    await search.fill("get a stack by id");
    const getCard = explorerCard(page, "/api/v1/stack/{id}");
    await expect(getCard).toBeVisible();
    const getRequest = waitForJson(page, "/api/v1/stack/stk-001");
    await getCard.click();
    await expect(page).toHaveURL(/\/commands\/stack_get$/);
    await page.getByLabel("Id").fill("stk-001");
    await page.getByRole("button", { name: "Execute request" }).click();
    await getRequest;
    await expect(responseBody(page)).toContainText("stk-001");
    await expect(responseBody(page)).toContainText("checkout");
    await expect(page.getByRole("radiogroup", { name: /clicky view mode/i })).toBeVisible();
    await expect(page.getByRole("button", { name: /^download$/i })).toBeVisible();

    await page.goto("/explorer", { waitUntil: "domcontentloaded" });
    await search.fill("restart a stack");
    const actionCard = explorerCard(page, "/api/v1/stack/{id}/restart");
    await expect(actionCard).toBeVisible();
    const actionRequest = page.waitForResponse((response) => {
      const url = new URL(response.url());
      return (
        url.pathname === "/api/v1/stack/stk-002/restart" &&
        response.request().method() === "POST" &&
        response.ok()
      );
    });
    await actionCard.click();
    await expect(page).toHaveURL(/\/commands\/stack_restart$/);
    await page.getByLabel("Id").fill("stk-002");
    await page.getByLabel("Reason").fill("explorer-test");
    await page.getByLabel("Drain").uncheck();
    await page.getByRole("button", { name: "Execute request" }).click();
    await actionRequest;
    await expect(responseBody(page)).toContainText("restart");
    await expect(responseBody(page)).toContainText("explorer-test");
  });

  test("reflects a real restart mutation after reloading the stacks table", async ({
    page,
    request,
  }) => {
    const initialList = waitForJson(page, "/api/v1/stack");

    await page.goto("/stacks", { waitUntil: "domcontentloaded" });
    await initialList;

    await expect(stackRow(page, "billing")).toContainText("status:degraded");

    const response = await request.post("/api/v1/stack/stk-002/restart", {
      data: {
        reason: "playwright",
        drain: false,
      },
    });
    expect(response.ok()).toBeTruthy();
    expect(await response.json()).toMatchObject({
      action: "restart",
      ids: ["stk-002"],
      reason: "playwright",
    });

    const refreshedList = waitForJson(page, "/api/v1/stack");
    await page.reload({ waitUntil: "domcontentloaded" });
    await refreshedList;

    await expect(stackRow(page, "billing")).toContainText("status:healthy");
    await expect(stackRow(page, "billing")).not.toContainText("status:degraded");
  });
});
