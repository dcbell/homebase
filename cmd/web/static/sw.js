// Homebase service worker. Served from /sw.js so its scope is the whole app.
//
// Sections:
//   1. Install and activate: precache the offline shell, drop old caches.
//   2. Fetch: show the offline page when a navigation can't reach the network.
//   3. Push and notification clicks: show the Push, set the app-icon badge,
//      and on tap mark it read and open its URL.
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

// 3. Push and notification clicks
//
// The API sends {title, body, url, notification_id, unread_count}. Showing it
// also sets the app-icon badge to the Unread count. Tapping it focuses or
// opens the app at url and marks the Notification read through the web
// service (same origin, so the session cookie goes along), which answers with
// the new Unread count.

function setBadge(count) {
	if (!("setAppBadge" in self.navigator)) return Promise.resolve();
	var badge = count > 0 ? self.navigator.setAppBadge(count) : self.navigator.clearAppBadge();
	return Promise.resolve(badge).catch(function () {});
}

function sameOriginPath(url) {
	if (typeof url !== "string" || url.charAt(0) !== "/" || url.charAt(1) === "/") return "/";
	return url;
}

self.addEventListener("push", function (event) {
	var data = {};
	try {
		data = event.data ? event.data.json() : {};
	} catch (e) {
		data = { body: event.data ? event.data.text() : "" };
	}
	var options = {
		body: data.body || "",
		icon: "/icons/icon-192.png",
		badge: "/icons/badge-96.png",
		data: { url: sameOriginPath(data.url), notification_id: data.notification_id || 0 }
	};
	if (data.notification_id) options.tag = "notification-" + data.notification_id;
	event.waitUntil(Promise.all([
		self.registration.showNotification(data.title || "Homebase", options),
		setBadge(data.unread_count || 0)
	]));
});

function openApp(url) {
	return self.clients.matchAll({ type: "window", includeUncontrolled: true }).then(function (windows) {
		for (var i = 0; i < windows.length; i++) {
			var client = windows[i];
			if (new URL(client.url).origin !== self.location.origin) continue;
			return client.focus().then(function (focused) {
				return focused.navigate ? focused.navigate(url) : focused;
			}).catch(function () {
				return self.clients.openWindow(url);
			});
		}
		return self.clients.openWindow(url);
	});
}

function markRead(id) {
	if (!id) return Promise.resolve();
	return fetch("/notifications/" + encodeURIComponent(id) + "/read", {
		method: "POST",
		credentials: "same-origin",
		headers: { "Accept": "application/json" }
	}).then(function (response) {
		return response.ok ? response.json() : null;
	}).then(function (result) {
		if (result && typeof result.unread_count === "number") return setBadge(result.unread_count);
	}).catch(function () {});
}

self.addEventListener("notificationclick", function (event) {
	var data = event.notification.data || {};
	event.notification.close();
	// Give marking read a moment so the page opens showing it read, but not
	// long: browsers only let a worker open a window shortly after a tap.
	var settle = new Promise(function (resolve) { setTimeout(resolve, 800); });
	event.waitUntil(Promise.race([markRead(data.notification_id), settle]).then(function () {
		return openApp(sameOriginPath(data.url));
	}));
});
