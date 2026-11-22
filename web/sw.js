// sw.js is the service worker: it keeps a copy of every file the game
// needs, so the game starts and plays with no network connection, and the
// browser can install it as an app (D22).
//
// Each build gets its own cache, named after the build's version, which
// `make web` writes in place of __VERSION__. When a new build is
// published, the new worker installs alongside the old one and waits; the
// page then offers "Reload" (render.js), and the old cache is deleted once
// the new worker takes over.

"use strict";

const CACHE = "codebreaker-__VERSION__";

// Everything the game needs. Paths are relative to this file, so the game
// works at any address, such as https://<user>.github.io/codebreaker/.
const FILES = [
  "./",
  "index.html",
  "style.css",
  "render.js",
  "wasm_exec.js",
  "codebreaker.wasm",
  "manifest.webmanifest",
  "icons/icon.svg",
  "icons/icon-192.png",
  "icons/icon-512.png",
  "icons/maskable-512.png",
  "icons/apple-touch-icon.png",
];

// Install: download and keep every file. If any download fails, the
// install fails and the old version stays in use.
self.addEventListener("install", (event) => {
  event.waitUntil(caches.open(CACHE).then((cache) => cache.addAll(FILES)));
});

// Activate: delete the caches of older versions, and take over open pages.
self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches.keys()
      .then((names) => Promise.all(
        names.filter((n) => n.startsWith("codebreaker-") && n !== CACHE).map((n) => caches.delete(n))))
      .then(() => self.clients.claim()),
  );
});

// Fetch: answer from the cache first, so the game works offline; anything
// not cached goes to the network as usual.
self.addEventListener("fetch", (event) => {
  if (event.request.method !== "GET") {
    return;
  }
  event.respondWith(
    caches.match(event.request, { ignoreSearch: true })
      .then((cached) => cached || fetch(event.request)),
  );
});

// The page sends "skipWaiting" when the player taps Reload on the update
// banner: the waiting new version then takes over at once.
self.addEventListener("message", (event) => {
  if (event.data === "skipWaiting") {
    self.skipWaiting();
  }
});
