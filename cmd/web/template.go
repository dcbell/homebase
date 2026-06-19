package main

const appTemplate = `
{{ define "layout" }}
<!doctype html>
<html lang="en">
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>{{ .Title }}</title>
	<script>
		(function () {
			var saved = localStorage.getItem("homebase-theme");
			if (saved === "dark" || saved === "light") document.documentElement.dataset.theme = saved;
		})();
	</script>
	<style>
		:root { color-scheme: light; --ink:#2e3338; --muted:#5c6470; --line:#d7dae2; --paper:#f2f3f5; --panel:#ffffff; --panel-soft:#f6f7f9; --field:#ffffff; --accent:#5865f2; --accent-hover:#4752c4; --accent-ink:#ffffff; --danger:#d83c3e; --danger-soft:#fff0f1; --shadow:0 14px 38px rgba(32, 34, 37, .14); }
		:root[data-theme="dark"] { color-scheme: dark; --ink:#f2f3f5; --muted:#b5bac1; --line:#3f4147; --paper:#313338; --panel:#2b2d31; --panel-soft:#383a40; --field:#1e1f22; --accent:#7983f5; --accent-hover:#5865f2; --accent-ink:#ffffff; --danger:#f87171; --danger-soft:#3a2528; --shadow:0 18px 44px rgba(0, 0, 0, .42); }
		* { box-sizing: border-box; }
		[hidden] { display:none !important; }
		body { margin:0; font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; background:var(--paper); color:var(--ink); }
		header { border-bottom:1px solid var(--line); background:var(--panel); position:sticky; top:0; z-index:30; }
		.shell { width:min(1180px, calc(100% - 32px)); margin:0 auto; }
		main.shell { width:min(1180px, calc(100% - 104px)); max-width:none; margin-left:calc(72px + max(16px, (100% - 72px - 1180px) / 2)); margin-right:auto; }
		.topbar { min-height:64px; padding:10px 16px; position:relative; }
		.topbar-brand { position:absolute; left:12px; top:50%; transform:translateY(-50%); display:flex; align-items:center; gap:10px; min-width:0; }
		.topbar-brand-name { color:var(--ink); text-decoration:none; font-weight:800; font-size:18px; line-height:1; white-space:nowrap; }
		.topbar-brand-name:hover { color:var(--ink); }
		.topbar-content { width:min(1180px, calc(100% - 208px)); min-height:44px; margin-left:calc(72px + max(104px, (100% - 72px - 1180px) / 2)); margin-right:auto; max-width:none; display:flex; align-items:center; justify-content:space-between; gap:16px; }
		.topbar-left { display:flex; align-items:center; gap:10px; min-width:0; flex:0 1 auto; }
		.header-page-title { font-weight:800; font-size:20px; line-height:1.15; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
		.menu-button { width:38px; height:38px; min-height:38px; padding:0; }
		.app-sidebar { position:fixed; inset:64px auto 0 0; z-index:19; width:72px; padding:12px; background:var(--panel); border-right:1px solid var(--line); box-shadow:var(--shadow); transition:width .16s ease; overflow:hidden; display:grid; }
		.app-sidebar.open { width:244px; }
		.sidebar-nav { display:grid; gap:8px; align-content:start; overflow:auto; }
		.sidebar-nav a { display:grid; grid-template-columns:38px minmax(0, 1fr) auto; align-items:center; gap:10px; min-height:42px; padding:0 10px 0 0; border:1px solid transparent; border-radius:6px; color:var(--ink); text-decoration:none; background:transparent; }
		.sidebar-nav a:hover { border-color:var(--accent); background:var(--panel-soft); color:var(--ink); }
		.sidebar-icon { width:38px; height:38px; display:grid; place-items:center; color:var(--accent); }
		.sidebar-nav .icon { width:19px; height:19px; }
		.sidebar-label, .sidebar-chevron { white-space:nowrap; overflow:hidden; opacity:0; transform:translateX(-6px); transition:opacity .12s ease, transform .12s ease; }
		.app-sidebar.open .sidebar-label, .app-sidebar.open .sidebar-chevron { opacity:1; transform:translateX(0); }
		.app-sidebar:not(.open) .sidebar-nav { overflow:hidden auto; }
		.app-sidebar:not(.open) .sidebar-nav a { grid-template-columns:38px; width:42px; padding:0; justify-self:center; justify-content:center; }
		.app-sidebar:not(.open) .sidebar-label, .app-sidebar:not(.open) .sidebar-chevron { display:none; }
		.sidebar-close { display:none; }
		.sidebar-backdrop { display:none; }
		.topbar-right { display:flex; gap:8px; flex-wrap:wrap; align-items:center; justify-content:flex-end; margin-left:auto; min-width:0; }
		.universal-search { position:relative; width:min(360px, 34vw); min-width:220px; }
		.universal-search input { min-height:36px; padding-left:34px; padding-right:34px; }
		.search-leading-icon { position:absolute; left:10px; top:50%; transform:translateY(-50%); color:var(--muted); pointer-events:none; }
		.search-leading-icon .icon { width:16px; height:16px; }
		.universal-results { position:absolute; right:0; top:calc(100% + 6px); z-index:32; width:min(440px, calc(100vw - 32px)); display:none; gap:6px; padding:8px; border:1px solid var(--line); border-radius:8px; background:var(--panel); box-shadow:var(--shadow); }
		.universal-results.open { display:grid; }
		.universal-result { display:grid; gap:2px; padding:9px 10px; border:1px solid transparent; border-radius:6px; text-decoration:none; color:var(--ink); background:var(--panel-soft); }
		.universal-result:hover { border-color:var(--accent); color:var(--ink); }
		.universal-result strong { overflow-wrap:anywhere; }
		.result-type { color:var(--accent); font-size:11px; font-weight:800; text-transform:uppercase; letter-spacing:0; }
		nav { display:flex; gap:8px; flex-wrap:wrap; align-items:center; }
		a, button { color:var(--accent); }
		button, .button { border:1px solid var(--accent); background:var(--accent); color:var(--accent-ink); min-height:36px; padding:0 12px; border-radius:6px; font:inherit; cursor:pointer; text-decoration:none; display:inline-flex; align-items:center; justify-content:center; white-space:nowrap; }
		.link-button { min-height:0; padding:0; border:0; background:transparent; color:var(--accent); display:inline; text-align:left; text-decoration:underline; white-space:normal; }
		.link-button:hover { border:0; background:transparent; color:var(--accent-hover); }
		button:hover, .button:hover { border-color:var(--accent-hover); background:var(--accent-hover); color:var(--accent-ink); }
		button.secondary, .button.secondary { background:var(--panel); color:var(--accent); }
		button.secondary:hover, .button.secondary:hover { background:var(--panel-soft); color:var(--accent); }
		button.danger, .button.danger { border-color:var(--danger); color:var(--danger); background:var(--panel); }
		button.danger:hover, .button.danger:hover { background:var(--danger-soft); color:var(--danger); border-color:var(--danger); }
		button.compact { min-height:28px; padding:0 8px; font-size:12px; }
		.drag-handle { min-width:28px; cursor:grab; font-size:15px; line-height:1; }
		.drag-handle:active { cursor:grabbing; }
		main { padding:24px 0 48px; }
		.hero { display:flex; justify-content:space-between; align-items:flex-end; gap:24px; margin-bottom:24px; }
		.detail-hero { display:flex; justify-content:space-between; align-items:center; gap:18px; margin-bottom:24px; }
		.title-row { display:flex; align-items:center; gap:14px; min-width:0; }
		.title-row h1 { overflow-wrap:anywhere; }
		.title-copy { min-width:0; display:grid; gap:4px; }
		.title-copy p { margin:0; }
		.back-link { gap:6px; flex:0 0 auto; }
		.back-arrow { font-size:20px; line-height:1; margin-top:-1px; }
		.detail-actions { display:flex; gap:8px; flex-wrap:wrap; justify-content:flex-end; align-items:center; }
		.dashboard-actions { display:flex; align-items:center; gap:10px; flex-wrap:wrap; justify-content:flex-end; }
		.dashboard-toolbar { display:flex; justify-content:flex-end; align-items:center; gap:10px; margin-bottom:16px; }
		.dashboard-edit-toggle.active { background:var(--accent); color:var(--accent-ink); }
		h1 { margin:0; font-size:32px; line-height:1.1; }
		h2 { margin:0; font-size:18px; }
		h3 { margin:0 0 8px; font-size:15px; }
		p { color:var(--muted); }
		.dashboard-tiles { display:grid; grid-template-columns:repeat(2, minmax(0, 1fr)); gap:20px; align-items:start; }
		.dashboard-masonry { display:block; position:relative; }
		.full-width { grid-column:1 / -1; }
		.dashboard-masonry .dashboard-tile { position:absolute; left:0; top:0; transition:transform .12s ease, opacity .12s ease; }
		.dashboard-masonry:not(.dashboard-ready) .dashboard-tile { transition:none; }
		.dashboard-tile.dragging { opacity:.55; z-index:2; }
		.dashboard-edit-control { display:none; }
		.dashboard-shell.editing .dashboard-edit-control { display:inline-flex; }
		.dashboard-shell.editing .dashboard-add-tile { display:flex; }
		.dashboard-add-tile { display:none; min-height:150px; border-style:dashed; align-items:center; justify-content:center; }
		.dashboard-masonry .dashboard-add-tile { transition:opacity .12s ease; }
		.dashboard-add-menu { display:grid; gap:10px; justify-items:center; text-align:center; }
		.dashboard-add-plus { width:54px; height:54px; border-radius:999px; font-size:32px; line-height:1; }
		.dashboard-add-options { display:none; gap:8px; flex-wrap:wrap; justify-content:center; }
		.dashboard-add-tile.open .dashboard-add-options { display:flex; }
		.dashboard-add-tile.open .dashboard-add-plus { display:none; }
		.dashboard-add-tile.empty { display:none !important; }
		.dashboard-add-options [hidden] { display:none !important; }
		.panel { background:var(--panel); border:1px solid var(--line); border-radius:8px; padding:16px; min-width:0; }
		.code-block { margin:12px 0 0; padding:12px; border:1px solid var(--line); border-radius:6px; background:var(--field); color:var(--ink); white-space:pre-wrap; overflow-wrap:anywhere; font:13px/1.5 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; }
		.tile-head { display:flex; align-items:center; justify-content:space-between; gap:12px; margin-bottom:12px; }
		.tile-actions { display:flex; gap:6px; flex-wrap:wrap; justify-content:flex-end; }
		.tile-actions form { display:block; }
		.cards { display:grid; gap:10px; }
		.maintenance-list { display:grid; gap:10px; }
		.maintenance-meta { display:flex; gap:8px; flex-wrap:wrap; margin-top:6px; }
		.item { border:1px solid var(--line); border-radius:6px; padding:12px; background:var(--panel-soft); min-width:0; }
		.item-head { display:flex; justify-content:space-between; gap:12px; align-items:flex-start; }
		.summary-strip { display:flex; gap:8px; flex-wrap:wrap; margin:0 0 12px; }
		.info-strip { display:grid; grid-template-columns:repeat(3, minmax(0, 1fr)); gap:10px; margin:12px 0 0; }
		.asset-summary { grid-template-columns:repeat(auto-fit, minmax(170px, 1fr)); margin:0 0 20px; }
		.info-cell { border:1px solid var(--line); border-radius:6px; padding:10px; background:var(--panel-soft); min-width:0; }
		.info-cell span { display:block; color:var(--muted); font-size:12px; margin-bottom:4px; }
		.project-title-line { display:flex; align-items:center; gap:10px; flex-wrap:wrap; }
		.page-title-line { display:flex; align-items:center; gap:10px; flex-wrap:wrap; }
		.back-icon { width:32px; height:32px; min-height:32px; padding:0; border-radius:6px; font-size:22px; line-height:1; }
		.title-icon-button { width:32px; height:32px; min-height:32px; padding:0; border-radius:6px; }
		.status-inline { display:inline-flex; align-items:center; }
		.status-inline select { min-height:30px; padding:4px 28px 4px 9px; }
		.action-menu { position:relative; }
		.action-menu summary { list-style:none; cursor:pointer; }
		.action-menu summary::-webkit-details-marker { display:none; }
		.action-menu-panel { position:absolute; right:0; top:calc(100% + 6px); z-index:8; min-width:150px; display:grid; gap:6px; padding:8px; border:1px solid var(--line); border-radius:6px; background:var(--panel); box-shadow:var(--shadow); }
		.action-menu.left .action-menu-panel { left:0; right:auto; }
		.action-menu.drop-up .action-menu-panel { top:auto; bottom:calc(100% + 6px); }
		.action-menu-panel > button, .action-menu-panel > form > button { width:100%; justify-content:flex-start; }
		.pill { display:inline-flex; align-items:center; justify-content:center; min-height:28px; padding:0 10px; border:1px solid var(--line); border-radius:999px; background:var(--panel); color:var(--muted); font-size:12px; font-weight:800; white-space:nowrap; }
		.pill.open, .pill.active { border-color:#23a55a; color:#23a55a; }
		.pill.waiting, .pill.normal { border-color:#f0b232; color:#f0b232; }
		.pill.done, .pill.low { border-color:#80848e; color:#80848e; }
		.pill.high { border-color:var(--danger); color:var(--danger); }
		.task-table-wrap { overflow:visible; }
		.task-table { width:100%; border-collapse:separate; border-spacing:0; min-width:760px; }
		.task-table th, .task-table td { border-bottom:1px solid var(--line); padding:8px; text-align:left; vertical-align:middle; }
		.task-table th { color:var(--muted); font-size:12px; font-weight:800; }
		.task-table td input, .task-table td select { min-height:32px; padding:5px 8px; }
		.task-table td.due-cell { width:120px; }
		.task-table .task-title-cell { min-width:220px; }
		.task-action-cell { width:44px; }
		.folder-row td { background:var(--panel-soft); font-weight:800; }
		.folder-row form { display:flex; gap:8px; align-items:center; }
		.folder-row.drop-target td { box-shadow:inset 0 0 0 2px var(--accent); }
		.folder-toggle { cursor:pointer; }
		.folder-summary { display:flex; align-items:center; gap:8px; flex-wrap:wrap; }
		.folder-title { font-weight:800; }
		.due-picker { position:relative; display:inline-grid; }
		.due-picker input { position:absolute; inset:0; opacity:0; width:100%; height:100%; pointer-events:none; }
		.task-row { cursor:grab; }
		.task-row.dragging { opacity:.45; }
		.task-row.done { opacity:.68; }
		.checklist { display:grid; gap:8px; }
		.check-item { display:grid; grid-template-columns:34px minmax(0, 1fr) auto; gap:10px; align-items:start; border:1px solid var(--line); border-radius:6px; padding:10px; background:var(--panel-soft); }
		.check-item.done { opacity:.68; }
		.check-item.done .check-title { text-decoration:line-through; }
		.check-toggle { width:26px; min-width:26px; height:26px; min-height:26px; padding:0; border-radius:6px; font-weight:900; }
		.check-main { display:grid; gap:3px; min-width:0; }
		.check-title { font-weight:800; }
		.check-meta { display:flex; gap:6px; flex-wrap:wrap; align-items:center; }
		.task-index-card { display:grid; gap:10px; }
		.task-index-card .item-head { align-items:flex-start; }
		.task-card-controls { display:flex; gap:8px; flex-wrap:wrap; align-items:center; }
		.related-list { display:grid; gap:10px; }
		.related-row { display:grid; grid-template-columns:minmax(0, 1fr) auto; align-items:start; gap:12px; border:1px solid var(--line); border-radius:6px; padding:12px; background:var(--panel-soft); }
		.related-row form { align-self:start; }
		.related-row .remove-button { width:28px; min-width:28px; height:28px; min-height:28px; padding:0; }
		.related-inline { display:flex; gap:6px; flex-wrap:wrap; margin-top:5px; }
		.related-inline a, .related-inline button.link-button { display:inline-flex; align-items:center; gap:4px; min-height:24px; padding:0 8px; border:1px solid var(--line); border-radius:999px; text-decoration:none; font-size:12px; background:var(--panel); }
		.related-inline button.link-button:hover { border-color:var(--accent); background:var(--panel-soft); }
		.related-inline .icon { width:13px; height:13px; }
		.paperclip-button { min-width:30px; width:30px; padding:0; }
		.modal-columns { display:grid; grid-template-columns:repeat(2, minmax(0, 1fr)); gap:16px; align-items:start; }
		.modal-card.wide { width:min(780px, calc(100vw - 40px)); }
		.search-panel { display:grid; gap:10px; }
		.search-list { display:grid; gap:8px; max-height:340px; overflow:auto; padding-right:6px; scrollbar-gutter:stable; }
		.search-choice { width:100%; min-height:54px; justify-content:flex-start; align-content:start; text-align:left; white-space:normal; padding:10px; display:grid; gap:3px; line-height:1.2; }
		.search-choice strong, .search-choice span { display:block; overflow-wrap:anywhere; }
		.search-choice .meta { font-size:12px; line-height:1.25; }
		.meta { color:var(--muted); font-size:13px; overflow-wrap:anywhere; }
		.badge { display:inline-flex; align-items:center; min-height:24px; padding:0 8px; border:1px solid var(--line); border-radius:999px; color:var(--muted); font-size:12px; white-space:nowrap; }
		.badge.high { border-color:var(--danger); color:var(--danger); }
		form { display:grid; gap:10px; min-width:0; }
		label { display:grid; gap:5px; color:var(--muted); font-size:13px; min-width:0; }
		input, textarea, select { width:100%; min-width:0; min-height:38px; border:1px solid var(--line); border-radius:6px; padding:8px 10px; font:inherit; background:var(--field); color:var(--ink); }
		textarea { min-height:74px; resize:vertical; }
		.form-row { display:grid; grid-template-columns:repeat(2, minmax(0, 1fr)); gap:10px; min-width:0; }
		.filter-row { margin-bottom:14px; grid-template-columns:minmax(0, 1fr) minmax(160px, 220px) minmax(160px, 220px); align-items:end; }
		.modules { display:grid; grid-template-columns:repeat(5, minmax(0, 1fr)); gap:10px; }
		.module { border:1px solid var(--line); border-radius:6px; padding:12px; background:var(--panel-soft); min-width:0; color:var(--ink); text-decoration:none; }
		a.module:hover { border-color:var(--accent); }
		.stat-grid { display:grid; grid-template-columns:repeat(3, minmax(0, 1fr)); gap:10px; }
		.stat-link { display:grid; gap:4px; min-height:86px; align-content:center; border:1px solid var(--line); border-radius:6px; padding:12px; text-decoration:none; background:var(--panel-soft); color:var(--ink); }
		.stat-link:hover { border-color:var(--accent); color:var(--ink); }
		.stat-link strong { font-size:28px; line-height:1; }
		.dashboard-list-select { min-width:190px; width:100%; margin-bottom:12px; }
		.detail-with-sidebar { display:grid; grid-template-columns:minmax(0, 1fr) minmax(280px, 340px); gap:20px; align-items:start; }
		.detail-main, .detail-sidebar { display:grid; gap:16px; min-width:0; }
		.info-panel-section + .info-panel-section { border-top:1px solid var(--line); padding-top:14px; margin-top:14px; }
		.empty { color:var(--muted); margin:0; }
		.login { min-height:calc(100vh - 64px); display:grid; place-items:center; }
		.login-box { width:min(440px, 100%); background:var(--panel); border:1px solid var(--line); border-radius:8px; padding:24px; }
		.error { border-color:var(--danger); color:var(--danger); background:var(--danger-soft); }
		.inline-form { display:inline-grid; }
		.subtasks { margin-top:10px; padding-top:10px; border-top:1px solid var(--line); display:grid; gap:8px; }
		.subtask { display:flex; justify-content:space-between; gap:10px; align-items:flex-start; }
		.calendar-toolbar { display:flex; justify-content:space-between; align-items:center; gap:10px; flex-wrap:wrap; margin-bottom:12px; }
		.month-switch { display:flex; align-items:center; gap:6px; flex-wrap:wrap; }
		.month-title { min-width:132px; text-align:center; font-weight:800; font-size:18px; }
		.hero .month-title, .detail-hero .month-title { font-size:32px; line-height:1.1; }
		.view-tabs { display:flex; align-items:center; gap:6px; flex-wrap:wrap; }
		.view-tabs .active { background:var(--accent); color:var(--accent-ink); }
		.calendar-grid { display:grid; grid-template-columns:repeat(7, minmax(0, 1fr)); border:1px solid var(--line); border-radius:8px; overflow:hidden; background:var(--panel); }
		.weekday { min-height:34px; padding:8px; background:var(--panel); color:var(--muted); font-size:12px; font-weight:700; text-align:center; }
		.weekday, .calendar-day { border-right:1px solid var(--line); border-bottom:1px solid var(--line); }
		.weekday:nth-child(7n), .calendar-day:nth-child(7n) { border-right:0; }
		.calendar-day:nth-last-child(-n+7) { border-bottom:0; }
		.calendar-day { min-height:138px; padding:8px; background:var(--panel); display:grid; align-content:start; gap:6px; min-width:0; }
		.calendar-day.outside { background:var(--panel-soft); }
		.calendar-day.today { box-shadow:inset 0 0 0 2px var(--accent); }
		.day-head { display:flex; align-items:center; justify-content:space-between; gap:8px; }
		.day-badges { display:flex; align-items:center; gap:4px; flex-wrap:wrap; justify-content:flex-end; }
		.day-number { color:var(--muted); font-weight:700; font-size:12px; }
		.calendar-entries { display:grid; gap:5px; min-width:0; }
		.calendar-entry { display:grid; gap:2px; min-width:0; border-left:3px solid var(--line); border-radius:4px; padding:5px 6px; background:var(--panel-soft); color:var(--ink); text-decoration:none; }
		.calendar-entry strong { font-size:12px; line-height:1.2; overflow-wrap:anywhere; }
		.calendar-entry.appointment { border-left-color:var(--accent); }
		.calendar-entry.task { border-left-color:#23a55a; }
		.calendar-entry.task.high { border-left-color:var(--danger); }
		.calendar-entry.project { border-left-color:#f0b232; }
		.calendar-entry.routine { border-left-color:#a970ff; }
		.calendar-entry.done { opacity:.68; }
		.calendar-week-grid { display:grid; grid-template-columns:repeat(7, minmax(0, 1fr)); border:1px solid var(--line); border-radius:8px; overflow:hidden; background:var(--line); gap:1px; }
		.calendar-week-grid .calendar-day { min-height:180px; }
		.calendar-list { display:grid; gap:10px; }
		.calendar-list-day { display:grid; grid-template-columns:120px minmax(0, 1fr); gap:12px; align-items:start; }
		.calendar-list-date { color:var(--muted); font-weight:800; font-size:13px; padding-top:8px; }
		.modal { position:fixed; inset:0; z-index:60; display:none; place-items:center; padding:20px; background:rgba(0,0,0,.48); }
		.modal.open { display:grid; }
		.modal-card { width:min(620px, 100%); max-height:calc(100vh - 40px); overflow:auto; background:var(--panel); border:1px solid var(--line); border-radius:8px; box-shadow:var(--shadow); padding:16px; }
		.modal-card.preview { width:min(1200px, calc(100vw - 40px)); height:calc(100vh - 40px); display:grid; grid-template-rows:auto auto minmax(0, 1fr) auto; gap:10px; overflow:hidden; }
		.modal-head { display:flex; align-items:center; justify-content:space-between; gap:12px; margin-bottom:12px; }
		.modal-card.preview .modal-head { margin-bottom:0; }
		.modal-head h2 { margin:0; }
		.document-preview-meta { margin:0; }
		.document-preview { width:100%; height:100%; min-height:0; border:1px solid var(--line); border-radius:6px; background:var(--field); }
		.document-detail-preview { height:min(72vh, 760px); min-height:520px; }
		.document-preview-empty { min-height:180px; display:grid; place-items:center; border:1px dashed var(--line); border-radius:6px; color:var(--muted); background:var(--panel-soft); }
		.floating-menu { position:relative; }
		.floating-menu summary { list-style:none; cursor:pointer; display:flex; align-items:center; gap:8px; }
		.floating-menu summary::-webkit-details-marker { display:none; }
		.avatar { width:38px; height:38px; border-radius:999px; border:1px solid var(--line); background:var(--accent); color:var(--accent-ink); display:grid; place-items:center; font-weight:800; overflow:hidden; }
		.avatar img { width:100%; height:100%; object-fit:cover; }
		.icon-button { width:38px; height:38px; padding:0; border-radius:999px; background:var(--panel); color:var(--accent); position:relative; display:grid; place-items:center; }
		.icon { width:19px; height:19px; stroke:currentColor; stroke-width:2; fill:none; stroke-linecap:round; stroke-linejoin:round; }
		.drag-handle .icon { width:16px; height:16px; }
		.drag-handle .icon circle { fill:currentColor; stroke:0; }
		.notice-dot { position:absolute; top:2px; right:1px; min-width:18px; height:18px; padding:0 5px; border-radius:999px; background:var(--danger); color:#fff; font-size:11px; display:grid; place-items:center; }
		.profile-panel { position:absolute; top:calc(100% + 8px); right:0; z-index:10; width:min(320px, calc(100vw - 32px)); background:var(--panel); border:1px solid var(--line); border-radius:8px; box-shadow:var(--shadow); padding:12px; display:grid; gap:12px; }
		.profile-panel form { display:block; }
		.notice-list { display:grid; gap:8px; max-height:340px; overflow:auto; }
		.theme-row { display:flex; align-items:center; justify-content:space-between; gap:12px; color:var(--muted); font-size:13px; }
		.switch { position:relative; width:48px; height:26px; display:inline-block; }
		.switch input { position:absolute; opacity:0; width:1px; height:1px; }
		.switch span { position:absolute; inset:0; border-radius:999px; background:var(--line); cursor:pointer; }
		.switch span::before { content:""; position:absolute; width:20px; height:20px; left:3px; top:3px; border-radius:50%; background:var(--panel); transition:transform .16s ease; }
		.switch input:checked + span { background:var(--accent); }
		.switch input:checked + span::before { transform:translateX(22px); }
		@media (max-width: 1100px) {
			.topbar-right > .meta { display:none; }
			.detail-with-sidebar { grid-template-columns:1fr; }
			.task-table-wrap { overflow:visible; }
			.task-table, .task-table tbody { display:block; min-width:0; }
			.task-table thead { display:none; }
			.task-table tbody { display:grid; gap:10px; }
			.task-table tr { width:100%; }
			.folder-row { display:block; }
			.folder-row td { display:block; width:100%; border:0; border-radius:6px; padding:10px; }
			.folder-summary { min-width:0; }
			.task-row { position:relative; display:grid; grid-template-columns:repeat(2, minmax(0, 1fr)); gap:8px 10px; padding:12px; border:1px solid var(--line); border-radius:6px; background:var(--panel-soft); cursor:default; }
			.task-table .task-row td { display:block; width:auto; min-width:0; padding:0; border:0; }
			.task-table .task-row .task-title-cell { grid-column:1 / -1; min-width:0; padding-right:38px; font-weight:800; }
			.task-table .task-row .task-meta-cell { display:flex; align-items:center; min-height:30px; }
			.task-table .task-row .task-status-cell { order:1; }
			.task-table .task-row .task-priority-cell { order:2; }
			.task-table .task-row .task-assignee-cell { order:3; }
			.task-table .task-row .task-due-cell { order:4; }
			.task-table .task-row .task-action-cell { position:absolute; top:10px; right:10px; width:auto; }
			.task-more-menu .action-menu-panel { left:auto; right:0; min-width:180px; }
			.detail-sidebar { order:2; }
		}
		@media (max-width: 860px) {
			.app-sidebar { width:60px; padding:10px; }
			.app-sidebar.open { width:224px; }
			.app-sidebar:not(.open) .sidebar-nav a { width:38px; }
			main.shell { margin-left:76px; margin-right:12px; width:auto; }
			.topbar { display:grid; gap:10px; }
			.topbar-brand { position:static; transform:none; }
			.topbar-content { width:100%; margin-left:0; margin-right:0; max-width:none; min-height:0; flex-wrap:wrap; }
			.dashboard-tiles { grid-template-columns:1fr; }
			.modules, .form-row { grid-template-columns:1fr; }
			.info-strip { grid-template-columns:1fr; }
			.modal-columns { grid-template-columns:1fr; }
			.calendar-grid { display:grid; grid-template-columns:1fr; background:transparent; border:0; gap:10px; overflow:visible; }
			.calendar-week-grid { display:grid; grid-template-columns:1fr; background:transparent; border:0; gap:10px; overflow:visible; }
			.weekday { display:none; }
			.calendar-day { min-height:0; border:1px solid var(--line); border-radius:8px; }
			.calendar-week-grid .calendar-day { min-height:0; }
			.calendar-day.outside.empty-day { display:none; }
			.calendar-list-day { grid-template-columns:1fr; }
			.calendar-toolbar { align-items:flex-start; }
			.month-switch { width:100%; justify-content:flex-start; }
			.month-title { min-width:0; }
			.hero, .detail-hero { align-items:flex-start; flex-direction:column; }
			.topbar-left { width:100%; }
			.topbar-right { width:100%; justify-content:flex-start; }
			.universal-search { width:100%; min-width:0; }
			.universal-results { left:0; right:auto; width:100%; }
			.detail-actions { justify-content:flex-start; }
			.dashboard-masonry { display:grid; position:static; }
			.dashboard-masonry .dashboard-tile { position:static; width:auto !important; transform:none !important; }
		}
		@media (max-width: 700px) {
			header { z-index:30; }
			.app-sidebar { inset:0 auto 0 0; z-index:40; width:min(280px, 86vw); padding:74px 12px 16px; transform:translateX(-100%); visibility:hidden; transition:transform .16s ease, visibility .16s ease; }
			.app-sidebar.open { width:min(280px, 86vw); transform:translateX(0); visibility:visible; }
			.sidebar-close { position:absolute; top:18px; left:14px; display:inline-flex; width:38px; height:38px; min-height:38px; padding:0; }
			.sidebar-backdrop { position:fixed; inset:0; z-index:39; width:100%; height:100%; min-height:0; padding:0; border:0; border-radius:0; background:rgba(0,0,0,.46); }
			.app-sidebar.open + .sidebar-backdrop { display:block; }
			.app-sidebar:not(.open) .sidebar-nav a, .app-sidebar.open .sidebar-nav a { width:100%; grid-template-columns:38px minmax(0, 1fr) auto; justify-self:stretch; justify-content:initial; padding-right:10px; }
			.app-sidebar.open .sidebar-label, .app-sidebar.open .sidebar-chevron { display:block; opacity:1; transform:none; }
			main.shell { width:auto; margin-left:12px; margin-right:12px; padding-top:16px; }
			.topbar { padding:10px 12px; }
			.topbar-content { gap:10px; }
			.topbar-right { gap:8px; }
			.topbar-right > .meta { display:block; width:100%; }
			h1 { font-size:26px; }
			.panel { padding:12px; }
			.project-title-line, .page-title-line { gap:8px; }
			.summary-strip { gap:6px; }
			.task-row { display:flex; flex-wrap:wrap; gap:8px; }
			.task-table .task-row .task-title-cell { flex:0 0 100%; }
			.task-table .task-row .task-meta-cell { flex:0 0 auto; width:auto; }
			.task-card-controls { display:flex; flex-wrap:wrap; gap:6px; }
			.task-card-controls > * { flex:0 0 auto; min-width:0; }
			.task-table .task-row .task-meta-cell { min-height:28px; }
			.folder-summary .folder-status-badge, .folder-summary .folder-done-badge { display:none; }
			.item-head { align-items:flex-start; }
			.check-item { grid-template-columns:34px minmax(0, 1fr); }
			.check-item > .tile-actions { grid-column:2; justify-content:flex-start; }
			.related-row { grid-template-columns:minmax(0, 1fr) auto; }
			.modal { padding:10px; }
			.modal-card { max-height:calc(100vh - 20px); }
			.modal-card.preview { width:calc(100vw - 20px); height:calc(100vh - 20px); }
			.document-detail-preview { min-height:360px; height:62vh; }
		}
	</style>
</head>
<body>
	{{ if .Dashboard.CurrentUser.Email }}
	<aside class="app-sidebar" aria-label="Main menu">
		<button class="secondary sidebar-close" type="button" data-menu-close title="Close menu" aria-label="Close menu"><svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M4 6h16"></path><path d="M4 12h16"></path><path d="M4 18h16"></path></svg></button>
		<nav class="sidebar-nav">
			<a href="/" title="Dashboard"><span class="sidebar-icon"><svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M3 11l9-8 9 8"></path><path d="M5 10v10h14V10"></path><path d="M9 20v-6h6v6"></path></svg></span><span class="sidebar-label">Dashboard</span><span class="sidebar-chevron">›</span></a>
			<a href="/tasks" title="Tasks"><span class="sidebar-icon"><svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M9 11l3 3L22 4"></path><path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11"></path></svg></span><span class="sidebar-label">Tasks</span><span class="sidebar-chevron">›</span></a>
			<a href="/projects" title="Projects"><span class="sidebar-icon"><svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M3 7h5l2 3h11v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"></path><path d="M3 7V5a2 2 0 0 1 2-2h4l2 4"></path></svg></span><span class="sidebar-label">Projects</span><span class="sidebar-chevron">›</span></a>
			<a href="/calendar" title="Calendar"><span class="sidebar-icon"><svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M8 2v4"></path><path d="M16 2v4"></path><rect x="3" y="4" width="18" height="18" rx="2"></rect><path d="M3 10h18"></path></svg></span><span class="sidebar-label">Calendar</span><span class="sidebar-chevron">›</span></a>
			<a href="/routines" title="Routines"><span class="sidebar-icon"><svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.7-2.7L3 16"></path><path d="M3 21v-5h5"></path><path d="M3 12a9 9 0 0 1 15.7-6.3L21 8"></path><path d="M16 8h5V3"></path></svg></span><span class="sidebar-label">Routines</span><span class="sidebar-chevron">›</span></a>
			<a href="/lists" title="Lists"><span class="sidebar-icon"><svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M8 6h13"></path><path d="M8 12h13"></path><path d="M8 18h13"></path><path d="M3 6h.01"></path><path d="M3 12h.01"></path><path d="M3 18h.01"></path></svg></span><span class="sidebar-label">Lists</span><span class="sidebar-chevron">›</span></a>
			<a href="/contacts" title="Contacts"><span class="sidebar-icon"><svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M16 21v-2a4 4 0 0 0-8 0v2"></path><circle cx="12" cy="7" r="4"></circle><path d="M22 21v-2a4 4 0 0 0-3-3.87"></path><path d="M16 3.13a4 4 0 0 1 0 7.75"></path></svg></span><span class="sidebar-label">Contacts</span><span class="sidebar-chevron">›</span></a>
			<a href="/assets" title="Assets"><span class="sidebar-icon"><svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.5-3.5a6 6 0 0 1-7.9 7.9l-6.1 6.1a2.1 2.1 0 0 1-3-3l6.1-6.1a6 6 0 0 1 7.9-7.9z"></path></svg></span><span class="sidebar-label">Assets</span><span class="sidebar-chevron">›</span></a>
			<a href="/documents" title="Documents"><span class="sidebar-icon"><svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path><path d="M14 2v6h6"></path><path d="M16 13H8"></path><path d="M16 17H8"></path></svg></span><span class="sidebar-label">Documents</span><span class="sidebar-chevron">›</span></a>
			<a href="/members" title="Users"><span class="sidebar-icon"><svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="8" r="5"></circle><path d="M20 21a8 8 0 0 0-16 0"></path></svg></span><span class="sidebar-label">Users</span><span class="sidebar-chevron">›</span></a>
			{{ if .Dashboard.BudgetAppURL }}<a href="{{ .Dashboard.BudgetAppURL }}" title="Budget"><span class="sidebar-icon"><svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><rect x="2" y="5" width="20" height="14" rx="2"></rect><path d="M2 10h20"></path></svg></span><span class="sidebar-label">Budget</span><span class="sidebar-chevron">›</span></a>{{ end }}
		</nav>
	</aside>
	<button class="sidebar-backdrop" type="button" data-menu-close aria-label="Close menu"></button>
	{{ end }}
	<header>
		<div class="topbar">
			{{ if .Dashboard.CurrentUser.Email }}
			<div class="topbar-brand">
				<button class="secondary menu-button" type="button" data-menu-toggle title="Toggle menu" aria-label="Toggle menu" aria-expanded="false"><svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M4 6h16"></path><path d="M4 12h16"></path><path d="M4 18h16"></path></svg></button>
				<a class="topbar-brand-name" href="/">Homebase</a>
			</div>
			{{ end }}
			<div class="topbar-content">
				<div class="topbar-left">
					<span class="header-page-title">{{ headerTitle . }}</span>
				</div>
				<nav class="topbar-right">
				{{ if .Dashboard.CurrentUser.Email }}
					<div class="universal-search" role="search">
						<span class="search-leading-icon"><svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><circle cx="11" cy="11" r="8"></circle><path d="m21 21-4.3-4.3"></path></svg></span>
						<input type="search" autocomplete="off" placeholder="Search tasks, projects, docs..." aria-label="Search" data-universal-search>
						<div class="universal-results" data-universal-search-results></div>
					</div>
					<span class="meta">{{ .Dashboard.CurrentUser.Name }} · {{ .Dashboard.Household.Name }}</span>
					{{ if .Dashboard.BudgetAppURL }}<a class="button secondary" href="{{ .Dashboard.BudgetAppURL }}">Budget</a>{{ end }}
					<details class="floating-menu">
						<summary aria-label="Notifications">
							<span class="icon-button"><svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9"></path><path d="M10 21h4"></path></svg>{{ if .Dashboard.Notices }}<span class="notice-dot">{{ len .Dashboard.Notices }}</span>{{ end }}</span>
						</summary>
						<div class="profile-panel">
							<strong>Notifications</strong>
							<div class="notice-list">
								{{ range .Dashboard.Notices }}
									<a class="item" href="/routines/{{ .Routine.ID }}">
										<strong>{{ .Message }}</strong>
										{{ if .Routine.NextDueAt }}<div class="meta">Due {{ date .Routine.NextDueAt }} · {{ .Kind }}</div>{{ end }}
									</a>
								{{ else }}
									<p class="empty">No routine notifications.</p>
								{{ end }}
							</div>
						</div>
					</details>
					<details class="floating-menu">
						<summary aria-label="Profile menu">
							<span class="avatar">
								{{ if .Dashboard.CurrentUser.AvatarURL }}<img src="{{ .Dashboard.CurrentUser.AvatarURL }}" alt="">{{ else }}{{ initials .Dashboard.CurrentUser.Name .Dashboard.CurrentUser.Email }}{{ end }}
							</span>
						</summary>
						<div class="profile-panel">
							<div>
								<strong>{{ .Dashboard.CurrentUser.Name }}</strong>
								<div class="meta">{{ .Dashboard.CurrentUser.Email }}</div>
							</div>
							<label class="theme-row">
								<span>Dark mode</span>
								<span class="switch"><input id="theme-toggle" type="checkbox"><span></span></span>
							</label>
							<a class="button secondary" href="/settings">Settings</a>
							<form method="post" action="/logout"><button class="secondary" type="submit">Logout</button></form>
						</div>
					</details>
				{{ else }}
					<a class="button" href="{{ .LoginURL }}">Login</a>
				{{ end }}
				</nav>
			</div>
		</div>
	</header>
	{{ if .DashboardPage }}
		{{ template "dashboard" . }}
	{{ else if .ProjectIndex }}
		{{ template "projectIndex" . }}
	{{ else if .TaskIndex }}
		{{ template "taskIndex" . }}
	{{ else if .RoutineIndex }}
		{{ template "routineIndex" . }}
	{{ else if .MemberIndex }}
		{{ template "memberIndex" . }}
	{{ else if .SettingsPage }}
		{{ template "settings" . }}
	{{ else if .Project.ID }}
		{{ template "projectDetail" . }}
	{{ else if .Task.ID }}
		{{ template "taskDetail" . }}
	{{ else if .Document.ID }}
		{{ template "documentDetail" . }}
	{{ else if .Asset.ID }}
		{{ template "assetDetail" . }}
	{{ else if .Event.ID }}
		{{ template "eventDetail" . }}
	{{ else if .Routine.ID }}
		{{ template "routineDetail" . }}
	{{ else if .Contact.ID }}
		{{ template "contactDetail" . }}
	{{ else if .List.ID }}
		{{ template "listDetail" . }}
	{{ else if .ListIndex }}
		{{ template "listIndex" . }}
	{{ else if .ContactIndex }}
		{{ template "contactIndex" . }}
	{{ else if .AssetIndex }}
		{{ template "assetIndex" . }}
	{{ else if .DocumentIndex }}
		{{ template "documentIndex" . }}
	{{ else if .CalendarPage }}
		{{ template "calendar" . }}
	{{ else if .Dashboard.CurrentUser.Email }}
		{{ template "dashboard" . }}
	{{ else }}
		{{ template "login" . }}
	{{ end }}
	<script>
		(function () {
			function openModal(id) {
				var modal = document.getElementById(id);
				if (modal) modal.classList.add("open");
			}
			function closeModal(modal) {
				if (modal) modal.classList.remove("open");
			}
			document.querySelectorAll("[data-modal-open]").forEach(function (button) {
				button.addEventListener("click", function () {
					openModal(button.getAttribute("data-modal-open"));
				});
			});
			document.querySelectorAll("[data-date-open]").forEach(function (button) {
				button.addEventListener("click", function () {
					var input = document.getElementById(button.getAttribute("data-date-open"));
					if (!input) return;
					if (input.showPicker) {
						input.showPicker();
					} else {
						input.focus();
					}
				});
			});
			document.querySelectorAll("[data-modal-close]").forEach(function (button) {
				button.addEventListener("click", function () {
					closeModal(button.closest(".modal"));
				});
			});
			document.querySelectorAll(".modal").forEach(function (modal) {
				modal.addEventListener("click", function (event) {
					if (event.target === modal) closeModal(modal);
				});
			});
			var drawer = document.querySelector(".app-sidebar");
			var mobileMenu = window.matchMedia("(max-width: 700px)");
			function setMenu(open) {
				if (drawer) drawer.classList.toggle("open", open);
				document.querySelectorAll("[data-menu-toggle]").forEach(function (button) {
					button.setAttribute("aria-expanded", open ? "true" : "false");
				});
				if (!mobileMenu.matches) localStorage.setItem("homebase-sidebar-open", open ? "true" : "false");
			}
			if (drawer && !mobileMenu.matches && localStorage.getItem("homebase-sidebar-open") === "true") setMenu(true);
			document.querySelectorAll("[data-menu-toggle]").forEach(function (button) {
				button.addEventListener("click", function () {
					setMenu(!(drawer && drawer.classList.contains("open")));
				});
			});
			document.querySelectorAll("[data-menu-close]").forEach(function (button) {
				button.addEventListener("click", function () { setMenu(false); });
			});
			if (drawer) {
				drawer.querySelectorAll("a").forEach(function (link) {
					link.addEventListener("click", function () {
						if (mobileMenu.matches) setMenu(false);
					});
				});
			}
			mobileMenu.addEventListener("change", function () {
				if (mobileMenu.matches) setMenu(false);
			});
			var universalSearch = document.querySelector("[data-universal-search]");
			var universalResults = document.querySelector("[data-universal-search-results]");
			var universalSearchTimer = null;
			var universalSearchAbort = null;
			function closeUniversalSearch() {
				if (!universalResults) return;
				universalResults.classList.remove("open");
				universalResults.innerHTML = "";
			}
			function renderUniversalResults(results) {
				if (!universalResults) return;
				universalResults.innerHTML = "";
				if (!results.length) {
					var empty = document.createElement("div");
					empty.className = "item meta";
					empty.textContent = "No matches";
					universalResults.appendChild(empty);
					universalResults.classList.add("open");
					return;
				}
				results.forEach(function (result) {
					var link = document.createElement("a");
					link.className = "universal-result";
					link.href = result.url;
					var type = document.createElement("span");
					type.className = "result-type";
					type.textContent = result.type || "Result";
					var title = document.createElement("strong");
					title.textContent = result.title || "Untitled";
					link.appendChild(type);
					link.appendChild(title);
					if (result.subtitle) {
						var subtitle = document.createElement("span");
						subtitle.className = "meta";
						subtitle.textContent = result.subtitle;
						link.appendChild(subtitle);
					}
					universalResults.appendChild(link);
				});
				universalResults.classList.add("open");
			}
			if (universalSearch && universalResults) {
				universalSearch.addEventListener("input", function () {
					window.clearTimeout(universalSearchTimer);
					var query = universalSearch.value.trim();
					if (query.length < 2) {
						closeUniversalSearch();
						return;
					}
					universalSearchTimer = window.setTimeout(function () {
						if (universalSearchAbort) universalSearchAbort.abort();
						universalSearchAbort = new AbortController();
						fetch("/search?q=" + encodeURIComponent(query), { signal: universalSearchAbort.signal })
							.then(function (response) {
								if (!response.ok) throw new Error("Search failed");
								return response.json();
							})
							.then(renderUniversalResults)
							.catch(function (error) {
								if (error.name !== "AbortError") closeUniversalSearch();
							});
					}, 160);
				});
				document.addEventListener("click", function (event) {
					if (!event.target.closest(".universal-search")) closeUniversalSearch();
				});
			}
			var dashboardListSelect = document.querySelector("[data-dashboard-list-select]");
			if (dashboardListSelect) {
				var storedList = localStorage.getItem("homebase-dashboard-list-id");
				var currentList = new URLSearchParams(window.location.search).get("list_id");
				if (!currentList && storedList && dashboardListSelect.querySelector('option[value="' + storedList + '"]') && dashboardListSelect.value !== storedList) {
					window.location.href = "/?list_id=" + encodeURIComponent(storedList);
					return;
				}
				localStorage.setItem("homebase-dashboard-list-id", dashboardListSelect.value);
				dashboardListSelect.addEventListener("change", function () {
					localStorage.setItem("homebase-dashboard-list-id", dashboardListSelect.value);
					window.location.href = "/?list_id=" + encodeURIComponent(dashboardListSelect.value);
				});
			}
			document.addEventListener("keydown", function (event) {
				if (event.key !== "Escape") return;
				document.querySelectorAll(".modal.open").forEach(closeModal);
				setMenu(false);
				closeUniversalSearch();
			});
			document.querySelectorAll(".action-menu").forEach(function (menu) {
				menu.addEventListener("toggle", function () {
					if (!menu.open) return;
					document.querySelectorAll(".action-menu[open]").forEach(function (other) {
						if (other !== menu) other.open = false;
					});
					menu.classList.remove("drop-up");
					var panel = menu.querySelector(".action-menu-panel");
					if (!panel) return;
					var rect = panel.getBoundingClientRect();
					if (rect.bottom > window.innerHeight - 12) menu.classList.add("drop-up");
				});
			});
			document.querySelectorAll("[data-auto-submit]").forEach(function (field) {
				field.addEventListener("change", function () {
					if (field.form) field.form.requestSubmit();
				});
			});
			document.querySelectorAll("[data-set-field]").forEach(function (button) {
				button.addEventListener("click", function () {
					var form = document.getElementById(button.getAttribute("data-form"));
					if (!form) return;
					var field = form.elements[button.getAttribute("data-set-field")];
					if (!field) return;
					field.value = button.getAttribute("data-value");
					form.requestSubmit();
				});
			});
			document.querySelectorAll("[data-date-field]").forEach(function (fieldControl) {
				fieldControl.addEventListener("change", function () {
					var form = document.getElementById(fieldControl.getAttribute("data-form"));
					if (!form) return;
					var field = form.elements[fieldControl.getAttribute("data-date-field")];
					if (!field) return;
					field.value = fieldControl.value;
					form.requestSubmit();
				});
			});
			function applyFilter(scope) {
				var input = scope.querySelector("[data-filter-input]");
				var kindSelect = scope.querySelector("[data-filter-kind]");
				var dueSelect = scope.querySelector("[data-filter-due]");
				var query = input ? input.value.trim().toLowerCase() : "";
				var kind = kindSelect ? kindSelect.value.trim().toLowerCase() : "";
				var due = dueSelect ? dueSelect.value.trim().toLowerCase() : "";
				var visible = 0;
				scope.querySelectorAll("[data-filter-item]").forEach(function (item) {
					var text = (item.getAttribute("data-filter-text") || item.textContent || "").toLowerCase();
					var itemKind = (item.getAttribute("data-filter-kind") || "").toLowerCase();
					var itemDue = (item.getAttribute("data-filter-due") || "").toLowerCase();
					var matchQuery = query === "" || text.indexOf(query) !== -1;
					var matchKind = kind === "" || itemKind === kind;
					var matchDue = due === "" || itemDue === due;
					var match = matchQuery && matchKind && matchDue;
					item.hidden = !match;
					if (match) visible += 1;
				});
				scope.querySelectorAll("[data-filter-empty]").forEach(function (empty) {
					empty.hidden = visible !== 0;
				});
			}
			document.querySelectorAll("[data-filter-input]").forEach(function (input) {
				input.addEventListener("input", function () {
					var scope = input.closest("[data-filter-scope]") || document;
					applyFilter(scope);
				});
			});
			document.querySelectorAll("[data-filter-kind]").forEach(function (select) {
				select.addEventListener("change", function () {
					var scope = select.closest("[data-filter-scope]") || document;
					applyFilter(scope);
				});
			});
			document.querySelectorAll("[data-filter-due]").forEach(function (select) {
				var scope = select.closest("[data-filter-scope]") || document;
				applyFilter(scope);
				select.addEventListener("change", function () {
					applyFilter(scope);
					var url = new URL(window.location.href);
					if (select.value) url.searchParams.set("due", select.value);
					else url.searchParams.delete("due");
					window.history.replaceState({}, "", url);
				});
			});
			document.querySelectorAll("[data-folder-toggle]").forEach(function (button) {
				button.addEventListener("click", function () {
					var folderID = button.getAttribute("data-folder-toggle");
					var collapsed = button.getAttribute("aria-expanded") === "true";
					button.setAttribute("aria-expanded", collapsed ? "false" : "true");
					button.textContent = collapsed ? ">" : "v";
					document.querySelectorAll('[data-folder-parent="' + folderID + '"]').forEach(function (row) {
						row.hidden = collapsed;
					});
				});
			});
			var draggedTask = null;
			document.querySelectorAll("[data-task-row]").forEach(function (row) {
				row.setAttribute("draggable", "true");
				row.addEventListener("dragstart", function (event) {
					draggedTask = row.getAttribute("data-task-row");
					row.classList.add("dragging");
					event.dataTransfer.effectAllowed = "move";
					event.dataTransfer.setData("text/plain", draggedTask);
				});
				row.addEventListener("dragend", function () {
					row.classList.remove("dragging");
					draggedTask = null;
					document.querySelectorAll("[data-folder-drop]").forEach(function (target) {
						target.classList.remove("drop-target");
					});
				});
			});
			document.querySelectorAll("[data-folder-drop]").forEach(function (target) {
				target.addEventListener("dragover", function (event) {
					event.preventDefault();
					target.classList.add("drop-target");
				});
				target.addEventListener("dragleave", function () {
					target.classList.remove("drop-target");
				});
				target.addEventListener("drop", function (event) {
					event.preventDefault();
					target.classList.remove("drop-target");
					var taskID = event.dataTransfer.getData("text/plain") || draggedTask;
					var form = document.getElementById("project-task-inline-" + taskID);
					if (!form) return;
					var field = form.elements.project_folder_id;
					if (!field) return;
					field.value = target.getAttribute("data-folder-drop");
					form.requestSubmit();
				});
			});
		})();

		(function () {
			var grid = document.querySelector(".dashboard-masonry");
			if (!grid) return;
			var shell = grid.closest(".dashboard-shell");
			var editToggle = document.querySelector("[data-dashboard-edit-toggle]");
			var addTile = grid.querySelector(".dashboard-add-tile");
			var draggingTile = null;
			var columnStorageKey = "homebase-dashboard-tile-columns";

			function tiles() {
				return Array.prototype.slice.call(grid.querySelectorAll(".dashboard-tile[data-tile]:not([hidden])"));
			}

			function layoutItems() {
				return Array.prototype.slice.call(grid.querySelectorAll(".dashboard-tile:not([hidden])")).filter(function (tile) {
					return tile.dataset.tile || editing() && !tile.classList.contains("empty");
				});
			}

			function setOrderStyles() {
				layoutItems().forEach(function (tile, index) {
					tile.style.order = index + 1;
				});
			}

			function storedColumns() {
				try {
					return JSON.parse(localStorage.getItem(columnStorageKey) || "{}");
				} catch (error) {
					return {};
				}
			}

			function saveColumnHints() {
				var columns = {};
				tiles().forEach(function (tile) {
					if (tile.dataset.column) columns[tile.dataset.tile] = tile.dataset.column;
				});
				localStorage.setItem(columnStorageKey, JSON.stringify(columns));
			}

			function applyColumnHints() {
				var columns = storedColumns();
				layoutItems().forEach(function (tile) {
					var column = columns[tile.dataset.tile];
					if (!tile.classList.contains("full-width") && (column === "1" || column === "2")) {
						tile.dataset.column = column;
					}
				});
			}

			function columnForPoint(x) {
				if (window.matchMedia("(max-width: 860px)").matches) return "";
				var rect = grid.getBoundingClientRect();
				return x < rect.left + rect.width / 2 ? "1" : "2";
			}

			function captureCurrentColumns() {
				if (window.matchMedia("(max-width: 860px)").matches) return;
				var rect = grid.getBoundingClientRect();
				var middle = rect.left + rect.width / 2;
				tiles().forEach(function (tile) {
					if (tile.classList.contains("full-width")) return;
					var tileRect = tile.getBoundingClientRect();
					tile.dataset.column = tileRect.left + tileRect.width / 2 < middle ? "1" : "2";
				});
			}

			function sortInitialTiles() {
				tiles().sort(function (a, b) {
					return (parseInt(a.style.order || "0", 10) || 0) - (parseInt(b.style.order || "0", 10) || 0);
				}).forEach(function (tile) {
					grid.insertBefore(tile, addTile || null);
				});
				setOrderStyles();
				applyColumnHints();
			}

			function layoutTiles() {
				if (window.matchMedia("(max-width: 860px)").matches) {
					grid.style.height = "auto";
					layoutItems().forEach(function (tile) {
						tile.style.position = "";
						tile.style.width = "";
						tile.style.transform = "";
					});
					return;
				}

				var styles = window.getComputedStyle(grid);
				var gap = parseFloat(styles.getPropertyValue("column-gap")) || 20;
				var gridWidth = grid.getBoundingClientRect().width;
				var columnWidth = (gridWidth - gap) / 2;
				var heights = [0, 0];

				layoutItems().forEach(function (tile) {
					tile.style.position = "absolute";
					tile.style.width = tile.classList.contains("full-width") ? gridWidth + "px" : columnWidth + "px";
				});

				layoutItems().forEach(function (tile) {
					var isFullWidth = tile.classList.contains("full-width");
					var top = 0;
					var left = 0;

					if (isFullWidth) {
						top = Math.max(heights[0], heights[1]);
						heights[0] = top + tile.offsetHeight + gap;
						heights[1] = heights[0];
					} else {
						var column = tile.dataset.column === "2" ? 1 : tile.dataset.column === "1" ? 0 : heights[0] <= heights[1] ? 0 : 1;
						tile.dataset.column = String(column + 1);
						left = column * (columnWidth + gap);
						top = heights[column];
						heights[column] = top + tile.offsetHeight + gap;
					}

					tile.style.transform = "translate(" + left + "px, " + top + "px)";
				});

				grid.style.height = Math.max(0, Math.max(heights[0], heights[1]) - gap) + "px";
			}

			function updateAddTile() {
				if (!addTile) return;
				var active = {};
				tiles().forEach(function (tile) {
					active[tile.dataset.tile] = true;
				});
				var missing = 0;
				addTile.querySelectorAll("[data-dashboard-add-tile]").forEach(function (button) {
					var isActive = active[button.dataset.dashboardAddTile];
					button.hidden = isActive;
					if (!isActive) missing += 1;
				});
				addTile.classList.toggle("empty", missing === 0);
				if (missing === 0) addTile.classList.remove("open");
			}

			function saveTileOrder() {
				fetch("/dashboard/tiles/order", {
					method: "POST",
					headers: { "Content-Type": "application/json" },
					body: JSON.stringify({ tiles: tiles().map(function (tile) { return tile.dataset.tile; }) })
				}).catch(function () {});
			}

			function replaceDashboardCalendar(url, pushState) {
				var current = grid.querySelector('.dashboard-tile[data-tile="calendar"]');
				if (!current) {
					window.location.href = url;
					return;
				}
				fetch(url, { headers: { "X-Requested-With": "fetch" } }).then(function (response) {
					if (!response.ok) throw new Error("calendar load failed");
					return response.text();
				}).then(function (html) {
					var nextDocument = new DOMParser().parseFromString(html, "text/html");
					var next = nextDocument.querySelector('.dashboard-tile[data-tile="calendar"]');
					if (!next) {
						window.location.href = url;
						return;
					}
					current.innerHTML = next.innerHTML;
					if (pushState) history.pushState({ dashboardCalendarURL: url }, "", url);
					window.requestAnimationFrame(layoutTiles);
				}).catch(function () {
					window.location.href = url;
				});
			}

			function closestTile(x, y) {
				var best = null;
				var bestDistance = Infinity;
				tiles().forEach(function (tile) {
					if (tile === draggingTile) return;
					var rect = tile.getBoundingClientRect();
					var closestX = Math.max(rect.left, Math.min(x, rect.right));
					var closestY = Math.max(rect.top, Math.min(y, rect.bottom));
					var dx = x - closestX;
					var dy = y - closestY;
					var distance = dx * dx + dy * dy;
					if (distance < bestDistance) {
						bestDistance = distance;
						best = tile;
					}
				});
				return best;
			}

			function placeDraggingTile(event) {
				var target = event.target.closest(".dashboard-tile");
				if (!target || target === draggingTile) target = closestTile(event.clientX, event.clientY);
				if (!target) {
					grid.appendChild(draggingTile);
					return;
				}

				var rect = target.getBoundingClientRect();
				var middleY = rect.top + rect.height / 2;
				var middleX = rect.left + rect.width / 2;
				var after = false;
				if (event.clientY < rect.top) {
					after = false;
				} else if (event.clientY > rect.bottom) {
					after = true;
				} else if (target.classList.contains("full-width") || draggingTile.classList.contains("full-width")) {
					after = event.clientY > middleY;
				} else if (Math.abs(event.clientY - middleY) < rect.height * 0.35) {
					after = event.clientX > middleX;
				} else {
					after = event.clientY > middleY;
				}

				var reference = after ? target.nextSibling : target;
				if (reference === draggingTile) return;
				grid.insertBefore(draggingTile, reference);
			}

			function editing() {
				return shell && shell.classList.contains("editing");
			}

			function setEditing(enabled) {
				if (!shell) return;
				shell.classList.toggle("editing", enabled);
				if (editToggle) {
					editToggle.classList.toggle("active", enabled);
					editToggle.setAttribute("aria-pressed", enabled ? "true" : "false");
				}
				updateAddTile();
				window.requestAnimationFrame(layoutTiles);
			}

			if (editToggle) {
				editToggle.addEventListener("click", function () {
					setEditing(!editing());
				});
			}

			grid.addEventListener("click", function (event) {
				var remove = event.target.closest("[data-dashboard-remove-tile]");
				if (remove) {
					var tile = remove.closest(".dashboard-tile[data-tile]");
					if (!tile || tiles().length <= 1) return;
					tile.hidden = true;
					updateAddTile();
					setOrderStyles();
					layoutTiles();
					saveTileOrder();
					saveColumnHints();
					return;
				}

				var add = event.target.closest("[data-dashboard-add-tile]");
				if (add) {
					var key = add.dataset.dashboardAddTile;
					var tileToAdd = grid.querySelector('.dashboard-tile[data-tile="' + key + '"]');
					if (!tileToAdd) return;
					tileToAdd.hidden = false;
					grid.insertBefore(tileToAdd, addTile || null);
					updateAddTile();
					setOrderStyles();
					layoutTiles();
					saveTileOrder();
					saveColumnHints();
					return;
				}

				var addToggle = event.target.closest("[data-dashboard-add-toggle]");
				if (addToggle && addTile) {
					addTile.classList.toggle("open");
					window.requestAnimationFrame(layoutTiles);
				}
			});

			grid.addEventListener("click", function (event) {
				var calendarLink = event.target.closest("[data-dashboard-calendar-link]");
				if (!calendarLink) return;
				event.preventDefault();
				replaceDashboardCalendar(calendarLink.href, true);
			});

			window.addEventListener("popstate", function () {
				if (location.pathname === "/") replaceDashboardCalendar(location.href, false);
			});

			grid.addEventListener("dragstart", function (event) {
				var handle = event.target.closest(".drag-handle");
				if (!handle || !editing()) {
					event.preventDefault();
					return;
				}
				draggingTile = handle.closest(".dashboard-tile");
				if (!draggingTile) return;
				captureCurrentColumns();
				draggingTile.classList.add("dragging");
				event.dataTransfer.effectAllowed = "move";
				event.dataTransfer.setData("text/plain", draggingTile.dataset.tile || "");
			});

			grid.addEventListener("dragover", function (event) {
				if (!draggingTile) return;
				event.preventDefault();
				var column = columnForPoint(event.clientX);
				if (!draggingTile.classList.contains("full-width")) {
					if (column) draggingTile.dataset.column = column;
					else delete draggingTile.dataset.column;
				}
				placeDraggingTile(event);
				setOrderStyles();
				window.requestAnimationFrame(layoutTiles);
			});

			grid.addEventListener("dragend", function () {
				if (!draggingTile) return;
				draggingTile.classList.remove("dragging");
				draggingTile = null;
				setOrderStyles();
				layoutTiles();
				saveTileOrder();
				saveColumnHints();
			});

			sortInitialTiles();
			updateAddTile();
			window.addEventListener("resize", layoutTiles);
			window.addEventListener("load", layoutTiles);
			layoutTiles();
			grid.classList.add("dashboard-ready");
			window.requestAnimationFrame(layoutTiles);
			window.setTimeout(layoutTiles, 80);
			window.setTimeout(layoutTiles, 250);
		})();

		(function () {
			var toggle = document.getElementById("theme-toggle");
			if (!toggle) return;
			var current = document.documentElement.dataset.theme || "light";
			toggle.checked = current === "dark";
			toggle.addEventListener("change", function () {
				var next = toggle.checked ? "dark" : "light";
				document.documentElement.dataset.theme = next;
				localStorage.setItem("homebase-theme", next);
			});
		})();
	</script>
</body>
</html>
{{ end }}

{{ define "tileControls" }}
<button class="secondary compact dashboard-edit-control drag-handle" type="button" title="Move tile" aria-label="Move tile" draggable="true"><svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><circle cx="9" cy="6" r="1.5"></circle><circle cx="15" cy="6" r="1.5"></circle><circle cx="9" cy="12" r="1.5"></circle><circle cx="15" cy="12" r="1.5"></circle><circle cx="9" cy="18" r="1.5"></circle><circle cx="15" cy="18" r="1.5"></circle></svg></button>
<button class="danger compact dashboard-edit-control" type="button" title="Remove tile" aria-label="Remove tile" data-dashboard-remove-tile>X</button>
{{ end }}

{{ define "pageHeader" }}
<section class="detail-hero">
	<div class="title-row">
		<div class="title-copy">
			<div class="page-title-line">
				<a class="button secondary back-icon" href="{{ .BackURL }}" title="Back" aria-label="Back">‹</a>
				<h1>{{ .Title }}</h1>
			</div>
		</div>
	</div>
</section>
{{ end }}

{{ define "titleActionMenu" }}
<details class="action-menu left">
	<summary class="button secondary title-icon-button" title="Actions" aria-label="Actions">⋯</summary>
	<div class="action-menu-panel">
		{{ if .EditModal }}<button class="secondary compact" type="button" data-modal-open="{{ .EditModal }}">Edit</button>{{ end }}
		{{ if .ReopenForm }}<button class="secondary compact" type="submit" form="{{ .ReopenForm }}">Reopen</button>{{ end }}
		{{ if .DeleteForm }}<button class="danger compact" type="submit" form="{{ .DeleteForm }}">{{ if .DeleteLabel }}{{ .DeleteLabel }}{{ else }}Delete{{ end }}</button>{{ end }}
		{{ if .ArchiveForm }}<button class="danger compact" type="submit" form="{{ .ArchiveForm }}">Archive</button>{{ end }}
	</div>
</details>
{{ end }}

{{ define "login" }}
<main class="shell login">
	<section class="login-box">
		<h1>Homebase</h1>
		<p>Household projects, tasks, appointments, routines, and records in one operations view.</p>
		{{ if .Error }}<p class="panel error">{{ .Error }}</p>{{ end }}
		<a class="button" href="{{ .LoginURL }}">Login</a>
	</section>
</main>
{{ end }}

{{ define "dashboard" }}
<main class="shell dashboard-shell">
	<div class="dashboard-toolbar">
		<div class="meta">{{ datetime .Now }}</div>
		<button class="secondary compact dashboard-edit-toggle" type="button" title="Edit dashboard layout" aria-label="Edit dashboard layout" aria-pressed="false" data-dashboard-edit-toggle>&#9998;</button>
	</div>

	{{ if .Error }}<p class="panel error">{{ .Error }}</p>{{ end }}

	<div class="dashboard-tiles dashboard-masonry">
		{{ if .Calendar }}
		<section class="panel dashboard-tile full-width" data-tile="calendar"{{ if not (tileActive .Dashboard.TileOrder "calendar") }} hidden{{ end }} style="order:{{ order .Dashboard.TileOrder "calendar" }}">
			<div class="tile-head">
				<h2>Calendar</h2>
				<div class="tile-actions">{{ template "tileControls" "calendar" }}</div>
			</div>
			{{ template "dashboardCalendarControls" . }}
			{{ if eq .CalendarView "day" }}
				{{ template "calendarDayView" . }}
			{{ else if eq .CalendarView "week" }}
				{{ template "calendarWeekView" . }}
			{{ else }}
				{{ template "calendarMonthGrid" .Calendar }}
			{{ end }}
		</section>
		{{ end }}

		<section id="tasks" class="panel dashboard-tile" data-tile="tasks"{{ if not (tileActive .Dashboard.TileOrder "tasks") }} hidden{{ end }} style="order:{{ order .Dashboard.TileOrder "tasks" }}">
			<div class="tile-head">
				<h2>Tasks</h2>
				<div class="tile-actions">
					<button class="button compact" type="button" data-modal-open="add-task" title="Add task" aria-label="Add task">+</button>
					{{ template "tileControls" "tasks" }}
				</div>
			</div>
			<div class="stat-grid">
				<a class="stat-link" href="/tasks?due=past"><strong>{{ taskStatCount .Dashboard.Tasks .Now "past" }}</strong><span class="meta">Past due</span></a>
				<a class="stat-link" href="/tasks?due=today"><strong>{{ taskStatCount .Dashboard.Tasks .Now "today" }}</strong><span class="meta">Due today</span></a>
				<a class="stat-link" href="/tasks?due=upcoming"><strong>{{ taskStatCount .Dashboard.Tasks .Now "upcoming" }}</strong><span class="meta">Upcoming</span></a>
			</div>
		</section>

		<section id="projects" class="panel dashboard-tile" data-tile="projects"{{ if not (tileActive .Dashboard.TileOrder "projects") }} hidden{{ end }} style="order:{{ order .Dashboard.TileOrder "projects" }}">
			<div class="tile-head">
				<h2>Projects</h2>
				<div class="tile-actions">
					<button class="button compact" type="button" data-modal-open="add-project" title="Add project" aria-label="Add project">+</button>
					{{ template "tileControls" "projects" }}
				</div>
			</div>
			<div class="stat-grid">
				<a class="stat-link" href="/projects?due=past"><strong>{{ projectStatCount .Dashboard.Projects .Now "past" }}</strong><span class="meta">Past due</span></a>
				<a class="stat-link" href="/projects?due=today"><strong>{{ projectStatCount .Dashboard.Projects .Now "today" }}</strong><span class="meta">Due today</span></a>
				<a class="stat-link" href="/projects?due=upcoming"><strong>{{ projectStatCount .Dashboard.Projects .Now "upcoming" }}</strong><span class="meta">Upcoming</span></a>
			</div>
		</section>

		<section class="panel dashboard-tile" data-tile="appointments"{{ if not (tileActive .Dashboard.TileOrder "appointments") }} hidden{{ end }} style="order:{{ order .Dashboard.TileOrder "appointments" }}">
			<div class="tile-head">
				<h2>Appointments</h2>
				<div class="tile-actions">
					<button class="button compact" type="button" data-modal-open="add-appointment" title="Add appointment" aria-label="Add appointment">+</button>
					{{ template "tileControls" "appointments" }}
				</div>
			</div>
			<div class="cards">
				<h3>Today</h3>
				{{ range eventsForDayOffset .Dashboard.Events .Now 0 }}
					<article class="item">
						<strong><a href="/events/{{ .ID }}">{{ .Title }}</a></strong>
						<div class="meta">{{ datetime .StartsAt }}{{ if .Location }} · {{ .Location }}{{ end }}</div>
					</article>
				{{ else }}
					<p class="empty">No appointments today.</p>
				{{ end }}
				<h3 style="margin-top:6px;">Tomorrow</h3>
				{{ range eventsForDayOffset .Dashboard.Events .Now 1 }}
					<article class="item">
						<strong><a href="/events/{{ .ID }}">{{ .Title }}</a></strong>
						<div class="meta">{{ datetime .StartsAt }}{{ if .Location }} · {{ .Location }}{{ end }}</div>
					</article>
				{{ else }}
					<p class="empty">No appointments tomorrow.</p>
				{{ end }}
			</div>
		</section>

		<section class="panel dashboard-tile" data-tile="list"{{ if not (tileActive .Dashboard.TileOrder "list") }} hidden{{ end }} style="order:{{ order .Dashboard.TileOrder "list" }}">
			<div class="tile-head">
				<h2>List</h2>
				<div class="tile-actions">
					{{ template "tileControls" "list" }}
				</div>
			</div>
			{{ if .Lists }}
			<select class="dashboard-list-select" data-dashboard-list-select>
				{{ range .Lists }}<option value="{{ .ID }}" {{ if eq $.List.ID .ID }}selected{{ end }}>{{ .Title }}</option>{{ end }}
			</select>
			{{ end }}
			<div class="checklist">
				{{ if .List.ID }}
				{{ range .ListItems }}
				<article class="check-item {{ .Status }}">
					{{ if eq .Status "open" }}
					<form method="post" action="/lists/{{ $.List.ID }}/items/{{ .ID }}/complete">
						<input type="hidden" name="return_to" value="/?list_id={{ $.List.ID }}">
						<button class="secondary check-toggle" type="submit" title="Complete item" aria-label="Complete item"></button>
					</form>
				{{ else }}
					<form method="post" action="/lists/{{ $.List.ID }}/items/{{ .ID }}/reopen">
						<input type="hidden" name="return_to" value="/?list_id={{ $.List.ID }}">
						<button class="secondary check-toggle" type="submit" title="Reopen item" aria-label="Reopen item">X</button>
					</form>
				{{ end }}
					<div class="check-main">
						<div class="check-title">{{ .Title }}</div>
						{{ if .Notes }}<div class="meta">{{ .Notes }}</div>{{ end }}
					</div>
				</article>
				{{ end }}
				{{ if not .ListItems }}<p class="empty">No items in this list.</p>{{ end }}
				{{ else }}
				<p class="empty">No lists yet.</p>
				{{ end }}
			</div>
		</section>

		<section class="panel dashboard-tile dashboard-add-tile">
			<div class="dashboard-add-menu">
				<button class="secondary dashboard-add-plus" type="button" title="Add tile" aria-label="Add tile" data-dashboard-add-toggle>+</button>
				<div class="dashboard-add-options">
					{{ range .Dashboard.AvailableTiles }}
						<button class="secondary compact" type="button" data-dashboard-add-tile="{{ .Key }}">{{ .Name }}</button>
					{{ end }}
				</div>
			</div>
		</section>
	</div>
	{{ template "dashboardModals" . }}
</main>
{{ end }}

{{ define "dashboardModals" }}
<section id="add-task" class="modal">
	<div class="modal-card">
		<div class="modal-head">
			<h2>Add Task</h2>
			<button class="secondary compact" type="button" data-modal-close>Close</button>
		</div>
		<form method="post" action="/tasks">
			<label>Title<input name="title" required></label>
			<label>Notes<textarea name="notes"></textarea></label>
			<div class="form-row">
				<label>Project<select name="project_id">
					<option value="">None</option>
					{{ range .Dashboard.Projects }}<option value="{{ .ID }}">{{ .Title }}</option>{{ end }}
				</select></label>
				<label>Assigned to<select name="assigned_to">
					<option value="">Unassigned</option>
					{{ range .Dashboard.Members }}<option value="{{ .ID }}">{{ .Name }}</option>{{ end }}
				</select></label>
			</div>
			<div class="form-row">
				<label>Due<input name="due_at" type="datetime-local"></label>
				<label>Priority<select name="priority"><option>normal</option><option>high</option><option>low</option></select></label>
			</div>
			<button type="submit">Add task</button>
		</form>
	</div>
</section>

<section id="add-project" class="modal">
	<div class="modal-card">
		<div class="modal-head">
			<h2>Add Project</h2>
			<button class="secondary compact" type="button" data-modal-close>Close</button>
		</div>
		<form method="post" action="/projects">
			<label>Title<input name="title" required></label>
			<label>Description<textarea name="description"></textarea></label>
			<div class="form-row">
				<label>Due<input name="due_date" type="date"></label>
				<label>Priority<select name="priority"><option>normal</option><option>high</option><option>low</option></select></label>
			</div>
			<button type="submit">Add project</button>
		</form>
	</div>
</section>

<section id="add-appointment" class="modal">
	<div class="modal-card">
		<div class="modal-head">
			<h2>Add Appointment</h2>
			<button class="secondary compact" type="button" data-modal-close>Close</button>
		</div>
		<form method="post" action="/events">
			<label>Title<input name="title" required></label>
			<label>Location<input name="location"></label>
			<div class="form-row">
				<label>Starts<input name="starts_at" type="datetime-local" required></label>
				<label>Ends<input name="ends_at" type="datetime-local" required></label>
			</div>
			<label>Description<textarea name="description"></textarea></label>
			<button type="submit">Add appointment</button>
		</form>
	</div>
</section>

<section id="add-routine" class="modal">
	<div class="modal-card">
		<div class="modal-head">
			<h2>Add Routine</h2>
			<button class="secondary compact" type="button" data-modal-close>Close</button>
		</div>
		<form method="post" action="/routines">
			<label>Title<input name="title" required></label>
			<label>Notes<textarea name="notes"></textarea></label>
			<div class="form-row">
				<label>Cadence<select name="cadence">
					<option value="monthly">monthly</option>
					<option value="daily">daily</option>
					<option value="weekly">weekly</option>
					<option value="quarterly">quarterly</option>
					<option value="yearly">yearly</option>
				</select></label>
				<label>Assigned to<select name="assigned_to">
					<option value="">Unassigned</option>
					{{ range .Dashboard.Members }}<option value="{{ .ID }}">{{ .Name }}</option>{{ end }}
				</select></label>
			</div>
			<label>Next due<input name="next_due_at" type="date"></label>
			<button type="submit">Add routine</button>
		</form>
	</div>
</section>

{{ if eq .Dashboard.Household.Role "owner" }}
<section id="add-member" class="modal">
	<div class="modal-card">
		<div class="modal-head">
			<h2>Add Member</h2>
			<button class="secondary compact" type="button" data-modal-close>Close</button>
		</div>
		<form method="post" action="/members">
			<div class="form-row">
				<label>Email<input name="email" type="email" required></label>
				<label>Name<input name="name"></label>
			</div>
			<label>Role<select name="role"><option>member</option><option>owner</option></select></label>
			<button type="submit">Add member</button>
		</form>
	</div>
</section>

{{ range .Dashboard.Members }}
<section id="edit-member-{{ .ID }}" class="modal">
	<div class="modal-card">
		<div class="modal-head">
			<h2>Edit Member</h2>
			<button class="secondary compact" type="button" data-modal-close>Close</button>
		</div>
		<form method="post" action="/members/{{ .ID }}">
			<div class="form-row">
				<label>Email<input name="email" type="email" value="{{ .Email }}" required></label>
				<label>Name<input name="name" value="{{ .Name }}" required></label>
			</div>
			<label>Role<select name="role">
				<option value="member"{{ selectedString .Role "member" }}>member</option>
				<option value="owner"{{ selectedString .Role "owner" }}>owner</option>
			</select></label>
			<button type="submit">Save member</button>
		</form>
		{{ if ne .ID $.Dashboard.CurrentUser.ID }}
		<form method="post" action="/members/{{ .ID }}/remove" style="margin-top:10px;">
			<button class="danger" type="submit">Remove member</button>
		</form>
		{{ end }}
	</div>
</section>
{{ end }}
{{ end }}
{{ end }}

{{ define "calendar" }}
<main class="shell">
	<section class="detail-hero">
		<div class="title-row">
			<div class="title-copy">
				<div class="month-switch">
					<a class="button secondary back-icon" href="/projects" title="Back" aria-label="Back">‹</a>
					<a class="button secondary" href="/calendar?month={{ .Calendar.PrevMonth }}">&lt;</a>
					<h1 class="month-title">{{ monthTitle .Calendar.Month }}</h1>
					<a class="button secondary" href="/calendar?month={{ .Calendar.NextMonth }}">&gt;</a>
				</div>
				<p>Appointments, due tasks, project dates, and routine due dates.</p>
			</div>
		</div>
	</section>

	{{ if .Error }}<p class="panel error">{{ .Error }}</p>{{ end }}

	{{ template "calendarMonthGrid" .Calendar }}
</main>
{{ end }}

{{ define "dashboardCalendarControls" }}
<div class="calendar-toolbar">
	<div class="month-switch">
		<a class="button secondary" href="{{ dashboardCalendarURL .CalendarView (calendarStep .CalendarView .CalendarFocus -1) }}" data-dashboard-calendar-link>&lt;</a>
		<span class="month-title">{{ monthTitle .Calendar.Month }}</span>
		<a class="button secondary" href="{{ dashboardCalendarURL .CalendarView (calendarStep .CalendarView .CalendarFocus 1) }}" data-dashboard-calendar-link>&gt;</a>
		<a class="button secondary" href="{{ dashboardCalendarURL .CalendarView .Now }}" data-dashboard-calendar-link>Today</a>
	</div>
	<div class="view-tabs">
		<a class="button secondary{{ if eq .CalendarView "day" }} active{{ end }}" href="{{ dashboardCalendarURL "day" .CalendarFocus }}" data-dashboard-calendar-link>Day</a>
		<a class="button secondary{{ if eq .CalendarView "week" }} active{{ end }}" href="{{ dashboardCalendarURL "week" .CalendarFocus }}" data-dashboard-calendar-link>Week</a>
		<a class="button secondary{{ if eq .CalendarView "month" }} active{{ end }}" href="{{ dashboardCalendarURL "month" .CalendarFocus }}" data-dashboard-calendar-link>Month</a>
	</div>
</div>
{{ end }}

{{ define "calendarMonthGrid" }}
<section class="calendar-grid">
	<div class="weekday">Sun</div>
	<div class="weekday">Mon</div>
	<div class="weekday">Tue</div>
	<div class="weekday">Wed</div>
	<div class="weekday">Thu</div>
	<div class="weekday">Fri</div>
	<div class="weekday">Sat</div>
	{{ range .Days }}
		{{ template "calendarDayCell" . }}
	{{ end }}
</section>
{{ end }}

{{ define "calendarDayCell" }}
<div class="calendar-day{{ if not .InMonth }} outside{{ end }}{{ if .IsToday }} today{{ end }}{{ if not .Entries }} empty-day{{ end }}">
	<div class="day-head">
		<span class="day-number">{{ dayNumber .Date }}</span>
		<div class="day-badges">
			{{ if .IsToday }}<span class="badge">Today</span>{{ end }}
			{{ if .Entries }}<span class="badge">{{ len .Entries }}</span>{{ end }}
		</div>
	</div>
	<div class="calendar-entries">
		{{ range .Entries }}
			{{ template "calendarEntry" . }}
		{{ end }}
	</div>
</div>
{{ end }}

{{ define "calendarEntry" }}
<a class="calendar-entry {{ .Type }} {{ .Priority }} {{ .Status }}" href="{{ .URL }}">
	{{ if .Time }}{{ if entryTime .Time }}<span class="meta">{{ entryTime .Time }}</span>{{ end }}{{ end }}
	<strong>{{ .Title }}</strong>
	{{ if .Meta }}<span class="meta">{{ .Meta }}</span>{{ end }}
</a>
{{ end }}

{{ define "calendarWeekView" }}
<section class="calendar-week-grid">
	{{ range calendarWeek .Calendar .CalendarFocus }}
		<div class="calendar-day{{ if not .InMonth }} outside{{ end }}{{ if .IsToday }} today{{ end }}{{ if not .Entries }} empty-day{{ end }}">
			<div class="day-head">
				<span class="day-number">{{ weekdayShort .Date }} {{ dateShort .Date }}</span>
				<div class="day-badges">
					{{ if .IsToday }}<span class="badge">Today</span>{{ end }}
					{{ if .Entries }}<span class="badge">{{ len .Entries }}</span>{{ end }}
				</div>
			</div>
			<div class="calendar-entries">
				{{ range .Entries }}
					{{ template "calendarEntry" . }}
				{{ end }}
			</div>
		</div>
	{{ end }}
</section>
{{ end }}

{{ define "calendarDayView" }}
{{ $day := calendarDay .Calendar .CalendarFocus }}
<div class="calendar-list">
	<div class="calendar-list-day">
		<div class="calendar-list-date">{{ dateTitle $day.Date }}</div>
		<div class="cards">
			{{ range $day.Entries }}
				{{ template "calendarEntry" . }}
			{{ else }}
				<p class="empty">No items.</p>
			{{ end }}
		</div>
	</div>
</div>
{{ end }}

{{ define "projectIndex" }}
<main class="shell">
	{{ template "pageHeader" (dict "BackURL" "/" "Title" "Projects") }}

	{{ if .Error }}<p class="panel error">{{ .Error }}</p>{{ end }}

	<section class="panel" data-filter-scope>
		<div class="tile-head">
			<h2>Household Projects</h2>
			<button class="button compact" type="button" data-modal-open="add-project-index" title="Add project" aria-label="Add project">+</button>
		</div>
		<div class="form-row filter-row">
			<label>Search<input type="search" data-filter-input placeholder="Search projects"></label>
			<label>Status<select data-filter-kind>
				<option value="">All statuses</option>
				<option value="active">active</option>
				<option value="waiting">waiting</option>
				<option value="done">done</option>
			</select></label>
			<label>Due<select data-filter-due>
				<option value="" {{ selectedString .DueFilter "" }}>All due dates</option>
				<option value="past" {{ selectedString .DueFilter "past" }}>Past due</option>
				<option value="today" {{ selectedString .DueFilter "today" }}>Due today</option>
				<option value="upcoming" {{ selectedString .DueFilter "upcoming" }}>Upcoming</option>
			</select></label>
		</div>
		<p class="empty" data-filter-empty hidden>No matching projects.</p>
		<div class="cards">
			{{ range .Projects }}
			{{ $project := . }}
			{{ $formID := printf "project-index-inline-%d" $project.ID }}
			<form id="{{ $formID }}" method="post" action="/projects/{{ $project.ID }}">
				<input type="hidden" name="return_to" value="/projects{{ if $.DueFilter }}?due={{ $.DueFilter }}{{ end }}">
				<input type="hidden" name="title" value="{{ $project.Title }}">
				<textarea name="description" hidden>{{ $project.Description }}</textarea>
				<input type="hidden" name="status" value="{{ $project.Status }}">
				<input type="hidden" name="priority" value="{{ $project.Priority }}">
				<input type="hidden" name="due_date" value="{{ dateInput $project.DueDate }}">
			</form>
			<article class="item task-index-card" data-filter-item data-filter-kind="{{ $project.Status }}" data-filter-due="{{ projectDueBucket $project $.Now }}" data-filter-text="{{ $project.Title }} {{ $project.Description }} {{ $project.Status }} {{ $project.Priority }}">
				<div class="item-head">
					<div>
						<strong><a href="/projects/{{ $project.ID }}">{{ $project.Title }}</a></strong>
						{{ if $project.Description }}<div class="meta">{{ $project.Description }}</div>{{ end }}
					</div>
				</div>
				<div class="task-card-controls">
					{{ template "projectStatusControl" (dict "Project" $project "FormID" $formID) }}
					{{ template "projectPriorityControl" (dict "Project" $project "FormID" $formID) }}
					{{ template "projectDueControl" (dict "Project" $project "FormID" $formID "InputID" (printf "project-index-due-%d" $project.ID)) }}
				</div>
			</article>
			{{ else }}
			<p class="empty">No projects yet.</p>
			{{ end }}
		</div>
	</section>

	<section id="add-project-index" class="modal">
		<div class="modal-card">
			<div class="modal-head">
				<h2>Add Project</h2>
				<button class="secondary compact" type="button" data-modal-close>Close</button>
			</div>
			<form method="post" action="/projects">
				<input type="hidden" name="return_to" value="/projects">
				<label>Title<input name="title" required></label>
				<label>Description<textarea name="description"></textarea></label>
				<div class="form-row">
					<label>Due<input name="due_date" type="date"></label>
					<label>Priority<select name="priority"><option>normal</option><option>high</option><option>low</option></select></label>
				</div>
				<button type="submit">Add project</button>
			</form>
		</div>
	</section>
</main>
{{ end }}

{{ define "taskIndex" }}
<main class="shell">
	{{ template "pageHeader" (dict "BackURL" "/" "Title" "Tasks") }}

	{{ if .Error }}<p class="panel error">{{ .Error }}</p>{{ end }}

	<section class="panel" data-filter-scope>
		<div class="tile-head">
			<h2>Household Tasks</h2>
			<button class="button compact" type="button" data-modal-open="add-task-index" title="Add task" aria-label="Add task">+</button>
		</div>
		<div class="form-row filter-row">
			<label>Search<input type="search" data-filter-input placeholder="Search tasks"></label>
			<label>Status<select data-filter-kind>
				<option value="">All statuses</option>
				<option value="open">open</option>
				<option value="done">done</option>
			</select></label>
			<label>Due<select data-filter-due>
				<option value="" {{ selectedString .DueFilter "" }}>All due dates</option>
				<option value="past" {{ selectedString .DueFilter "past" }}>Past due</option>
				<option value="today" {{ selectedString .DueFilter "today" }}>Due today</option>
				<option value="upcoming" {{ selectedString .DueFilter "upcoming" }}>Upcoming</option>
			</select></label>
		</div>
		<p class="empty" data-filter-empty hidden>No matching tasks.</p>
		<div class="cards">
			{{ range .Tasks }}
			{{ $task := . }}
			{{ $formID := printf "task-index-inline-%d" $task.ID }}
			<form id="{{ $formID }}" method="post" action="/tasks/{{ $task.ID }}">
				<input type="hidden" name="return_to" value="/tasks{{ if $.DueFilter }}?due={{ $.DueFilter }}{{ end }}">
				<input type="hidden" name="title" value="{{ $task.Title }}">
				<input type="hidden" name="notes" value="{{ $task.Notes }}">
				<input type="hidden" name="project_id" value="{{ idValue $task.ProjectID }}">
				<input type="hidden" name="project_folder_id" value="{{ idValue $task.ProjectFolderID }}">
				<input type="hidden" name="assigned_to" value="{{ idValue $task.AssignedTo }}">
				<input type="hidden" name="due_at" value="{{ datetimeInputPtr $task.DueAt }}">
				<input type="hidden" name="status" value="{{ $task.Status }}">
				<input type="hidden" name="priority" value="{{ $task.Priority }}">
				{{ if $task.RoutineID }}<input type="hidden" name="routine_id" value="{{ idValue $task.RoutineID }}">{{ end }}
				{{ if $task.AssetID }}<input type="hidden" name="asset_id" value="{{ idValue $task.AssetID }}">{{ end }}
				{{ if $task.AssetMaintenanceItemID }}<input type="hidden" name="asset_maintenance_item_id" value="{{ idValue $task.AssetMaintenanceItemID }}">{{ end }}
			</form>
			<article class="item task-index-card" data-filter-item data-filter-kind="{{ $task.Status }}" data-filter-due="{{ taskDueBucket $task $.Now }}" data-filter-text="{{ $task.Title }} {{ $task.Notes }} {{ $task.Status }} {{ $task.Priority }} {{ $task.AssignedName }}">
				<div class="item-head">
					<div>
						<strong><a href="/tasks/{{ $task.ID }}">{{ $task.Title }}</a></strong>
						{{ if $task.Notes }}<div class="meta">{{ $task.Notes }}</div>{{ end }}
						<div class="meta">{{ taskContext $task $.Dashboard.Projects $.Dashboard.Routines }}</div>
					</div>
				</div>
				<div class="task-card-controls">
					{{ template "taskStatusControl" (dict "Task" $task "FormID" $formID) }}
					{{ template "taskPriorityControl" (dict "Task" $task "FormID" $formID) }}
					{{ template "taskAssigneeControl" (dict "Task" $task "FormID" $formID "Members" $.Members) }}
					{{ template "taskDueControl" (dict "Task" $task "FormID" $formID "InputID" (printf "task-index-due-%d" $task.ID)) }}
				</div>
			</article>
			{{ else }}
			<p class="empty">No tasks yet.</p>
			{{ end }}
		</div>
	</section>

	<section id="add-task-index" class="modal">
		<div class="modal-card">
			<div class="modal-head">
				<h2>Add Task</h2>
				<button class="secondary compact" type="button" data-modal-close>Close</button>
			</div>
			<form method="post" action="/tasks">
				<input type="hidden" name="return_to" value="/tasks">
				<label>Title<input name="title" required></label>
				<label>Notes<textarea name="notes"></textarea></label>
				<div class="form-row">
					<label>Project<select name="project_id">
						<option value="">Standalone</option>
						{{ range .Projects }}<option value="{{ .ID }}">{{ .Title }}</option>{{ end }}
					</select></label>
					<label>Assigned to<select name="assigned_to">
						<option value="">Unassigned</option>
						{{ range .Members }}<option value="{{ .ID }}">{{ .Name }}</option>{{ end }}
					</select></label>
				</div>
				<div class="form-row">
					<label>Due<input name="due_at" type="datetime-local"></label>
					<label>Priority<select name="priority"><option>normal</option><option>high</option><option>low</option></select></label>
				</div>
				<button type="submit">Add task</button>
			</form>
		</div>
	</section>
</main>
{{ end }}

{{ define "routineIndex" }}
<main class="shell">
	{{ template "pageHeader" (dict "BackURL" "/" "Title" "Routines") }}

	{{ if .Error }}<p class="panel error">{{ .Error }}</p>{{ end }}

	<section class="panel" data-filter-scope>
		<div class="tile-head">
			<h2>Household Routines</h2>
			<button class="button compact" type="button" data-modal-open="add-routine-index" title="Add routine" aria-label="Add routine">+</button>
		</div>
		<div class="form-row filter-row">
			<label>Search<input type="search" data-filter-input placeholder="Search routines"></label>
			<label>Cadence<select data-filter-kind>
				<option value="">All cadences</option>
				<option value="daily">daily</option>
				<option value="weekly">weekly</option>
				<option value="monthly">monthly</option>
				<option value="quarterly">quarterly</option>
				<option value="yearly">yearly</option>
			</select></label>
		</div>
		<p class="empty" data-filter-empty hidden>No matching routines.</p>
		<div class="cards">
			{{ range .Routines }}
			<article class="item" data-filter-item data-filter-kind="{{ .Cadence }}" data-filter-text="{{ .Title }} {{ .Notes }} {{ .Cadence }} {{ .Status }} {{ .AssignedName }}">
				<div class="item-head">
					<div>
						<strong><a href="/routines/{{ .ID }}">{{ .Title }}</a></strong>
						{{ if .Notes }}<div class="meta">{{ .Notes }}</div>{{ end }}
						<div class="meta">{{ .Cadence }}{{ if .AssignedName }} · {{ .AssignedName }}{{ end }}{{ if .NextDueAt }} · Due {{ date .NextDueAt }}{{ end }}</div>
					</div>
					<span class="badge">{{ .Status }}</span>
				</div>
			</article>
			{{ else }}
			<p class="empty">No routines yet.</p>
			{{ end }}
		</div>
	</section>

	<section id="add-routine-index" class="modal">
		<div class="modal-card">
			<div class="modal-head">
				<h2>Add Routine</h2>
				<button class="secondary compact" type="button" data-modal-close>Close</button>
			</div>
			<form method="post" action="/routines">
				<input type="hidden" name="return_to" value="/routines">
				<label>Title<input name="title" required></label>
				<label>Notes<textarea name="notes"></textarea></label>
				<div class="form-row">
					<label>Cadence<select name="cadence">
						<option value="monthly">monthly</option>
						<option value="daily">daily</option>
						<option value="weekly">weekly</option>
						<option value="quarterly">quarterly</option>
						<option value="yearly">yearly</option>
					</select></label>
					<label>Assigned to<select name="assigned_to">
						<option value="">Unassigned</option>
						{{ range .Members }}<option value="{{ .ID }}">{{ .Name }}</option>{{ end }}
					</select></label>
				</div>
				<label>Next due<input name="next_due_at" type="date"></label>
				<button type="submit">Add routine</button>
			</form>
		</div>
	</section>
</main>
{{ end }}

{{ define "memberIndex" }}
<main class="shell">
	{{ template "pageHeader" (dict "BackURL" "/" "Title" "Users") }}

	{{ if .Error }}<p class="panel error">{{ .Error }}</p>{{ end }}

	<section class="panel">
		<div class="tile-head">
			<h2>Household Users</h2>
			{{ if eq .Dashboard.Household.Role "owner" }}<button class="button compact" type="button" data-modal-open="add-member-index" title="Add user" aria-label="Add user">+</button>{{ end }}
		</div>
		<div class="cards">
			{{ range .Members }}
			<article class="item">
				<div class="item-head">
					<div>
						<strong>{{ .Name }}</strong>
						<div class="meta">{{ .Email }}</div>
					</div>
					<div class="tile-actions">
						<span class="badge">{{ .Role }}</span>
						{{ if eq $.Dashboard.Household.Role "owner" }}<button class="secondary compact" type="button" data-modal-open="edit-member-index-{{ .ID }}" title="Edit user" aria-label="Edit user">&#9998;</button>{{ end }}
					</div>
				</div>
			</article>
			{{ else }}
			<p class="empty">No users yet.</p>
			{{ end }}
		</div>
	</section>

	{{ if eq .Dashboard.Household.Role "owner" }}
	<section id="add-member-index" class="modal">
		<div class="modal-card">
			<div class="modal-head">
				<h2>Add User</h2>
				<button class="secondary compact" type="button" data-modal-close>Close</button>
			</div>
			<form method="post" action="/members">
				<input type="hidden" name="return_to" value="/members">
				<div class="form-row">
					<label>Email<input name="email" type="email" required></label>
					<label>Name<input name="name"></label>
				</div>
				<label>Role<select name="role"><option>member</option><option>owner</option></select></label>
				<button type="submit">Add user</button>
			</form>
		</div>
	</section>
	{{ range .Members }}
	<section id="edit-member-index-{{ .ID }}" class="modal">
		<div class="modal-card">
			<div class="modal-head">
				<h2>Edit User</h2>
				<button class="secondary compact" type="button" data-modal-close>Close</button>
			</div>
			<form method="post" action="/members/{{ .ID }}">
				<input type="hidden" name="return_to" value="/members">
				<div class="form-row">
					<label>Email<input name="email" type="email" value="{{ .Email }}" required></label>
					<label>Name<input name="name" value="{{ .Name }}" required></label>
				</div>
				<label>Role<select name="role">
					<option value="member"{{ selectedString .Role "member" }}>member</option>
					<option value="owner"{{ selectedString .Role "owner" }}>owner</option>
				</select></label>
				<button type="submit">Save user</button>
			</form>
			{{ if ne .ID $.Dashboard.CurrentUser.ID }}
			<form method="post" action="/members/{{ .ID }}/remove" style="margin-top:10px;">
				<input type="hidden" name="return_to" value="/members">
				<button class="danger" type="submit">Remove user</button>
			</form>
			{{ end }}
		</div>
	</section>
	{{ end }}
	{{ end }}
</main>
{{ end }}

{{ define "settings" }}
<main class="shell">
	{{ template "pageHeader" (dict "BackURL" "/" "Title" "Settings") }}

	{{ if .Error }}<p class="panel error">{{ .Error }}</p>{{ end }}

	{{ if .CreatedAPIToken.Token }}
	<section class="panel">
		<h2>API Token Created</h2>
		<p class="empty">Copy this token now. It will not be shown again.</p>
		<pre class="code-block">{{ .CreatedAPIToken.Token }}</pre>
	</section>
	{{ end }}

	<section class="panel">
		<div class="tile-head">
			<h2>API Tokens</h2>
		</div>
		<form method="post" action="/settings/api-tokens">
			<div class="form-row">
				<label>Name<input name="name" placeholder="Home Assistant" required></label>
				<label>Access<select name="scope">
					<option value="read">Read-only</option>
					<option value="write">Full access</option>
				</select></label>
			</div>
			<button type="submit">Create token</button>
		</form>
		<div class="cards" style="margin-top:16px;">
			{{ range .APITokens }}
			<article class="item">
				<div class="item-head">
					<div>
						<strong>{{ .Name }}</strong>
						<div class="meta">{{ .Prefix }}... · {{ .Scope }} access · Created {{ datetime .CreatedAt }}</div>
						{{ if .LastUsedAt }}<div class="meta">Last used {{ date .LastUsedAt }}</div>{{ end }}
						{{ if .RevokedAt }}<div class="meta">Revoked {{ date .RevokedAt }}</div>{{ end }}
					</div>
					{{ if not .RevokedAt }}
					<form method="post" action="/settings/api-tokens/{{ .ID }}/revoke">
						<button class="danger compact" type="submit" title="Revoke token" aria-label="Revoke token">X</button>
					</form>
					{{ end }}
				</div>
			</article>
			{{ else }}
			<p class="empty">No API tokens yet.</p>
			{{ end }}
		</div>
	</section>
</main>
{{ end }}

{{ define "documentIndex" }}
<main class="shell">
	{{ template "pageHeader" (dict "BackURL" "/" "Title" "Documents") }}

	{{ if .Error }}<p class="panel error">{{ .Error }}</p>{{ end }}

	<section class="panel" data-filter-scope>
		<div class="tile-head">
			<h2>Household Documents</h2>
			<button class="button compact" type="button" data-modal-open="add-document" title="Add document" aria-label="Add document">+</button>
		</div>
		<div class="form-row filter-row">
			<label>Search<input type="search" data-filter-input placeholder="Search documents"></label>
			<label>Type<select data-filter-kind>
				<option value="">All types</option>
				<option value="general">general</option>
				<option value="manual">manual</option>
				<option value="receipt">receipt</option>
				<option value="warranty">warranty</option>
				<option value="quote">quote</option>
			</select></label>
		</div>
		<p class="empty" data-filter-empty hidden>No matching documents.</p>
		<div class="cards">
			{{ range .Documents }}
			<article class="item" data-filter-item data-filter-kind="{{ .Kind }}" data-filter-text="{{ .Title }} {{ .Description }} {{ .Kind }} {{ .Status }} {{ .FileName }}">
				<div class="item-head">
					<div>
						{{ $doc := . }}
						<strong><a href="/documents/{{ .ID }}">{{ .Title }}</a></strong>
						{{ if .Description }}<div class="meta">{{ .Description }}</div>{{ end }}
						<div class="meta">{{ .Kind }}{{ if .FileName }} · {{ fileSize .FileSize }} · <a href="{{ documentOpenURL $doc }}" target="_blank" rel="noreferrer">{{ documentSourceLabel $doc }}</a>{{ end }}</div>
					</div>
					<span class="badge">{{ .Status }}</span>
				</div>
			</article>
			{{ else }}
			<p class="empty">No documents yet.</p>
			{{ end }}
		</div>
	</section>

	{{ template "documentCreateModal" . }}
</main>
{{ end }}

{{ define "documentDetail" }}
<main class="shell">
	<section class="detail-hero">
		<div class="title-row">
			<div class="title-copy">
				<div class="project-title-line">
					<a class="button secondary back-icon" href="/documents" title="Back" aria-label="Back">‹</a>
					<h1>{{ .Document.Title }}</h1>
					{{ template "titleActionMenu" (dict "EditModal" "edit-document" "ArchiveForm" "document-archive") }}
					<button class="pill active" type="button" data-modal-open="edit-document" title="Edit document type">{{ .Document.Kind }}</button>
				</div>
				<div class="info-strip">
					<div class="info-cell"><span>Description</span>{{ if .Document.Description }}{{ .Document.Description }}{{ else }}No description{{ end }}</div>
					<div class="info-cell"><span>File</span>{{ if .Document.FileName }}<a href="{{ documentOpenURL .Document }}" target="_blank" rel="noreferrer">{{ documentSourceLabel .Document }}</a> · {{ fileSize .Document.FileSize }}{{ else }}No file{{ end }}</div>
					<div class="info-cell"><span>Updated</span>{{ datetime .Document.UpdatedAt }}</div>
				</div>
			</div>
		</div>
	</section>
	<form id="document-archive" method="post" action="/documents/{{ .Document.ID }}/archive"></form>

	{{ if .Error }}<p class="panel error">{{ .Error }}</p>{{ end }}

	<div class="detail-with-sidebar">
		<div class="detail-main">
			<section class="panel">
				<div class="tile-head">
					<h2>Preview</h2>
					{{ if documentOpenURL .Document }}<a class="button secondary compact" href="{{ documentOpenURL .Document }}" target="_blank" rel="noreferrer">Download</a>{{ end }}
				</div>
				{{ if documentOpenURL .Document }}
				<iframe class="document-preview document-detail-preview" src="{{ documentOpenURL .Document }}" title="{{ .Document.Title }}"></iframe>
				{{ else }}
				<div class="document-preview-empty document-detail-preview">No file preview available.</div>
				{{ end }}
			</section>
		</div>

		<aside class="panel detail-sidebar">
			<section class="info-panel-section">
				<div class="tile-head">
					<h2>Related Items</h2>
					<button class="button compact" type="button" data-modal-open="link-related-item">+</button>
				</div>
				<div class="related-list">
					{{ range .RelatedItems }}
					<article class="related-row">
						<div>
							<strong><a href="{{ .URL }}">{{ .Title }}</a></strong>
							<div class="meta">{{ .Type }}{{ if .Subtitle }} · {{ .Subtitle }}{{ end }}</div>
						</div>
						<form method="post" action="/documents/{{ $.Document.ID }}/related-items/{{ .LinkID }}/remove"><button class="danger compact remove-button" type="submit" title="Remove" aria-label="Remove">X</button></form>
					</article>
					{{ else }}
					<p class="empty">No related items yet.</p>
					{{ end }}
				</div>
			</section>
		</aside>
	</div>

	<section id="edit-document" class="modal">
		<div class="modal-card">
			<div class="modal-head">
				<h2>Edit Document</h2>
				<button class="secondary compact" type="button" data-modal-close>Close</button>
			</div>
			<form method="post" action="/documents/{{ .Document.ID }}" enctype="multipart/form-data">
				<label>Title<input name="title" value="{{ .Document.Title }}" required></label>
				<label>Description<textarea name="description">{{ .Document.Description }}</textarea></label>
				<label>File<input name="file" type="file"></label>
				<div class="form-row">
					<label>Type<select name="kind">
						<option value="general" {{ selectedString .Document.Kind "general" }}>general</option>
						<option value="manual" {{ selectedString .Document.Kind "manual" }}>manual</option>
						<option value="receipt" {{ selectedString .Document.Kind "receipt" }}>receipt</option>
						<option value="warranty" {{ selectedString .Document.Kind "warranty" }}>warranty</option>
						<option value="quote" {{ selectedString .Document.Kind "quote" }}>quote</option>
					</select></label>
					<label>Status<select name="status">
						<option value="active" {{ selectedString .Document.Status "active" }}>active</option>
						<option value="archived" {{ selectedString .Document.Status "archived" }}>archived</option>
					</select></label>
				</div>
				<button type="submit">Save document</button>
			</form>
		</div>
	</section>

	<section id="link-related-item" class="modal">
		<div class="modal-card wide">
			<div class="modal-head">
				<h2>Link Related Item</h2>
				<button class="secondary compact" type="button" data-modal-close>Close</button>
			</div>
			<div data-filter-scope>
			<div class="search-panel">
				<label>Search<input type="search" data-filter-input placeholder="Search projects, tasks, and assets"></label>
				<p class="empty" data-filter-empty hidden>No matching items.</p>
			</div>
			<div class="modal-columns">
				<form method="post" action="/documents/{{ .Document.ID }}/related-items">
					<h3>Projects</h3>
					<input type="hidden" name="entity_type" value="project">
					<div class="search-list">
						{{ range .Projects }}
						<button class="secondary search-choice" type="submit" name="entity_id" value="{{ .ID }}" data-filter-item data-filter-text="{{ .Title }} {{ .Description }} {{ .Status }}">
							<strong>{{ .Title }}</strong>
							<span class="meta">{{ .Status }} project</span>
						</button>
						{{ else }}
						<p class="empty">No projects.</p>
						{{ end }}
					</div>
				</form>
				<form method="post" action="/documents/{{ .Document.ID }}/related-items">
					<h3>Tasks</h3>
					<input type="hidden" name="entity_type" value="task">
					<div class="search-list">
						{{ range .Tasks }}
						<button class="secondary search-choice" type="submit" name="entity_id" value="{{ .ID }}" data-filter-item data-filter-text="{{ .Title }} {{ .Notes }} {{ .AssignedName }} {{ .Status }}">
							<strong>{{ .Title }}</strong>
							<span class="meta">{{ .Status }} task{{ if .AssignedName }} · {{ .AssignedName }}{{ end }}</span>
						</button>
						{{ else }}
						<p class="empty">No tasks.</p>
						{{ end }}
					</div>
				</form>
				<form method="post" action="/documents/{{ .Document.ID }}/related-items">
					<h3>Assets</h3>
					<input type="hidden" name="entity_type" value="asset">
					<div class="search-list">
						{{ range .Assets }}
						<button class="secondary search-choice" type="submit" name="entity_id" value="{{ .ID }}" data-filter-item data-filter-text="{{ .Name }} {{ .Kind }} {{ .SerialNumber }} {{ .Notes }} {{ .Status }}">
							<strong>{{ .Name }}</strong>
							<span class="meta">{{ .Kind }} asset{{ if .SerialNumber }} · {{ .SerialNumber }}{{ end }}</span>
						</button>
						{{ else }}
						<p class="empty">No assets.</p>
						{{ end }}
					</div>
				</form>
			</div>
			</div>
		</div>
	</section>
</main>
{{ end }}

{{ define "documentCreateModal" }}
<section id="add-document" class="modal">
	<div class="modal-card">
		<div class="modal-head">
			<h2>Add Document</h2>
			<button class="secondary compact" type="button" data-modal-close>Close</button>
		</div>
		<form method="post" action="/documents" enctype="multipart/form-data">
			<label>Title<input name="title" required></label>
			<label>Description<textarea name="description"></textarea></label>
			<label>File<input name="file" type="file" required></label>
			<label>Type<select name="kind">
				<option value="general">general</option>
				<option value="manual">manual</option>
				<option value="receipt">receipt</option>
				<option value="warranty">warranty</option>
				<option value="quote">quote</option>
			</select></label>
			<button type="submit">Add document</button>
		</form>
	</div>
</section>
{{ end }}

{{ define "relatedDocumentsList" }}
<div class="related-list">
	{{ range .Docs }}
	<article class="related-row">
		<div>
			{{ $doc := .Document }}
			<strong><button class="link-button" type="button" data-modal-open="related-document-{{ .LinkID }}">{{ .Document.Title }}</button></strong>
			{{ if .Document.Description }}<div class="meta">{{ .Document.Description }}</div>{{ end }}
			<div class="meta">{{ .Document.Kind }}{{ if .Document.FileName }} · {{ fileSize .Document.FileSize }} · {{ documentSourceLabel $doc }}{{ end }}</div>
		</div>
		<form method="post" action="{{ $.RemovePrefix }}/{{ .LinkID }}/remove"><button class="danger compact remove-button" type="submit" title="Remove" aria-label="Remove">X</button></form>
	</article>
	{{ template "documentInfoModal" (dictDocumentInfo (printf "related-document-%d" .LinkID) .Document) }}
	{{ else }}
	<p class="empty">No related documents yet.</p>
	{{ end }}
</div>
{{ end }}

{{ define "documentInfoModal" }}
{{ $doc := .Document }}
<section id="{{ .ID }}" class="modal">
	<div class="modal-card preview">
		<div class="modal-head">
			<h2>{{ $doc.Title }}</h2>
			<button class="secondary compact" type="button" data-modal-close>Close</button>
		</div>
		<div class="item document-preview-meta">
			<div class="meta">{{ $doc.Kind }}{{ if $doc.FileName }} · {{ fileSize $doc.FileSize }} · {{ $doc.FileName }}{{ end }}</div>
			{{ if $doc.Description }}<div class="meta">{{ $doc.Description }}</div>{{ end }}
		</div>
		{{ if documentOpenURL $doc }}
		<iframe class="document-preview" src="{{ documentOpenURL $doc }}" title="{{ $doc.Title }}"></iframe>
		{{ else }}
		<div class="document-preview-empty">No file preview available.</div>
		{{ end }}
		<div class="detail-actions">
			<a class="button secondary" href="/documents/{{ $doc.ID }}">Details</a>
			{{ if documentOpenURL $doc }}<a class="button secondary" href="{{ documentOpenURL $doc }}" target="_blank" rel="noreferrer">Download</a>{{ end }}
		</div>
	</div>
</section>
{{ end }}

{{ define "relatedContactsList" }}
<div class="related-list">
	{{ range .Contacts }}
	<article class="related-row">
		<div>
			<strong><button class="link-button" type="button" data-modal-open="related-contact-{{ .LinkID }}">{{ .Contact.Name }}</button></strong>
			<div class="meta">{{ .Contact.Kind }}{{ if .Contact.Email }} · <a href="mailto:{{ .Contact.Email }}">{{ .Contact.Email }}</a>{{ end }}{{ if .Contact.Phone }} · <a href="tel:{{ .Contact.Phone }}">{{ .Contact.Phone }}</a>{{ end }}</div>
			{{ if .Contact.Notes }}<div class="meta">{{ .Contact.Notes }}</div>{{ end }}
		</div>
		<form method="post" action="{{ $.RemovePrefix }}/{{ .LinkID }}/remove"><button class="danger compact remove-button" type="submit" title="Remove" aria-label="Remove">X</button></form>
	</article>
	{{ template "contactInfoModal" (dictContactInfo (printf "related-contact-%d" .LinkID) .Contact) }}
	{{ else }}
	<p class="empty">No related contacts yet.</p>
	{{ end }}
</div>
{{ end }}

{{ define "contactInfoModal" }}
<section id="{{ .ID }}" class="modal">
	<div class="modal-card">
		<div class="modal-head">
			<h2>{{ .Contact.Name }}</h2>
			<button class="secondary compact" type="button" data-modal-close>Close</button>
		</div>
		<div class="cards">
			<article class="item">
				<div class="meta">{{ .Contact.Kind }}</div>
				{{ if .Contact.Email }}<div><strong>Email</strong><div class="meta"><a href="mailto:{{ .Contact.Email }}">{{ .Contact.Email }}</a></div></div>{{ end }}
				{{ if .Contact.Phone }}<div><strong>Phone</strong><div class="meta"><a href="tel:{{ .Contact.Phone }}">{{ .Contact.Phone }}</a></div></div>{{ end }}
				{{ if .Contact.Notes }}<div><strong>Notes</strong><div class="meta">{{ .Contact.Notes }}</div></div>{{ end }}
			</article>
		</div>
		<div class="detail-actions" style="margin-top:12px;">
			<a class="button secondary" href="/contacts/{{ .Contact.ID }}">Open contact</a>
		</div>
	</div>
</section>
{{ end }}

{{ define "relatedAssetsList" }}
<div class="related-list">
	{{ range .Assets }}
	<article class="related-row">
		<div>
			<strong><button class="link-button" type="button" data-modal-open="related-asset-{{ .LinkID }}">{{ .Asset.Name }}</button></strong>
			<div class="meta">{{ .Asset.Kind }}{{ if .Asset.Model }} · {{ .Asset.Model }}{{ end }}{{ if .Asset.Vendor }} · {{ .Asset.Vendor }}{{ end }}{{ if .Asset.SerialNumber }} · {{ .Asset.SerialNumber }}{{ end }}{{ if .Asset.WarrantyExpiresAt }} · Warranty {{ date .Asset.WarrantyExpiresAt }}{{ end }}</div>
			{{ if .Asset.Notes }}<div class="meta">{{ .Asset.Notes }}</div>{{ end }}
		</div>
		<form method="post" action="{{ $.RemovePrefix }}/{{ .LinkID }}/remove"><button class="danger compact remove-button" type="submit" title="Remove" aria-label="Remove">X</button></form>
	</article>
	{{ template "assetInfoModal" (dictAssetInfo (printf "related-asset-%d" .LinkID) .Asset) }}
	{{ else }}
	<p class="empty">No related assets yet.</p>
	{{ end }}
</div>
{{ end }}

{{ define "assetInfoModal" }}
<section id="{{ .ID }}" class="modal">
	<div class="modal-card">
		<div class="modal-head">
			<h2>{{ .Asset.Name }}</h2>
			<button class="secondary compact" type="button" data-modal-close>Close</button>
		</div>
		<div class="cards">
			<article class="item">
				<div class="meta">{{ .Asset.Kind }}</div>
				{{ if .Asset.Model }}<div><strong>Model</strong><div class="meta">{{ .Asset.Model }}</div></div>{{ end }}
				{{ if .Asset.Vendor }}<div><strong>Vendor</strong><div class="meta">{{ .Asset.Vendor }}</div></div>{{ end }}
				{{ if .Asset.SerialNumber }}<div><strong>Serial</strong><div class="meta">{{ .Asset.SerialNumber }}</div></div>{{ end }}
				{{ if .Asset.PurchaseDate }}<div><strong>Purchased</strong><div class="meta">{{ date .Asset.PurchaseDate }}</div></div>{{ end }}
				{{ if .Asset.PurchaseCost }}<div><strong>Cost</strong><div class="meta">${{ money .Asset.PurchaseCost }}</div></div>{{ end }}
				{{ if .Asset.WarrantyExpiresAt }}<div><strong>Warranty</strong><div class="meta">{{ date .Asset.WarrantyExpiresAt }}</div></div>{{ end }}
				{{ if .Asset.MaintenanceCadence }}<div><strong>Maintenance</strong><div class="meta">{{ .Asset.MaintenanceCadence }}{{ if .Asset.MaintenanceNextDueAt }} · Due {{ date .Asset.MaintenanceNextDueAt }}{{ end }}</div></div>{{ end }}
				{{ if .Asset.Notes }}<div><strong>Notes</strong><div class="meta">{{ .Asset.Notes }}</div></div>{{ end }}
			</article>
		</div>
		<div class="detail-actions" style="margin-top:12px;">
			<a class="button secondary" href="/assets/{{ .Asset.ID }}">Details</a>
		</div>
	</div>
</section>
{{ end }}

{{ define "attachContactModal" }}
<section id="{{ .ID }}" class="modal">
	<div class="modal-card wide">
		<div class="modal-head">
			<h2>Attach Contact</h2>
			<button class="secondary compact" type="button" data-modal-close>Close</button>
		</div>
		<div class="modal-columns">
			<form method="post" action="{{ .Action }}" data-filter-scope>
				<h3>Existing</h3>
				<label>Search<input type="search" data-filter-input placeholder="Search contacts"></label>
				<p class="empty" data-filter-empty hidden>No matching contacts.</p>
				<div class="search-list">
					{{ range .Contacts }}
					<button class="secondary search-choice" type="submit" name="contact_id" value="{{ .ID }}" data-filter-item data-filter-text="{{ .Name }} {{ .Kind }} {{ .Email }} {{ .Phone }} {{ .Notes }}">
						<strong>{{ .Name }}</strong>
						<span class="meta">{{ .Kind }}{{ if .Email }} · {{ .Email }}{{ end }}{{ if .Phone }} · {{ .Phone }}{{ end }}</span>
					</button>
					{{ else }}
					<p class="empty">No contacts yet.</p>
					{{ end }}
				</div>
			</form>
			<form method="post" action="{{ .Action }}">
				<h3>Create New</h3>
				<label>Name<input name="name" required></label>
				<label>Type<select name="kind">
					<option value="general">general</option>
					<option value="family">family</option>
					<option value="contractor">contractor</option>
					<option value="service">service</option>
					<option value="medical">medical</option>
					<option value="emergency">emergency</option>
				</select></label>
				<div class="form-row">
					<label>Email<input name="email" type="email"></label>
					<label>Phone<input name="phone" type="tel"></label>
				</div>
				<label>Notes<textarea name="notes"></textarea></label>
				<button type="submit">Create and link</button>
			</form>
		</div>
	</div>
</section>
{{ end }}

{{ define "attachAssetModal" }}
<section id="{{ .ID }}" class="modal">
	<div class="modal-card wide">
		<div class="modal-head">
			<h2>Attach Asset</h2>
			<button class="secondary compact" type="button" data-modal-close>Close</button>
		</div>
		<div class="modal-columns">
			<form method="post" action="{{ .Action }}" data-filter-scope>
				<h3>Existing</h3>
				<label>Search<input type="search" data-filter-input placeholder="Search assets"></label>
				<p class="empty" data-filter-empty hidden>No matching assets.</p>
				<div class="search-list">
					{{ range .Assets }}
					<button class="secondary search-choice" type="submit" name="asset_id" value="{{ .ID }}" data-filter-item data-filter-text="{{ .Name }} {{ .Kind }} {{ .SerialNumber }} {{ .Vendor }} {{ .Model }} {{ .Notes }}">
						<strong>{{ .Name }}</strong>
						<span class="meta">{{ .Kind }}{{ if .Model }} · {{ .Model }}{{ end }}{{ if .Vendor }} · {{ .Vendor }}{{ end }}{{ if .SerialNumber }} · {{ .SerialNumber }}{{ end }}</span>
					</button>
					{{ else }}
					<p class="empty">No assets yet.</p>
					{{ end }}
				</div>
			</form>
			<form method="post" action="{{ .Action }}">
				<h3>Create New</h3>
				{{ template "assetFields" .Asset }}
				<button type="submit">Create and link</button>
			</form>
		</div>
	</div>
</section>
{{ end }}

{{ define "attachDocumentModal" }}
<section id="{{ .ID }}" class="modal">
	<div class="modal-card wide">
		<div class="modal-head">
			<h2>Attach Document</h2>
			<button class="secondary compact" type="button" data-modal-close>Close</button>
		</div>
		<div class="modal-columns">
			<form method="post" action="{{ .Action }}" data-filter-scope>
				<h3>Existing</h3>
				<label>Search<input type="search" data-filter-input placeholder="Search documents"></label>
				<p class="empty" data-filter-empty hidden>No matching documents.</p>
				<div class="search-list">
					{{ range .Documents }}
					<button class="secondary search-choice" type="submit" name="document_id" value="{{ .ID }}" data-filter-item data-filter-text="{{ .Title }} {{ .Description }} {{ .Kind }}">
						<strong>{{ .Title }}</strong>
						<span class="meta">{{ .Kind }}{{ if .Description }} · {{ .Description }}{{ end }}</span>
					</button>
					{{ else }}
					<p class="empty">No documents yet.</p>
					{{ end }}
				</div>
			</form>
			<form method="post" action="{{ .Action }}" enctype="multipart/form-data">
				<h3>Create New</h3>
				<label>Title<input name="title" required></label>
				<label>Description<textarea name="description"></textarea></label>
				<label>File<input name="file" type="file" required></label>
				<label>Type<select name="kind">
					<option value="general">general</option>
					<option value="manual">manual</option>
					<option value="receipt">receipt</option>
					<option value="warranty">warranty</option>
					<option value="quote">quote</option>
				</select></label>
				<button type="submit">Create and link</button>
			</form>
		</div>
	</div>
</section>
{{ end }}

{{ define "listIndex" }}
<main class="shell">
	{{ template "pageHeader" (dict "BackURL" "/" "Title" "Lists") }}

	{{ if .Error }}<p class="panel error">{{ .Error }}</p>{{ end }}

	<section class="panel">
		<div class="tile-head">
			<h2>Household Lists</h2>
			<button class="button compact" type="button" data-modal-open="add-list" title="Add list" aria-label="Add list">+</button>
		</div>
		<div class="cards">
			{{ range .Lists }}
			<article class="item">
				<div class="item-head">
					<div>
						<strong><a href="/lists/{{ .ID }}">{{ .Title }}</a></strong>
						{{ if .Description }}<div class="meta">{{ .Description }}</div>{{ end }}
						<div class="meta">{{ .Kind }}</div>
					</div>
					<span class="badge">{{ .Status }}</span>
				</div>
			</article>
			{{ else }}
			<p class="empty">No lists yet.</p>
			{{ end }}
		</div>
	</section>

	<section id="add-list" class="modal">
		<div class="modal-card">
			<div class="modal-head">
				<h2>Add List</h2>
				<button class="secondary compact" type="button" data-modal-close>Close</button>
			</div>
			<form method="post" action="/lists">
				<label>Title<input name="title" required></label>
				<label>Description<textarea name="description"></textarea></label>
				<label>Type<select name="kind">
					<option value="general">general</option>
					<option value="grocery">grocery</option>
					<option value="shopping">shopping</option>
					<option value="packing">packing</option>
				</select></label>
				<button type="submit">Add list</button>
			</form>
		</div>
	</section>
</main>
{{ end }}

{{ define "contactIndex" }}
<main class="shell">
	{{ template "pageHeader" (dict "BackURL" "/" "Title" "Contacts") }}

	{{ if .Error }}<p class="panel error">{{ .Error }}</p>{{ end }}

	<section class="panel" data-filter-scope>
		<div class="tile-head">
			<h2>Household Contacts</h2>
			<button class="button compact" type="button" data-modal-open="add-contact" title="Add contact" aria-label="Add contact">+</button>
		</div>
		<div class="form-row filter-row">
			<label>Search<input type="search" data-filter-input placeholder="Search contacts"></label>
			<label>Type<select data-filter-kind>
				<option value="">All types</option>
				<option value="general">general</option>
				<option value="family">family</option>
				<option value="contractor">contractor</option>
				<option value="service">service</option>
				<option value="medical">medical</option>
				<option value="emergency">emergency</option>
			</select></label>
		</div>
		<p class="empty" data-filter-empty hidden>No matching contacts.</p>
		<div class="cards">
			{{ range .Contacts }}
			<article class="item" data-filter-item data-filter-kind="{{ .Kind }}" data-filter-text="{{ .Name }} {{ .Kind }} {{ .Email }} {{ .Phone }} {{ .Notes }} {{ .Status }}">
				<div class="item-head">
					<div>
						<strong><a href="/contacts/{{ .ID }}">{{ .Name }}</a></strong>
						<div class="meta">{{ .Kind }}{{ if .Email }} · <a href="mailto:{{ .Email }}">{{ .Email }}</a>{{ end }}{{ if .Phone }} · <a href="tel:{{ .Phone }}">{{ .Phone }}</a>{{ end }}</div>
						{{ if .Notes }}<div class="meta">{{ .Notes }}</div>{{ end }}
					</div>
					<span class="badge">{{ .Status }}</span>
				</div>
			</article>
			{{ else }}
			<p class="empty">No contacts yet.</p>
			{{ end }}
		</div>
	</section>

	<section id="add-contact" class="modal">
		<div class="modal-card">
			<div class="modal-head">
				<h2>Add Contact</h2>
				<button class="secondary compact" type="button" data-modal-close>Close</button>
			</div>
			<form method="post" action="/contacts">
				{{ template "contactFields" .Contact }}
				<button type="submit">Add contact</button>
			</form>
		</div>
	</section>
</main>
{{ end }}

{{ define "contactDetail" }}
<main class="shell">
	<section class="detail-hero">
		<div class="title-row">
			<div class="title-copy">
				<div class="project-title-line">
					<a class="button secondary back-icon" href="/contacts" title="Back" aria-label="Back">‹</a>
					<h1>{{ .Contact.Name }}</h1>
					{{ template "titleActionMenu" (dict "EditModal" "edit-contact" "ArchiveForm" "contact-archive") }}
					<button class="pill active" type="button" data-modal-open="edit-contact" title="Edit contact type">{{ .Contact.Kind }}</button>
				</div>
				<div class="info-strip">
					<div class="info-cell"><span>Email</span>{{ if .Contact.Email }}<a href="mailto:{{ .Contact.Email }}">{{ .Contact.Email }}</a>{{ else }}No email{{ end }}</div>
					<div class="info-cell"><span>Phone</span>{{ if .Contact.Phone }}<a href="tel:{{ .Contact.Phone }}">{{ .Contact.Phone }}</a>{{ else }}No phone{{ end }}</div>
					<div class="info-cell"><span>Updated</span>{{ datetime .Contact.UpdatedAt }}</div>
				</div>
				{{ if .Contact.Notes }}<p>{{ .Contact.Notes }}</p>{{ end }}
			</div>
		</div>
	</section>
	<form id="contact-archive" method="post" action="/contacts/{{ .Contact.ID }}/archive"></form>

	{{ if .Error }}<p class="panel error">{{ .Error }}</p>{{ end }}

	<section id="edit-contact" class="modal">
		<div class="modal-card">
			<div class="modal-head">
				<h2>Edit Contact</h2>
				<button class="secondary compact" type="button" data-modal-close>Close</button>
			</div>
			<form method="post" action="/contacts/{{ .Contact.ID }}">
				{{ template "contactFields" .Contact }}
				<button type="submit">Save contact</button>
			</form>
		</div>
	</section>
</main>
{{ end }}

{{ define "contactFields" }}
<label>Name<input name="name" value="{{ .Name }}" required></label>
<div class="form-row">
	<label>Type<select name="kind">
		<option value="general" {{ selectedString .Kind "general" }}>general</option>
		<option value="family" {{ selectedString .Kind "family" }}>family</option>
		<option value="contractor" {{ selectedString .Kind "contractor" }}>contractor</option>
		<option value="service" {{ selectedString .Kind "service" }}>service</option>
		<option value="medical" {{ selectedString .Kind "medical" }}>medical</option>
		<option value="emergency" {{ selectedString .Kind "emergency" }}>emergency</option>
	</select></label>
	<label>Status<select name="status">
		<option value="active" {{ selectedString .Status "active" }}>active</option>
		<option value="archived" {{ selectedString .Status "archived" }}>archived</option>
	</select></label>
</div>
<div class="form-row">
	<label>Email<input name="email" type="email" value="{{ .Email }}"></label>
	<label>Phone<input name="phone" type="tel" value="{{ .Phone }}"></label>
</div>
<label>Notes<textarea name="notes">{{ .Notes }}</textarea></label>
{{ end }}

{{ define "assetIndex" }}
<main class="shell">
	{{ template "pageHeader" (dict "BackURL" "/" "Title" "Assets") }}

	{{ if .Error }}<p class="panel error">{{ .Error }}</p>{{ end }}

	<section class="panel" data-filter-scope>
		<div class="tile-head">
			<h2>Household Assets</h2>
			<button class="button compact" type="button" data-modal-open="add-asset" title="Add asset" aria-label="Add asset">+</button>
		</div>
		<div class="form-row filter-row">
			<label>Search<input type="search" data-filter-input placeholder="Search assets"></label>
			<label>Type<select data-filter-kind>
				<option value="">All types</option>
				<option value="general">general</option>
				<option value="appliance">appliance</option>
				<option value="electronics">electronics</option>
				<option value="vehicle">vehicle</option>
				<option value="tool">tool</option>
				<option value="furniture">furniture</option>
			</select></label>
		</div>
		<p class="empty" data-filter-empty hidden>No matching assets.</p>
		<div class="cards">
			{{ range .Assets }}
			<article class="item" data-filter-item data-filter-kind="{{ .Kind }}" data-filter-text="{{ .Name }} {{ .Kind }} {{ .SerialNumber }} {{ .Vendor }} {{ .Model }} {{ .Notes }} {{ .Status }}">
				<div class="item-head">
					<div>
						<strong><a href="/assets/{{ .ID }}">{{ .Name }}</a></strong>
						<div class="meta">{{ .Kind }}{{ if .Model }} · {{ .Model }}{{ end }}{{ if .Vendor }} · {{ .Vendor }}{{ end }}{{ if .SerialNumber }} · {{ .SerialNumber }}{{ end }}{{ if .WarrantyExpiresAt }} · Warranty {{ date .WarrantyExpiresAt }}{{ end }}</div>
						{{ if .Notes }}<div class="meta">{{ .Notes }}</div>{{ end }}
					</div>
					<span class="badge">{{ .Status }}</span>
				</div>
			</article>
			{{ else }}
			<p class="empty">No assets yet.</p>
			{{ end }}
		</div>
	</section>

	<section id="add-asset" class="modal">
		<div class="modal-card">
			<div class="modal-head">
				<h2>Add Asset</h2>
				<button class="secondary compact" type="button" data-modal-close>Close</button>
			</div>
			<form method="post" action="/assets">
				{{ template "assetFields" .Asset }}
				<button type="submit">Add asset</button>
			</form>
		</div>
	</section>
</main>
{{ end }}

{{ define "assetDetail" }}
<main class="shell">
	<section class="detail-hero">
		<div class="title-row">
			<div class="title-copy">
				<div class="project-title-line">
					<a class="button secondary back-icon" href="/assets" title="Back" aria-label="Back">‹</a>
					<h1>{{ .Asset.Name }}</h1>
					{{ template "titleActionMenu" (dict "EditModal" "edit-asset" "ArchiveForm" "asset-archive") }}
					<button class="pill active" type="button" data-modal-open="edit-asset" title="Edit asset type">{{ .Asset.Kind }}</button>
				</div>
				{{ if .Asset.Notes }}<p>{{ .Asset.Notes }}</p>{{ end }}
			</div>
		</div>
	</section>
	<form id="asset-archive" method="post" action="/assets/{{ .Asset.ID }}/archive"></form>

	{{ if .Error }}<p class="panel error">{{ .Error }}</p>{{ end }}

	<div class="detail-with-sidebar">
	<div class="detail-main">
	<section class="info-strip asset-summary">
		<div class="info-cell"><span>Serial</span>{{ if .Asset.SerialNumber }}{{ .Asset.SerialNumber }}{{ else }}No serial number{{ end }}</div>
		<div class="info-cell"><span>Model</span>{{ if .Asset.Model }}{{ .Asset.Model }}{{ else }}No model{{ end }}</div>
		<div class="info-cell"><span>Vendor</span>{{ if .Asset.Vendor }}{{ .Asset.Vendor }}{{ else }}No vendor{{ end }}</div>
		<div class="info-cell"><span>Purchased</span>{{ if .Asset.PurchaseDate }}{{ date .Asset.PurchaseDate }}{{ else }}No purchase date{{ end }}</div>
		<div class="info-cell"><span>Cost</span>{{ if .Asset.PurchaseCost }}${{ money .Asset.PurchaseCost }}{{ else }}No cost{{ end }}</div>
		<div class="info-cell"><span>Warranty</span>{{ if .Asset.WarrantyExpiresAt }}{{ date .Asset.WarrantyExpiresAt }}{{ else }}No warranty date{{ end }}</div>
		<div class="info-cell"><span>Updated</span>{{ datetime .Asset.UpdatedAt }}</div>
	</section>

	<section class="panel">
		<div class="tile-head">
			<h2>Tasks</h2>
		</div>
		<div class="cards">
			{{ range assetTasks .Tasks .Asset.ID }}
				{{ template "taskItem" . }}
			{{ else }}
				<p class="empty">No generated maintenance tasks yet.</p>
			{{ end }}
		</div>
	</section>
	</div>

	<aside class="panel detail-sidebar">
		<section class="info-panel-section">
			<div class="tile-head">
				<h2>Maintenance</h2>
				<button class="button compact" type="button" data-modal-open="add-asset-maintenance">+</button>
			</div>
			<div class="maintenance-list">
				{{ range .AssetMaintenanceItems }}
				<article class="item">
					<div class="item-head">
						<div>
							<strong>{{ .Title }}</strong>
							<div class="maintenance-meta">
								<span class="badge">{{ .Cadence }}</span>
								<span class="badge">{{ if .NextDueAt }}Due {{ date .NextDueAt }}{{ else }}No due date{{ end }}</span>
								{{ if .LastCompletedAt }}<span class="badge">Last done {{ date .LastCompletedAt }}</span>{{ end }}
							</div>
							{{ if .Notes }}<div class="meta">{{ .Notes }}</div>{{ end }}
						</div>
						<div class="tile-actions">
							<form method="post" action="/assets/{{ $.Asset.ID }}/maintenance/{{ .ID }}/generate-task"><button class="secondary compact" type="submit">Generate task</button></form>
							<button class="secondary compact" type="button" data-modal-open="edit-asset-maintenance-{{ .ID }}" title="Edit maintenance" aria-label="Edit maintenance">&#9998;</button>
							<form method="post" action="/assets/{{ $.Asset.ID }}/maintenance/{{ .ID }}/archive"><button class="danger compact" type="submit" title="Archive maintenance" aria-label="Archive maintenance">X</button></form>
						</div>
					</div>
				</article>
			{{ else }}
				<p class="empty">No maintenance schedules yet.</p>
			{{ end }}
			</div>
		</section>

		<section class="info-panel-section">
			<div class="tile-head">
				<h2>Documents</h2>
				<button class="button compact" type="button" data-modal-open="attach-asset-document">+</button>
			</div>
			{{ template "relatedDocumentsList" (dictRelatedPrefix .RelatedDocs (printf "/assets/%d/documents" .Asset.ID)) }}
		</section>

		<section class="info-panel-section">
			<div class="tile-head">
				<h2>Contacts</h2>
				<button class="button compact" type="button" data-modal-open="attach-asset-contact">+</button>
			</div>
			{{ template "relatedContactsList" (dictRelatedContacts .RelatedContacts (printf "/assets/%d/contacts" .Asset.ID)) }}
		</section>
	</aside>
	</div>

	{{ template "attachDocumentModal" (dictAttach "attach-asset-document" (printf "/assets/%d/documents" .Asset.ID) .Documents) }}
	{{ template "attachContactModal" (dictAttachContact "attach-asset-contact" (printf "/assets/%d/contacts" .Asset.ID) .Contacts) }}

	<section id="add-asset-maintenance" class="modal">
		<div class="modal-card">
			<div class="modal-head">
				<h2>Add Maintenance</h2>
				<button class="secondary compact" type="button" data-modal-close>Close</button>
			</div>
			<form method="post" action="/assets/{{ .Asset.ID }}/maintenance">
				<label>Title<input name="title" required></label>
				<label>Notes<textarea name="notes"></textarea></label>
				<div class="form-row">
					<label>Cadence<select name="cadence">
						<option value="daily">daily</option>
						<option value="weekly">weekly</option>
						<option value="monthly" selected>monthly</option>
						<option value="quarterly">quarterly</option>
						<option value="yearly">yearly</option>
					</select></label>
					<label>Next due<input name="next_due_at" type="date"></label>
				</div>
				<button type="submit">Add maintenance</button>
			</form>
		</div>
	</section>

	{{ range .AssetMaintenanceItems }}
	<section id="edit-asset-maintenance-{{ .ID }}" class="modal">
		<div class="modal-card">
			<div class="modal-head">
				<h2>Edit Maintenance</h2>
				<button class="secondary compact" type="button" data-modal-close>Close</button>
			</div>
			<form method="post" action="/assets/{{ $.Asset.ID }}/maintenance/{{ .ID }}">
				{{ template "assetMaintenanceFields" . }}
				<button type="submit">Save maintenance</button>
			</form>
		</div>
	</section>
	{{ end }}

	<section id="edit-asset" class="modal">
		<div class="modal-card">
			<div class="modal-head">
				<h2>Edit Asset</h2>
				<button class="secondary compact" type="button" data-modal-close>Close</button>
			</div>
			<form method="post" action="/assets/{{ .Asset.ID }}">
				{{ template "assetFields" .Asset }}
				<button type="submit">Save asset</button>
			</form>
		</div>
	</section>
</main>
{{ end }}

{{ define "assetFields" }}
<label>Name<input name="name" value="{{ .Name }}" required></label>
<div class="form-row">
	<label>Type<select name="kind">
		<option value="general" {{ selectedString .Kind "general" }}>general</option>
		<option value="appliance" {{ selectedString .Kind "appliance" }}>appliance</option>
		<option value="electronics" {{ selectedString .Kind "electronics" }}>electronics</option>
		<option value="vehicle" {{ selectedString .Kind "vehicle" }}>vehicle</option>
		<option value="tool" {{ selectedString .Kind "tool" }}>tool</option>
		<option value="furniture" {{ selectedString .Kind "furniture" }}>furniture</option>
	</select></label>
	<label>Status<select name="status">
		<option value="active" {{ selectedString .Status "active" }}>active</option>
		<option value="archived" {{ selectedString .Status "archived" }}>archived</option>
	</select></label>
</div>
<div class="form-row">
	<label>Serial number<input name="serial_number" value="{{ .SerialNumber }}"></label>
	<label>Warranty expires<input name="warranty_expires_at" type="date" value="{{ dateInput .WarrantyExpiresAt }}"></label>
</div>
<div class="form-row">
	<label>Model<input name="model" value="{{ .Model }}"></label>
	<label>Vendor<input name="vendor" value="{{ .Vendor }}"></label>
</div>
<div class="form-row">
	<label>Purchase date<input name="purchase_date" type="date" value="{{ dateInput .PurchaseDate }}"></label>
	<label>Cost<input name="purchase_cost" type="number" min="0" step="0.01" value="{{ money .PurchaseCost }}"></label>
</div>
<label>Notes<textarea name="notes">{{ .Notes }}</textarea></label>
{{ end }}

{{ define "assetMaintenanceFields" }}
<label>Title<input name="title" value="{{ .Title }}" required></label>
<label>Notes<textarea name="notes">{{ .Notes }}</textarea></label>
<div class="form-row">
	<label>Cadence<select name="cadence">
		<option value="daily" {{ selectedString .Cadence "daily" }}>daily</option>
		<option value="weekly" {{ selectedString .Cadence "weekly" }}>weekly</option>
		<option value="monthly" {{ selectedString .Cadence "monthly" }}>monthly</option>
		<option value="quarterly" {{ selectedString .Cadence "quarterly" }}>quarterly</option>
		<option value="yearly" {{ selectedString .Cadence "yearly" }}>yearly</option>
	</select></label>
	<label>Next due<input name="next_due_at" type="date" value="{{ dateInput .NextDueAt }}"></label>
</div>
<label>Status<select name="status">
	<option value="active" {{ selectedString .Status "active" }}>active</option>
	<option value="archived" {{ selectedString .Status "archived" }}>archived</option>
</select></label>
{{ end }}

{{ define "listDetail" }}
{{ $openItems := openListItemCount .ListItems }}
{{ $doneItems := doneListItemCount .ListItems }}
{{ $totalItems := len .ListItems }}
<main class="shell">
	<section class="detail-hero">
		<div class="title-row">
			<div class="title-copy">
				<div class="project-title-line">
					<a class="button secondary back-icon" href="/lists" title="Back" aria-label="Back">‹</a>
					<h1>{{ .List.Title }}</h1>
					{{ template "titleActionMenu" (dict "EditModal" "edit-list" "ArchiveForm" "list-archive") }}
					<button class="pill active" type="button" data-modal-open="edit-list" title="Edit list type">{{ .List.Kind }}</button>
				</div>
				<div class="summary-strip">
					<span class="badge">{{ $openItems }} open</span>
					<span class="badge">{{ $doneItems }} done</span>
					<span class="badge">{{ $totalItems }} total</span>
				</div>
				{{ if .List.Description }}<p>{{ .List.Description }}</p>{{ end }}
			</div>
		</div>
	</section>
	<form id="list-archive" method="post" action="/lists/{{ .List.ID }}/archive"></form>

	{{ if .Error }}<p class="panel error">{{ .Error }}</p>{{ end }}

	<section class="panel full-width">
		<div class="tile-head">
			<h2>Items</h2>
			<button class="button compact" type="button" data-modal-open="add-list-item">+</button>
		</div>
		<div class="checklist">
			{{ range .ListItems }}
			{{ $item := . }}
			<article class="check-item {{ $item.Status }}">
				{{ if eq $item.Status "open" }}
				<form method="post" action="/lists/{{ $.List.ID }}/items/{{ $item.ID }}/complete">
					<button class="secondary check-toggle" type="submit" title="Complete item" aria-label="Complete item"></button>
				</form>
				{{ else }}
				<form method="post" action="/lists/{{ $.List.ID }}/items/{{ $item.ID }}/reopen">
					<button class="secondary check-toggle" type="submit" title="Reopen item" aria-label="Reopen item">X</button>
				</form>
				{{ end }}
				<div class="check-main">
					<div class="check-title">{{ $item.Title }}</div>
					{{ if $item.Notes }}<div class="meta">{{ $item.Notes }}</div>{{ end }}
					<div class="check-meta">
						<button class="pill" type="button" data-modal-open="edit-list-item-{{ $item.ID }}">Assigned: {{ if $item.AssignedName }}{{ $item.AssignedName }}{{ else }}Unassigned{{ end }}</button>
						<button class="pill" type="button" data-modal-open="edit-list-item-{{ $item.ID }}">Due: {{ if $item.DueAt }}{{ date $item.DueAt }}{{ else }}No due date{{ end }}</button>
					</div>
				</div>
				<div class="tile-actions">
					<button class="secondary compact" type="button" data-modal-open="edit-list-item-{{ $item.ID }}" title="Edit item" aria-label="Edit item">&#9998;</button>
					<form method="post" action="/lists/{{ $.List.ID }}/items/{{ $item.ID }}/archive"><button class="danger compact" type="submit" title="Archive item" aria-label="Archive item">X</button></form>
				</div>
			</article>
			{{ else }}
			<p class="empty">No items yet.</p>
			{{ end }}
		</div>
	</section>

	<section id="edit-list" class="modal">
		<div class="modal-card">
			<div class="modal-head">
				<h2>Edit List</h2>
				<button class="secondary compact" type="button" data-modal-close>Close</button>
			</div>
			<form method="post" action="/lists/{{ .List.ID }}">
				<label>Title<input name="title" value="{{ .List.Title }}" required></label>
				<label>Description<textarea name="description">{{ .List.Description }}</textarea></label>
				<div class="form-row">
					<label>Type<select name="kind">
						<option value="general" {{ selectedString .List.Kind "general" }}>general</option>
						<option value="grocery" {{ selectedString .List.Kind "grocery" }}>grocery</option>
						<option value="shopping" {{ selectedString .List.Kind "shopping" }}>shopping</option>
						<option value="packing" {{ selectedString .List.Kind "packing" }}>packing</option>
					</select></label>
					<label>Status<select name="status">
						<option value="active" {{ selectedString .List.Status "active" }}>active</option>
					</select></label>
				</div>
				<button type="submit">Save list</button>
			</form>
		</div>
	</section>

	<section id="add-list-item" class="modal">
		<div class="modal-card">
			<div class="modal-head">
				<h2>Add Item</h2>
				<button class="secondary compact" type="button" data-modal-close>Close</button>
			</div>
			<form method="post" action="/lists/{{ .List.ID }}/items">
				<label>Title<input name="title" required></label>
				<label>Notes<textarea name="notes"></textarea></label>
				<div class="form-row">
					<label>Assigned to<select name="assigned_to">
						<option value="">Unassigned</option>
						{{ range .Members }}<option value="{{ .ID }}">{{ .Name }}</option>{{ end }}
					</select></label>
					<label>Due<input name="due_at" type="date"></label>
				</div>
				<button type="submit">Add item</button>
			</form>
		</div>
	</section>

	{{ range .ListItems }}
	{{ $item := . }}
	<section id="edit-list-item-{{ $item.ID }}" class="modal">
		<div class="modal-card">
			<div class="modal-head">
				<h2>Edit Item</h2>
				<button class="secondary compact" type="button" data-modal-close>Close</button>
			</div>
			<form method="post" action="/lists/{{ $.List.ID }}/items/{{ $item.ID }}">
				<label>Title<input name="title" value="{{ $item.Title }}" required></label>
				<label>Notes<textarea name="notes">{{ $item.Notes }}</textarea></label>
				<div class="form-row">
					<label>Assigned to<select name="assigned_to">
						<option value="">Unassigned</option>
						{{ range $.Members }}<option value="{{ .ID }}" {{ selectedID $item.AssignedTo .ID }}>{{ .Name }}</option>{{ end }}
					</select></label>
					<label>Due<input name="due_at" type="date" value="{{ dateInput $item.DueAt }}"></label>
				</div>
				<label>Status<select name="status">
					<option value="open" {{ selectedString $item.Status "open" }}>open</option>
					<option value="done" {{ selectedString $item.Status "done" }}>done</option>
				</select></label>
				<button type="submit">Save item</button>
			</form>
		</div>
	</section>
	{{ end }}
</main>
{{ end }}

{{ define "projectStatusControl" }}
<details class="action-menu left">
	<summary class="pill {{ .Project.Status }}">{{ .Project.Status }}</summary>
	<div class="action-menu-panel">
		<button class="secondary compact" type="button" data-form="{{ .FormID }}" data-set-field="status" data-value="active">active</button>
		<button class="secondary compact" type="button" data-form="{{ .FormID }}" data-set-field="status" data-value="waiting">waiting</button>
		<button class="secondary compact" type="button" data-form="{{ .FormID }}" data-set-field="status" data-value="done">done</button>
	</div>
</details>
{{ end }}

{{ define "projectPriorityControl" }}
<details class="action-menu left">
	<summary class="pill {{ .Project.Priority }}">{{ .Project.Priority }}</summary>
	<div class="action-menu-panel">
		<button class="secondary compact" type="button" data-form="{{ .FormID }}" data-set-field="priority" data-value="normal">normal</button>
		<button class="secondary compact" type="button" data-form="{{ .FormID }}" data-set-field="priority" data-value="high">high</button>
		<button class="secondary compact" type="button" data-form="{{ .FormID }}" data-set-field="priority" data-value="low">low</button>
	</div>
</details>
{{ end }}

{{ define "projectDueControl" }}
<span class="due-picker">
	<button class="secondary compact" type="button" data-date-open="{{ .InputID }}">{{ if .Project.DueDate }}{{ date .Project.DueDate }}{{ else }}No due date{{ end }}</button>
	<input id="{{ .InputID }}" type="date" value="{{ dateInput .Project.DueDate }}" data-form="{{ .FormID }}" data-date-field="due_date">
</span>
{{ end }}

{{ define "taskStatusControl" }}
<details class="action-menu left">
	<summary class="pill {{ .Task.Status }}">{{ .Task.Status }}</summary>
	<div class="action-menu-panel">
		<button class="secondary compact" type="button" data-form="{{ .FormID }}" data-set-field="status" data-value="open">open</button>
		<button class="secondary compact" type="button" data-form="{{ .FormID }}" data-set-field="status" data-value="done">done</button>
	</div>
</details>
{{ end }}

{{ define "taskPriorityControl" }}
<details class="action-menu left">
	<summary class="pill {{ .Task.Priority }}">{{ .Task.Priority }}</summary>
	<div class="action-menu-panel">
		<button class="secondary compact" type="button" data-form="{{ .FormID }}" data-set-field="priority" data-value="normal">normal</button>
		<button class="secondary compact" type="button" data-form="{{ .FormID }}" data-set-field="priority" data-value="high">high</button>
		<button class="secondary compact" type="button" data-form="{{ .FormID }}" data-set-field="priority" data-value="low">low</button>
	</div>
</details>
{{ end }}

{{ define "taskAssigneeControl" }}
<details class="action-menu left">
	<summary class="pill">{{ if .Task.AssignedName }}{{ .Task.AssignedName }}{{ else }}Unassigned{{ end }}</summary>
	<div class="action-menu-panel">
		<button class="secondary compact" type="button" data-form="{{ .FormID }}" data-set-field="assigned_to" data-value="">Unassigned</button>
		{{ range .Members }}<button class="secondary compact" type="button" data-form="{{ $.FormID }}" data-set-field="assigned_to" data-value="{{ .ID }}">{{ .Name }}</button>{{ end }}
	</div>
</details>
{{ end }}

{{ define "taskDueControl" }}
<span class="due-picker">
	<button class="secondary compact" type="button" data-date-open="{{ .InputID }}">{{ if .Task.DueAt }}{{ date .Task.DueAt }}{{ else }}No due date{{ end }}</button>
	<input id="{{ .InputID }}" type="date" value="{{ dateInput .Task.DueAt }}" data-form="{{ .FormID }}" data-date-field="due_at">
</span>
{{ end }}

{{ define "projectTaskRow" }}
{{ $task := .Task }}
{{ $formID := printf "project-task-inline-%d" $task.ID }}
<tr class="task-row {{ $task.Status }}"{{ if .FolderID }} data-folder-parent="{{ .FolderID }}"{{ end }} data-task-row="{{ $task.ID }}">
	<td class="task-title-cell">
		<a href="/tasks/{{ $task.ID }}">{{ $task.Title }}</a>
		{{ if or .Documents .Contacts .Assets }}
		<div class="related-inline">
			{{ range .Documents }}<button class="link-button" type="button" data-modal-open="related-document-{{ .LinkID }}"><svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="m21.4 11.6-8.8 8.8a6 6 0 0 1-8.5-8.5l9.2-9.2a4 4 0 0 1 5.7 5.7l-9.2 9.2a2 2 0 0 1-2.8-2.8l8.8-8.8"></path></svg>{{ .Document.Title }}</button>{{ end }}
			{{ range .Contacts }}<button class="link-button" type="button" data-modal-open="related-contact-{{ .LinkID }}">@ {{ .Contact.Name }}</button>{{ end }}
			{{ range .Assets }}<button class="link-button" type="button" data-modal-open="related-asset-{{ .LinkID }}"># {{ .Asset.Name }}</button>{{ end }}
		</div>
		{{ end }}
	</td>
	<td class="task-meta-cell task-assignee-cell">
		{{ template "taskAssigneeControl" (dict "Task" $task "FormID" $formID "Members" .Members) }}
	</td>
	<td class="due-cell task-meta-cell task-due-cell">
		{{ template "taskDueControl" (dict "Task" $task "FormID" $formID "InputID" (printf "task-due-%d" $task.ID)) }}
	</td>
	<td class="task-meta-cell task-status-cell">
		{{ template "taskStatusControl" (dict "Task" $task "FormID" $formID) }}
	</td>
	<td class="task-meta-cell task-priority-cell">
		{{ template "taskPriorityControl" (dict "Task" $task "FormID" $formID) }}
	</td>
	<td class="task-action-cell">
		<details class="action-menu task-more-menu">
			<summary class="button secondary compact" title="Task actions" aria-label="Task actions">...</summary>
			<div class="action-menu-panel">
				<button class="secondary compact" type="button" data-modal-open="attach-project-task-document-{{ $task.ID }}">Attach document</button>
				<button class="secondary compact" type="button" data-modal-open="attach-project-task-contact-{{ $task.ID }}">Attach contact</button>
				<button class="secondary compact" type="button" data-modal-open="attach-project-task-asset-{{ $task.ID }}">Attach asset</button>
				<button class="secondary compact" type="button" data-modal-open="edit-project-task-{{ $task.ID }}">Edit</button>
				<form method="post" action="/projects/{{ .Project.ID }}/tasks/{{ $task.ID }}/archive">
					<button class="danger compact" type="submit">Archive</button>
				</form>
			</div>
		</details>
	</td>
</tr>
{{ end }}

{{ define "projectDetail" }}
{{ $openTasks := openTaskCount .Tasks }}
{{ $doneTasks := doneTaskCount .Tasks }}
{{ $totalTasks := len .Tasks }}
{{ $ungroupedTasks := tasksWithoutFolder .Tasks }}
<main class="shell">
	<section class="detail-hero">
		<div class="title-row">
			<div class="title-copy">
				<div class="project-title-line">
					<a class="button secondary back-icon" href="/projects" title="Back" aria-label="Back">‹</a>
					<h1>{{ .Project.Title }}</h1>
					{{ template "titleActionMenu" (dict "EditModal" "edit-project" "ArchiveForm" "project-archive") }}
					<form id="project-status-form" class="status-inline" method="post" action="/projects/{{ .Project.ID }}">
						<input type="hidden" name="title" value="{{ .Project.Title }}">
						<textarea name="description" hidden>{{ .Project.Description }}</textarea>
						<input type="hidden" name="priority" value="{{ .Project.Priority }}">
						<input type="hidden" name="due_date" value="{{ dateInput .Project.DueDate }}">
						<input type="hidden" name="status" value="{{ .Project.Status }}">
					</form>
					<details class="action-menu left">
						<summary class="pill {{ .Project.Status }}">{{ .Project.Status }}</summary>
						<div class="action-menu-panel">
							<button class="secondary compact" type="button" data-form="project-status-form" data-set-field="status" data-value="active">active</button>
							<button class="secondary compact" type="button" data-form="project-status-form" data-set-field="status" data-value="waiting">waiting</button>
							<button class="secondary compact" type="button" data-form="project-status-form" data-set-field="status" data-value="done">done</button>
						</div>
					</details>
				</div>
				<div class="summary-strip">
					<span class="badge">{{ $openTasks }} open</span>
					<span class="badge">{{ $doneTasks }} done</span>
					<span class="badge">{{ $totalTasks }} total</span>
				</div>
				<div class="info-strip">
					<div class="info-cell"><span>Description</span>{{ if .Project.Description }}{{ .Project.Description }}{{ else }}No description{{ end }}</div>
					<div class="info-cell"><span>Priority</span>{{ .Project.Priority }}</div>
					<div class="info-cell"><span>Due</span>{{ if .Project.DueDate }}{{ date .Project.DueDate }}{{ else }}No due date{{ end }}</div>
				</div>
			</div>
		</div>
	</section>
	<form id="project-archive" method="post" action="/projects/{{ .Project.ID }}/archive"></form>

	{{ if .Error }}<p class="panel error">{{ .Error }}</p>{{ end }}

	<div class="detail-with-sidebar">
	<div class="detail-main">
	<section class="panel">
		<div class="tile-head">
			<h2>Tasks</h2>
			<details class="action-menu">
				<summary class="button compact" aria-label="Add project item">+</summary>
				<div class="action-menu-panel">
					<button class="secondary compact" type="button" data-modal-open="add-project-folder">Folder</button>
					<button class="secondary compact" type="button" data-modal-open="add-project-task">Task</button>
				</div>
			</details>
		</div>

		{{ range .Tasks }}
		{{ $task := . }}
		<form id="project-task-inline-{{ $task.ID }}" method="post" action="/projects/{{ $.Project.ID }}/tasks/{{ $task.ID }}">
			<input type="hidden" name="title" value="{{ $task.Title }}">
			<input type="hidden" name="notes" value="{{ $task.Notes }}">
			<input type="hidden" name="project_folder_id" value="{{ idValue $task.ProjectFolderID }}">
			<input type="hidden" name="assigned_to" value="{{ idValue $task.AssignedTo }}">
			<input type="hidden" name="due_at" value="{{ datetimeInputPtr $task.DueAt }}">
			<input type="hidden" name="status" value="{{ $task.Status }}">
			<input type="hidden" name="priority" value="{{ $task.Priority }}">
			{{ if $task.RoutineID }}<input type="hidden" name="routine_id" value="{{ idValue $task.RoutineID }}">{{ end }}
			{{ if $task.AssetID }}<input type="hidden" name="asset_id" value="{{ idValue $task.AssetID }}">{{ end }}
			{{ if $task.AssetMaintenanceItemID }}<input type="hidden" name="asset_maintenance_item_id" value="{{ idValue $task.AssetMaintenanceItemID }}">{{ end }}
		</form>
		{{ end }}

		<div class="task-table-wrap">
			<table class="task-table">
				<thead>
					<tr>
						<th>Task</th>
						<th>Assigned</th>
						<th>Due</th>
						<th>Status</th>
						<th>Priority</th>
						<th></th>
					</tr>
				</thead>
				<tbody>
					{{ range .ProjectFolders }}
					{{ $folder := . }}
					{{ $folderTasks := tasksInFolder $.Tasks $folder.ID }}
						<tr class="folder-row" data-folder-drop="{{ $folder.ID }}">
							<td colspan="6">
							<div class="folder-summary">
								<button class="secondary compact" type="button" data-folder-toggle="{{ $folder.ID }}" aria-expanded="true">v</button>
								<span class="folder-title">{{ $folder.Title }}</span>
								<span class="badge folder-status-badge">{{ folderStatus $folderTasks }}</span>
								<span class="badge">{{ openTaskCount $folderTasks }} open</span>
								<span class="badge folder-done-badge">{{ doneTaskCount $folderTasks }} done</span>
								{{ if folderDue $folderTasks }}<span class="badge">Due {{ date (folderDue $folderTasks) }}</span>{{ end }}
								<details class="action-menu left">
									<summary class="button secondary compact" title="Folder actions" aria-label="Folder actions">...</summary>
									<div class="action-menu-panel">
										<button class="secondary compact" type="button" data-modal-open="edit-project-folder-{{ $folder.ID }}">Edit</button>
										<form method="post" action="/projects/{{ $.Project.ID }}/folders/{{ $folder.ID }}/archive">
											<button class="danger compact" type="submit">Archive</button>
										</form>
									</div>
								</details>
							</div>
						</td>
					</tr>
					{{ range $folderTasks }}
					{{ $task := . }}
					{{ $taskDocs := taskRelatedDocs $.TaskDocuments $task.ID }}
					{{ $taskContacts := taskRelatedContacts $.TaskContacts $task.ID }}
					{{ $taskAssets := taskRelatedAssets $.TaskAssets $task.ID }}
					{{ template "projectTaskRow" (dict "Task" $task "FolderID" $folder.ID "Project" $.Project "Members" $.Dashboard.Members "Documents" $taskDocs "Contacts" $taskContacts "Assets" $taskAssets) }}
					{{ end }}
					{{ end }}

					{{ if or .ProjectFolders $ungroupedTasks }}
						<tr class="folder-row" data-folder-drop="">
							<td colspan="6"><div class="folder-summary"><span class="folder-title">Ungrouped</span><span class="badge">{{ openTaskCount $ungroupedTasks }} open</span><span class="badge folder-done-badge">{{ doneTaskCount $ungroupedTasks }} done</span>{{ if folderDue $ungroupedTasks }}<span class="badge">Due {{ date (folderDue $ungroupedTasks) }}</span>{{ end }}</div></td>
					</tr>
					{{ range $ungroupedTasks }}
					{{ $task := . }}
					{{ $taskDocs := taskRelatedDocs $.TaskDocuments $task.ID }}
					{{ $taskContacts := taskRelatedContacts $.TaskContacts $task.ID }}
					{{ $taskAssets := taskRelatedAssets $.TaskAssets $task.ID }}
					{{ template "projectTaskRow" (dict "Task" $task "Project" $.Project "Members" $.Dashboard.Members "Documents" $taskDocs "Contacts" $taskContacts "Assets" $taskAssets) }}
					{{ end }}
					{{ end }}

					{{ if and (not .ProjectFolders) (not .Tasks) }}
						<tr><td colspan="6"><p class="empty">No project tasks yet.</p></td></tr>
					{{ end }}
				</tbody>
			</table>
		</div>
	</section>

	</div>
	<aside class="panel detail-sidebar">
		<section class="info-panel-section">
			<div class="tile-head">
				<h2>Documents</h2>
				<button class="button compact" type="button" data-modal-open="attach-project-document">+</button>
			</div>
		{{ if .RelatedDocs }}
		<h3>Project</h3>
		{{ template "relatedDocumentsList" (dictRelated .RelatedDocs "project" .Project.ID) }}
		{{ end }}
		{{ range .Tasks }}
		{{ $taskDocs := taskRelatedDocs $.TaskDocuments .ID }}
		{{ if $taskDocs }}
		<h3 style="margin-top:14px;">Task - <a href="/tasks/{{ .ID }}">{{ .Title }}</a></h3>
		{{ template "relatedDocumentsList" (dictRelatedPrefix $taskDocs (printf "/projects/%d/tasks/%d/documents" $.Project.ID .ID)) }}
		{{ end }}
		{{ end }}
		{{ if and (not .RelatedDocs) (not (hasTaskDocuments .TaskDocuments)) }}<p class="empty">No related documents yet.</p>{{ end }}
		</section>

		<section class="info-panel-section">
			<div class="tile-head">
				<h2>Contacts</h2>
				<button class="button compact" type="button" data-modal-open="attach-project-contact">+</button>
			</div>
		{{ if .RelatedContacts }}
		<h3>Project</h3>
		{{ template "relatedContactsList" (dictRelatedContacts .RelatedContacts (printf "/projects/%d/contacts" .Project.ID)) }}
		{{ end }}
		{{ range .Tasks }}
		{{ $taskContacts := taskRelatedContacts $.TaskContacts .ID }}
		{{ if $taskContacts }}
		<h3 style="margin-top:14px;">Task - <a href="/tasks/{{ .ID }}">{{ .Title }}</a></h3>
		{{ template "relatedContactsList" (dictRelatedContacts $taskContacts (printf "/projects/%d/tasks/%d/contacts" $.Project.ID .ID)) }}
		{{ end }}
		{{ end }}
		{{ if and (not .RelatedContacts) (not (hasTaskContacts .TaskContacts)) }}<p class="empty">No related contacts yet.</p>{{ end }}
		</section>

		<section class="info-panel-section">
			<div class="tile-head">
				<h2>Assets</h2>
				<button class="button compact" type="button" data-modal-open="attach-project-asset">+</button>
			</div>
		{{ if .RelatedAssets }}
		<h3>Project</h3>
		{{ template "relatedAssetsList" (dictRelatedAssets .RelatedAssets (printf "/projects/%d/assets" .Project.ID)) }}
		{{ end }}
		{{ range .Tasks }}
		{{ $taskAssets := taskRelatedAssets $.TaskAssets .ID }}
		{{ if $taskAssets }}
		<h3 style="margin-top:14px;">Task - <a href="/tasks/{{ .ID }}">{{ .Title }}</a></h3>
		{{ template "relatedAssetsList" (dictRelatedAssets $taskAssets (printf "/projects/%d/tasks/%d/assets" $.Project.ID .ID)) }}
		{{ end }}
		{{ end }}
		{{ if and (not .RelatedAssets) (not (hasTaskAssets .TaskAssets)) }}<p class="empty">No related assets yet.</p>{{ end }}
		</section>
	</aside>
	</div>

	{{ template "attachDocumentModal" (dictAttach "attach-project-document" (printf "/projects/%d/documents" .Project.ID) .Documents) }}
	{{ template "attachContactModal" (dictAttachContact "attach-project-contact" (printf "/projects/%d/contacts" .Project.ID) .Contacts) }}
	{{ template "attachAssetModal" (dictAttachAsset "attach-project-asset" (printf "/projects/%d/assets" .Project.ID) .Assets) }}
	{{ range .Tasks }}
		{{ template "attachDocumentModal" (dictAttach (printf "attach-project-task-document-%d" .ID) (printf "/projects/%d/tasks/%d/documents" $.Project.ID .ID) $.Documents) }}
		{{ template "attachContactModal" (dictAttachContact (printf "attach-project-task-contact-%d" .ID) (printf "/projects/%d/tasks/%d/contacts" $.Project.ID .ID) $.Contacts) }}
		{{ template "attachAssetModal" (dictAttachAsset (printf "attach-project-task-asset-%d" .ID) (printf "/projects/%d/tasks/%d/assets" $.Project.ID .ID) $.Assets) }}
	{{ end }}

	<section id="edit-project" class="modal">
		<div class="modal-card">
			<div class="modal-head">
				<h2>Edit Project</h2>
				<button class="secondary compact" type="button" data-modal-close>Close</button>
			</div>
			<form id="project-edit" method="post" action="/projects/{{ .Project.ID }}">
				<label>Title<input name="title" value="{{ .Project.Title }}" required></label>
				<label>Description<textarea name="description">{{ .Project.Description }}</textarea></label>
				<div class="form-row">
					<label>Status<select name="status">
						<option value="active" {{ selectedString .Project.Status "active" }}>active</option>
						<option value="waiting" {{ selectedString .Project.Status "waiting" }}>waiting</option>
						<option value="done" {{ selectedString .Project.Status "done" }}>done</option>
					</select></label>
					<label>Priority<select name="priority">
						<option value="normal" {{ selectedString .Project.Priority "normal" }}>normal</option>
						<option value="high" {{ selectedString .Project.Priority "high" }}>high</option>
						<option value="low" {{ selectedString .Project.Priority "low" }}>low</option>
					</select></label>
				</div>
				<label>Due<input name="due_date" type="date" value="{{ dateInput .Project.DueDate }}"></label>
				<button type="submit">Save project</button>
			</form>
		</div>
	</section>

	<section id="add-project-folder" class="modal">
		<div class="modal-card">
			<div class="modal-head">
				<h2>Add Folder</h2>
				<button class="secondary compact" type="button" data-modal-close>Close</button>
			</div>
			<form method="post" action="/projects/{{ .Project.ID }}/folders">
				<label>Title<input name="title" required></label>
				<button type="submit">Add folder</button>
			</form>
		</div>
	</section>

	{{ range .ProjectFolders }}
	<section id="edit-project-folder-{{ .ID }}" class="modal">
		<div class="modal-card">
			<div class="modal-head">
				<h2>Edit Folder</h2>
				<button class="secondary compact" type="button" data-modal-close>Close</button>
			</div>
			<form method="post" action="/projects/{{ $.Project.ID }}/folders/{{ .ID }}">
				<label>Title<input name="title" value="{{ .Title }}" required></label>
				<button type="submit">Save folder</button>
			</form>
		</div>
	</section>
	{{ end }}

	<section id="add-project-task" class="modal">
		<div class="modal-card">
			<div class="modal-head">
				<h2>Add Project Task</h2>
				<button class="secondary compact" type="button" data-modal-close>Close</button>
			</div>
			<form method="post" action="/projects/{{ .Project.ID }}/tasks">
				<label>Title<input name="title" required></label>
				<label>Notes<textarea name="notes"></textarea></label>
				<div class="form-row">
					<label>Folder<select name="project_folder_id">
						<option value="">None</option>
						{{ range .ProjectFolders }}<option value="{{ .ID }}">{{ .Title }}</option>{{ end }}
					</select></label>
					<label>Assigned to<select name="assigned_to">
						<option value="">Unassigned</option>
						{{ range .Dashboard.Members }}<option value="{{ .ID }}">{{ .Name }}</option>{{ end }}
					</select></label>
				</div>
				<div class="form-row">
					<label>Due<input name="due_at" type="date"></label>
					<label>Priority<select name="priority"><option>normal</option><option>high</option><option>low</option></select></label>
				</div>
				<button type="submit">Add task</button>
			</form>
		</div>
	</section>

	{{ range .Tasks }}
	{{ $task := . }}
	<section id="edit-project-task-{{ $task.ID }}" class="modal">
		<div class="modal-card">
			<div class="modal-head">
				<h2>Edit Task</h2>
				<button class="secondary compact" type="button" data-modal-close>Close</button>
			</div>
			<form method="post" action="/projects/{{ $.Project.ID }}/tasks/{{ $task.ID }}">
				{{ if $task.RoutineID }}<input type="hidden" name="routine_id" value="{{ idValue $task.RoutineID }}">{{ end }}
				{{ if $task.AssetID }}<input type="hidden" name="asset_id" value="{{ idValue $task.AssetID }}">{{ end }}
				{{ if $task.AssetMaintenanceItemID }}<input type="hidden" name="asset_maintenance_item_id" value="{{ idValue $task.AssetMaintenanceItemID }}">{{ end }}
				<label>Title<input name="title" value="{{ $task.Title }}" required></label>
				<label>Notes<textarea name="notes">{{ $task.Notes }}</textarea></label>
				<div class="form-row">
					<label>Folder<select name="project_folder_id">
						<option value="">None</option>
						{{ range $.ProjectFolders }}<option value="{{ .ID }}" {{ selectedID $task.ProjectFolderID .ID }}>{{ .Title }}</option>{{ end }}
					</select></label>
					<label>Assigned to<select name="assigned_to">
						<option value="">Unassigned</option>
						{{ range $.Dashboard.Members }}<option value="{{ .ID }}" {{ selectedID $task.AssignedTo .ID }}>{{ .Name }}</option>{{ end }}
					</select></label>
				</div>
				<div class="form-row">
					<label>Due<input name="due_at" type="date" value="{{ dateInput $task.DueAt }}"></label>
					<label>Priority<select name="priority">
						<option value="normal" {{ selectedString $task.Priority "normal" }}>normal</option>
						<option value="high" {{ selectedString $task.Priority "high" }}>high</option>
						<option value="low" {{ selectedString $task.Priority "low" }}>low</option>
					</select></label>
				</div>
				<div class="form-row">
					<label>Status<select name="status">
						<option value="open" {{ selectedString $task.Status "open" }}>open</option>
						<option value="done" {{ selectedString $task.Status "done" }}>done</option>
					</select></label>
				</div>
				<button type="submit">Save task</button>
			</form>
		</div>
	</section>
	{{ end }}
</main>
{{ end }}

{{ define "taskDetail" }}
<main class="shell">
	<section class="detail-hero">
		<div class="title-row">
			<div class="title-copy">
				<div class="page-title-line">
					<a class="button secondary back-icon" href="/tasks" title="Back" aria-label="Back">‹</a>
					<h1>{{ .Task.Title }}</h1>
					{{ if eq .Task.Status "done" }}
						{{ template "titleActionMenu" (dict "ReopenForm" "task-reopen" "ArchiveForm" "task-archive") }}
					{{ else }}
						{{ template "titleActionMenu" (dict "ArchiveForm" "task-archive") }}
					{{ end }}
				</div>
			</div>
		</div>
	</section>
	<form id="task-archive" method="post" action="/tasks/{{ .Task.ID }}/archive"></form>
	{{ if eq .Task.Status "done" }}<form id="task-reopen" method="post" action="/tasks/{{ .Task.ID }}/reopen"></form>{{ end }}

	{{ if .Error }}<p class="panel error">{{ .Error }}</p>{{ end }}

	<div class="detail-with-sidebar">
	<div class="detail-main">
	<section class="panel">
		<div class="tile-head"><h2>Edit Task</h2></div>
		<form id="task-edit" method="post" action="/tasks/{{ .Task.ID }}">
			{{ if .Task.RoutineID }}<input type="hidden" name="routine_id" value="{{ idValue .Task.RoutineID }}">{{ end }}
			{{ if .Task.AssetID }}<input type="hidden" name="asset_id" value="{{ idValue .Task.AssetID }}">{{ end }}
			{{ if .Task.AssetMaintenanceItemID }}<input type="hidden" name="asset_maintenance_item_id" value="{{ idValue .Task.AssetMaintenanceItemID }}">{{ end }}
			<label>Title<input name="title" value="{{ .Task.Title }}" required></label>
			<label>Notes<textarea name="notes">{{ .Task.Notes }}</textarea></label>
			<div class="form-row">
				<label>Project<select name="project_id">
					<option value="">Standalone</option>
					{{ range .Projects }}<option value="{{ .ID }}" {{ selectedID $.Task.ProjectID .ID }}>{{ .Title }}</option>{{ end }}
				</select></label>
				<label>Assigned to<select name="assigned_to">
					<option value="">Unassigned</option>
					{{ range .Members }}<option value="{{ .ID }}" {{ selectedID $.Task.AssignedTo .ID }}>{{ .Name }}</option>{{ end }}
				</select></label>
			</div>
			<div class="form-row">
				<label>Status<select name="status">
					<option value="open" {{ selectedString .Task.Status "open" }}>open</option>
					<option value="done" {{ selectedString .Task.Status "done" }}>done</option>
				</select></label>
				<label>Priority<select name="priority">
					<option value="normal" {{ selectedString .Task.Priority "normal" }}>normal</option>
					<option value="high" {{ selectedString .Task.Priority "high" }}>high</option>
					<option value="low" {{ selectedString .Task.Priority "low" }}>low</option>
				</select></label>
			</div>
			<label>Due<input name="due_at" type="datetime-local" value="{{ datetimeInputPtr .Task.DueAt }}"></label>
			<button type="submit">Save task</button>
		</form>
	</section>
	</div>

	<aside class="panel detail-sidebar">
		<section class="info-panel-section">
			<div class="tile-head">
				<h2>Documents</h2>
				<button class="button compact" type="button" data-modal-open="attach-task-document">+</button>
			</div>
			{{ template "relatedDocumentsList" (dictRelated .RelatedDocs "task" .Task.ID) }}
		</section>

		<section class="info-panel-section">
			<div class="tile-head">
				<h2>Contacts</h2>
				<button class="button compact" type="button" data-modal-open="attach-task-contact">+</button>
			</div>
			{{ template "relatedContactsList" (dictRelatedContacts .RelatedContacts (printf "/tasks/%d/contacts" .Task.ID)) }}
		</section>

		<section class="info-panel-section">
			<div class="tile-head">
				<h2>Assets</h2>
				<button class="button compact" type="button" data-modal-open="attach-task-asset">+</button>
			</div>
			{{ template "relatedAssetsList" (dictRelatedAssets .RelatedAssets (printf "/tasks/%d/assets" .Task.ID)) }}
		</section>
	</aside>
	</div>

	{{ template "attachDocumentModal" (dictAttach "attach-task-document" (printf "/tasks/%d/documents" .Task.ID) .Documents) }}
	{{ template "attachContactModal" (dictAttachContact "attach-task-contact" (printf "/tasks/%d/contacts" .Task.ID) .Contacts) }}
	{{ template "attachAssetModal" (dictAttachAsset "attach-task-asset" (printf "/tasks/%d/assets" .Task.ID) .Assets) }}
</main>
{{ end }}

{{ define "eventDetail" }}
<main class="shell">
	<section class="detail-hero">
		<div class="title-row">
			<div class="title-copy">
				<div class="page-title-line">
					<a class="button secondary back-icon" href="/" title="Back" aria-label="Back">‹</a>
					<h1>{{ .Event.Title }}</h1>
					{{ template "titleActionMenu" (dict "DeleteForm" "event-delete" "DeleteLabel" "Delete") }}
				</div>
			</div>
		</div>
	</section>
	<form id="event-delete" method="post" action="/events/{{ .Event.ID }}/delete"></form>

	{{ if .Error }}<p class="panel error">{{ .Error }}</p>{{ end }}

	<section class="panel">
		<div class="tile-head"><h2>Edit Appointment</h2></div>
		<form id="event-edit" method="post" action="/events/{{ .Event.ID }}">
			<label>Title<input name="title" value="{{ .Event.Title }}" required></label>
			<label>Location<input name="location" value="{{ .Event.Location }}"></label>
			<div class="form-row">
				<label>Starts<input name="starts_at" type="datetime-local" value="{{ datetimeInput .Event.StartsAt }}" required></label>
				<label>Ends<input name="ends_at" type="datetime-local" value="{{ datetimeInput .Event.EndsAt }}" required></label>
			</div>
			<label>Description<textarea name="description">{{ .Event.Description }}</textarea></label>
			<button type="submit">Save appointment</button>
		</form>
	</section>
</main>
{{ end }}

{{ define "routineDetail" }}
<main class="shell">
	<section class="detail-hero">
		<div class="title-row">
			<div class="title-copy">
				<div class="page-title-line">
					<a class="button secondary back-icon" href="/routines" title="Back" aria-label="Back">‹</a>
					<h1>{{ .Routine.Title }}</h1>
					{{ template "titleActionMenu" (dict "ArchiveForm" "routine-archive") }}
				</div>
			</div>
		</div>
	</section>
	<form id="routine-archive" method="post" action="/routines/{{ .Routine.ID }}/archive"></form>

	{{ if .Error }}<p class="panel error">{{ .Error }}</p>{{ end }}

	<div class="dashboard-tiles">
		<section class="panel">
			<div class="tile-head"><h2>Edit Routine</h2></div>
			<form id="routine-edit" method="post" action="/routines/{{ .Routine.ID }}">
				<label>Title<input name="title" value="{{ .Routine.Title }}" required></label>
				<label>Notes<textarea name="notes">{{ .Routine.Notes }}</textarea></label>
				<div class="form-row">
					<label>Cadence<select name="cadence">
						<option value="daily" {{ selectedString .Routine.Cadence "daily" }}>daily</option>
						<option value="weekly" {{ selectedString .Routine.Cadence "weekly" }}>weekly</option>
						<option value="monthly" {{ selectedString .Routine.Cadence "monthly" }}>monthly</option>
						<option value="quarterly" {{ selectedString .Routine.Cadence "quarterly" }}>quarterly</option>
						<option value="yearly" {{ selectedString .Routine.Cadence "yearly" }}>yearly</option>
					</select></label>
					<label>Assigned to<select name="assigned_to">
						<option value="">Unassigned</option>
						{{ range .Members }}<option value="{{ .ID }}" {{ selectedID $.Routine.AssignedTo .ID }}>{{ .Name }}</option>{{ end }}
					</select></label>
				</div>
				<div class="form-row">
					<label>Status<select name="status">
						<option value="active" {{ selectedString .Routine.Status "active" }}>active</option>
						<option value="archived" {{ selectedString .Routine.Status "archived" }}>archived</option>
					</select></label>
					<label>Next due<input name="next_due_at" type="date" value="{{ dateInput .Routine.NextDueAt }}"></label>
				</div>
				<button type="submit">Save routine</button>
			</form>
		</section>

		<section class="panel">
			<div class="tile-head"><h2>Generated Tasks</h2></div>
			<div class="cards">
				{{ range routineTasks .Dashboard.Tasks .Routine.ID }}
					{{ template "taskItem" . }}
				{{ else }}
					<p class="empty">No generated tasks yet.</p>
				{{ end }}
			</div>
		</section>
	</div>
</main>
{{ end }}

{{ define "taskItem" }}
<article class="item">
	<div class="item-head">
		<div>
			<strong><a href="/tasks/{{ .ID }}">{{ .Title }}</a></strong>
			{{ if .Notes }}<div class="meta">{{ .Notes }}</div>{{ end }}
			{{ if .AssignedName }}<div class="meta">Assigned to {{ .AssignedName }}</div>{{ end }}
			{{ if .RoutineID }}<div class="meta">Generated from routine</div>{{ end }}
			{{ if .AssetID }}<div class="meta">Generated from asset maintenance</div>{{ end }}
			{{ if .DueAt }}<div class="meta">Due {{ date .DueAt }}</div>{{ end }}
		</div>
		<span class="badge {{ .Priority }}">{{ .Priority }}</span>
	</div>
	{{ if eq .Status "open" }}
	<form method="post" action="/tasks/complete" style="margin-top:10px;">
		<input type="hidden" name="id" value="{{ .ID }}">
		<button class="secondary" type="submit">Complete</button>
	</form>
	{{ end }}
</article>
{{ end }}
`
