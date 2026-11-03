// Homebase service worker. Served from /sw.js so its scope is the whole app.
//
// Sections:
//   1. Install and activate: precache the offline shell, drop old caches.
//   2. Fetch: show the offline page when a navigation can't reach the network.
//   3. Push and notification clicks: added with Web Push (see #15).
//
// There is no offline data: only the self-contained offline page is cached.

var SHELL_CACHE = "homebase-shell-v1";
var OFFLINE_URL = "/offline.html";

// 1. Install and activate

self.addEventListener("install", function (event) {
	event.waitUntil(
		caches.open(SHELL_CACHE).then(function (cache) {
			return cache.add(new Request(OFFLINE_URL, { cache: "reload" }));
		}).then(function () {
			return self.skipWaiting();
		})
	);
});

self.addEventListener("activate", function (event) {
	event.waitUntil(
		caches.keys().then(function (names) {
			return Promise.all(names.filter(function (name) {
				return name.indexOf("homebase-shell-") === 0 && name !== SHELL_CACHE;
			}).map(function (name) {
				return caches.delete(name);
			}));
		}).then(function () {
			return self.clients.claim();
		})
	);
});

// 2. Fetch

self.addEventListener("fetch", function (event) {
	if (event.request.mode !== "navigate") return;
	event.respondWith(
		fetch(event.request).catch(function () {
			return caches.match(OFFLINE_URL);
		})
	);
});

// 3. Push and notification clicks (#15): add "push" and "notificationclick"
// listeners below this line.
