import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path) => readFileSync(new URL(path, root), "utf8");

test("manifest configures Ground Control Roasters as a standalone app", () => {
  const manifest = JSON.parse(read("public/manifest.webmanifest"));
  assert.equal(manifest.name, "Ground Control Roasters");
  assert.equal(manifest.display, "standalone");
  assert.equal(manifest.start_url, "/");
  assert.equal(manifest.scope, "/");
  assert.deepEqual(
    manifest.icons.map(({ src, sizes }) => [src, sizes]),
    [["/pwa-192.png", "192x192"], ["/pwa-512.png", "512x512"]],
  );
});

test("HTML advertises manifest and Apple Home Screen metadata", () => {
  const html = read("index.html");
  assert.match(html, /rel="manifest" href="\/manifest\.webmanifest"/);
  assert.match(html, /rel="apple-touch-icon" href="\/apple-touch-icon\.png"/);
  assert.match(html, /name="apple-mobile-web-app-capable" content="yes"/);
  assert.match(html, /name="theme-color" content="#111111"/);
});

test("promotional install prompt is gated to mobile devices", () => {
  const component = read("src/components/InstallPrompt.tsx");
  assert.match(
    component,
    /const handleInstallPrompt[\s\S]*?if \(!isMobileDevice\(\)\) return;[\s\S]*?event\.preventDefault\(\)/,
  );
});

test("service worker never caches authenticated backend requests", () => {
  const worker = read("public/sw.js");
  assert.match(worker, /pathname\.startsWith\("\/api\/"\)/);
  assert.match(worker, /pathname\.startsWith\("\/_\/"\)/);
});

test("required Home Screen icons exist", () => {
  for (const path of ["public/pwa-192.png", "public/pwa-512.png", "public/apple-touch-icon.png"]) {
    assert.equal(existsSync(new URL(path, root)), true, `${path} is missing`);
  }
});
