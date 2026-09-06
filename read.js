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
