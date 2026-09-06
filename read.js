// Reading chrome: theme, type scale, measure, progress, and a TOC
// built from the article headings.

(function() {
"use strict";

var article = document.querySelector("article");
var toc = document.getElementById("toc");
var bar = document.querySelector("#progress > i");
var measureLabel = document.getElementById("measure-label");
var html = document.documentElement;

var scales = ["size-sm", "size-md", "size-lg"];
var measures = [
	{cls: "measure-narrow", label: "42"},
	{cls: "measure-optimal", label: "48"},
	{cls: "measure-wide", label: "64"}
];

function get(k, fallback) {
	try {
		var v = localStorage.getItem("mdweb-" + k);
		return v == null ? fallback : v;
	} catch (e) {
		return fallback;
	}
}

function set(k, v) {
	try { localStorage.setItem("mdweb-" + k, v); } catch (e) {}
}

// A directory listing page auto-opens its top file (by the saved sort
// order) so loading mdv drops straight into reading, not a bare index.
if (document.body.dataset.listing) {
	var indexLinks = Array.prototype.slice.call(document.querySelectorAll(".index a[data-mtime]"));
	if (indexLinks.length) {
		if (get("filesort", "name") == "recent") {
			indexLinks.sort(function(a, b) {
				return (parseInt(b.dataset.mtime, 10) || 0) - (parseInt(a.dataset.mtime, 10) || 0);
			});
		}
		location.replace(indexLinks[0].getAttribute("href"));
	}
}

function applyTheme(name) {
	html.classList.remove("light", "sepia", "dark");
	if (name == "system" || name == "") {
		if (window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches)
			html.classList.add("dark");
		return;
	}
	html.classList.add(name);
}

function applyScale(i) {
	i = Math.max(0, Math.min(scales.length - 1, i));
	for (var k = 0; k < scales.length; k++)
		html.classList.remove(scales[k]);
	if (i != 1)
		html.classList.add(scales[i]);
	return i;
}

function applyMeasure(i) {
	i = Math.max(0, Math.min(measures.length - 1, i));
	for (var k = 0; k < measures.length; k++)
		html.classList.remove(measures[k].cls);
	if (i != 1)
		html.classList.add(measures[i].cls);
	if (measureLabel)
		measureLabel.textContent = measures[i].label;
	return i;
}

var theme = get("theme", "system");
var scale = applyScale(parseInt(get("scale", "1"), 10) || 1);
var measure = applyMeasure(parseInt(get("measure", "1"), 10) || 1);
applyTheme(theme);

document.getElementById("theme-light").onclick = function() {
	theme = "light"; set("theme", theme); applyTheme(theme);
};
document.getElementById("theme-sepia").onclick = function() {
	theme = "sepia"; set("theme", theme); applyTheme(theme);
};
document.getElementById("theme-dark").onclick = function() {
	theme = "dark"; set("theme", theme); applyTheme(theme);
};
document.getElementById("font-dec").onclick = function() {
	scale = applyScale(scale - 1); set("scale", String(scale));
};
document.getElementById("font-inc").onclick = function() {
	scale = applyScale(scale + 1); set("scale", String(scale));
};
document.getElementById("measure").onclick = function() {
	measure = applyMeasure((measure + 1) % measures.length);
	set("measure", String(measure));
};

var filesNav = document.getElementById("files");

var filesHidden = get("fileshidden", "0") == "1";
function applyFilesHidden() {
	document.body.classList.toggle("files-hidden", filesHidden);
}
applyFilesHidden();
var filesToggle = document.getElementById("files-toggle");
if (filesToggle) {
	filesToggle.onclick = function() {
		filesHidden = !filesHidden;
		set("fileshidden", filesHidden ? "1" : "0");
		applyFilesHidden();
	};
}

function updateNavButtons() {
	var prevBtn = document.getElementById("nav-prev");
	var nextBtn = document.getElementById("nav-next");
	if (!prevBtn || !nextBtn)
		return;
	var links = filesNav ? Array.prototype.slice.call(filesNav.querySelectorAll("a")) : [];
	var at = -1;
	for (var i = 0; i < links.length; i++) {
		if (links[i].classList.contains("current"))
			at = i;
	}
	var prevLink = at > 0 ? links[at - 1] : null;
	var nextLink = at >= 0 && at < links.length - 1 ? links[at + 1] : null;
	prevBtn.disabled = !prevLink;
	nextBtn.disabled = !nextLink;
	prevBtn.onclick = prevLink ? function() { location.href = prevLink.getAttribute("href"); } : null;
	nextBtn.onclick = nextLink ? function() { location.href = nextLink.getAttribute("href"); } : null;
}

function sortFiles(mode) {
	if (!filesNav)
		return;
	var links = Array.prototype.slice.call(filesNav.querySelectorAll("a"));
	links.sort(function(a, b) {
		if (mode == "recent")
			return (parseInt(b.dataset.mtime, 10) || 0) - (parseInt(a.dataset.mtime, 10) || 0);
		return a.textContent.localeCompare(b.textContent);
	});
	for (var i = 0; i < links.length; i++)
		filesNav.appendChild(links[i]);
	updateNavButtons();
}

var filesSort = get("filesort", "name");
var filesSortBtn = document.getElementById("files-sort");
function applyFilesSortLabel() {
	if (filesSortBtn)
		filesSortBtn.textContent = filesSort == "recent" ? "Recent" : "Name";
}
applyFilesSortLabel();
sortFiles(filesSort);
if (filesSortBtn) {
	filesSortBtn.onclick = function() {
		filesSort = filesSort == "recent" ? "name" : "recent";
		set("filesort", filesSort);
		applyFilesSortLabel();
		sortFiles(filesSort);
	};
}

if (toc && article) {
	var heads = article.querySelectorAll("h1, h2, h3");
	if (heads.length > 1) {
		var label = document.createElement("div");
		label.className = "label";
		label.textContent = "Contents";
		toc.appendChild(label);
		for (var i = 0; i < heads.length; i++) {
			var h = heads[i];
			if (!h.id) {
				h.id = "h-" + (i + 1);
			}
			var a = document.createElement("a");
			a.href = "#" + h.id;
			a.textContent = h.textContent;
			toc.appendChild(a);
		}
	}
}

function onScroll() {
	var max = document.documentElement.scrollHeight - document.documentElement.clientHeight;
	var pct = max <= 0 ? 0 : Math.min(1, Math.max(0, window.scrollY / max));
	if (bar)
		bar.style.width = (pct * 100) + "%";
	if (!toc)
		return;
	var links = toc.querySelectorAll("a");
	var at = 0;
	for (var i = 0; i < links.length; i++) {
		var id = links[i].hash.substr(1);
		var el = document.getElementById(id);
		if (el != null && el.getBoundingClientRect().top < 120)
			at = i;
	}
	for (var i = 0; i < links.length; i++)
		links[i].classList.toggle("current", i == at);
}

window.addEventListener("scroll", onScroll, {passive: true});
onScroll();

})();
