import { test as base, expect, type Page } from "@playwright/test";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

// Upload flow stubs: the real UI POSTs FormData to /api/upload and
// closes the modal on success. See ui/src/routes/documents/UploadModal.tsx.
const API_PATH = /^\/api\//;
const MCP_PATH = /^\/mcp\//;

async function stubUploadAccept(page: Page) {
  await page.route(
    (url) => API_PATH.test(url.pathname),
    async (route) => {
      const req = route.request();
      const p = new URL(req.url()).pathname;

      // Upload accept — return a synthetic success. The real endpoint
      // is POST /api/upload?project=...; see UploadModal.tsx:20.
      if (req.method() === "POST" && /\/upload$/.test(p)) {
        return route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ ok: true, count: 1 }),
        });
      }
      // Post-upload list refresh returns the uploaded document so the
      // documents grid reflects the write.
      if (/\/documents(\?|$)/.test(p)) {
        return route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify([
            { id: "doc-1", title: "fixture.md", doc_type: "md" },
          ]),
        });
      }
      if (/\/stats$/.test(p)) {
        return route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ notes: 0, documents: 1, entities: 0, relationships: 0 }),
        });
      }
      if (/\/notes(\?|$)/.test(p) || /\/activity(\?|$)/.test(p)) {
        return route.fulfill({ status: 200, contentType: "application/json", body: "[]" });
      }
      return route.fulfill({ status: 200, contentType: "application/json", body: "{}" });
    },
  );
  await page.route(
    (url) => MCP_PATH.test(url.pathname),
    (route) => route.fulfill({ status: 200, contentType: "application/json", body: "{}" }),
  );
}

const test = base.extend<{ uploadPage: Page }>({
  uploadPage: async ({ page }, use) => {
    await stubUploadAccept(page);
    await use(page);
  },
});

test.describe("upload happy-path", () => {
  test("user can open the upload modal, pick a file, and see the modal close on success", async ({ uploadPage: page }) => {
    await page.goto("/docs");
    await expect(page.locator("main#main")).toBeVisible();

    // Open the upload affordance — UploadModal.tsx has <Button>Upload</Button>.
    const uploadButton = page.getByRole("button", { name: /^upload$/i }).first();
    await expect(uploadButton).toBeVisible({ timeout: 5_000 });
    await uploadButton.click();

    // DialogTitle is "Upload documents" — verify the modal actually opened.
    await expect(page.getByRole("heading", { name: /upload documents/i })).toBeVisible();

    // The modal renders a bare <input type="file"> (no button). Set files
    // directly on the input; onChange fires onFiles which POSTs and then
    // calls onOpenChange(false), closing the dialog.
    const fileInput = page.locator('input[type="file"]');
    await fileInput.setInputFiles(path.join(__dirname, "fixtures/fixture.md"));

    // Success surface: the modal closes when the POST resolves, so the
    // DialogTitle should disappear within a few seconds.
    await expect(
      page.getByRole("heading", { name: /upload documents/i }),
    ).toBeHidden({ timeout: 5_000 });
  });
});
